// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"net/http"

	"errors"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	ContextPostKey    = "post"
	ContextChannelKey = "channel"
	ContextBotKey     = "bot"
)

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := gin.Default()
	router.Use(p.ginlogger)
	router.Use(p.metricsMiddleware)

	interPluginRoute := router.Group("/inter-plugin/v1")
	interPluginRoute.Use(p.interPluginAuthorizationRequired)
	interPluginRoute.POST("/simple_completion", p.handleInterPluginSimpleCompletion)

	router.Use(p.MattermostAuthorizationRequired)

	router.GET("/ai_threads", p.handleGetAIThreads)
	router.GET("/ai_bots", p.handleGetAIBots)

	botRequiredRouter := router.Group("")
	botRequiredRouter.Use(p.aiBotRequired)

	postRouter := botRequiredRouter.Group("/post/:postid")
	postRouter.Use(p.postAuthorizationRequired)
	postRouter.POST("/react", p.handleReact)
	postRouter.POST("/analyze", p.handleThreadAnalysis)
	postRouter.POST("/transcribe/file/:fileid", p.handleTranscribeFile)
	postRouter.POST("/summarize_transcription", p.handleSummarizeTranscription)
	postRouter.POST("/stop", p.handleStop)
	postRouter.POST("/regenerate", p.handleRegenerate)
	postRouter.POST("/tool_call", p.handleToolCall)
	postRouter.POST("/postback_summary", p.handlePostbackSummary)

	channelRouter := botRequiredRouter.Group("/channel/:channelid")
	channelRouter.Use(p.channelAuthorizationRequired)
	channelRouter.POST("/interval", p.handleInterval)

	adminRouter := router.Group("/admin")
	adminRouter.Use(p.mattermostAdminAuthorizationRequired)
	adminRouter.POST("/reindex", p.handleReindexPosts)
	adminRouter.GET("/reindex/status", p.handleGetJobStatus)
	adminRouter.POST("/reindex/cancel", p.handleCancelJob)

	searchRouter := botRequiredRouter.Group("/search")
	// Only returns search results
	searchRouter.POST("", p.handleSearchQuery)
	// Initiates a search and responds to the user in a DM with the selected bot
	searchRouter.POST("/run", p.handleRunSearch)

	router.ServeHTTP(w, r)
}

func (p *Plugin) aiBotRequired(c *gin.Context) {
	botUsername := c.DefaultQuery("botUsername", p.getConfiguration().DefaultBotName)
	bot := p.GetBotByUsernameOrFirst(botUsername)
	if bot == nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get bot: %s", botUsername))
		return
	}
	c.Set(ContextBotKey, bot)
}

func (p *Plugin) ginlogger(c *gin.Context) {
	c.Next()

	for _, ginErr := range c.Errors {
		p.API.LogError(ginErr.Error())
	}
}

func (p *Plugin) MattermostAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
}

func (p *Plugin) interPluginAuthorizationRequired(c *gin.Context) {
	pluginID := c.GetHeader("Mattermost-Plugin-ID")
	if pluginID != "" {
		return
	}
	c.AbortWithStatus(http.StatusUnauthorized)
}

func (p *Plugin) handleGetAIThreads(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	p.botsLock.RLock()
	defer p.botsLock.RUnlock()
	dmChannelIDs := []string{}
	for _, bot := range p.bots {
		channelName := model.GetDMNameFromIds(userID, bot.mmBot.UserId)
		botDMChannel, err := p.pluginAPI.Channel.GetByName("", channelName, false)
		if err != nil {
			if errors.Is(err, pluginapi.ErrNotFound) {
				// Channel doesn't exist yet, so we'll skip it
				continue
			}
			p.API.LogError("unable to get DM channel for bot", "error", err, "bot_id", bot.mmBot.UserId)
			continue
		}

		// Extra permissions checks are not totally necessary since a user should always have permission to read their own DMs
		if !p.pluginAPI.User.HasPermissionToChannel(userID, botDMChannel.Id, model.PermissionReadChannel) {
			p.API.LogDebug("user doesn't have permission to read channel", "user_id", userID, "channel_id", botDMChannel.Id, "bot_id", bot.mmBot.UserId)
			continue
		}

		dmChannelIDs = append(dmChannelIDs, botDMChannel.Id)
	}

	threads, err := p.getAIThreads(dmChannelIDs)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get posts for bot DM: %w", err))
		return
	}

	c.JSON(http.StatusOK, threads)
}

type AIBotInfo struct {
	ID                 string                 `json:"id"`
	DisplayName        string                 `json:"displayName"`
	Username           string                 `json:"username"`
	LastIconUpdate     int64                  `json:"lastIconUpdate"`
	DMChannelID        string                 `json:"dmChannelID"`
	ChannelAccessLevel llm.ChannelAccessLevel `json:"channelAccessLevel"`
	ChannelIDs         []string               `json:"channelIDs"`
	UserAccessLevel    llm.UserAccessLevel    `json:"userAccessLevel"`
	UserIDs            []string               `json:"userIDs"`
}

type AIBotsResponse struct {
	Bots          []AIBotInfo `json:"bots"`
	SearchEnabled bool        `json:"searchEnabled"`
}

func (p *Plugin) handleGetAIBots(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	p.botsLock.RLock()
	defer p.botsLock.RUnlock()

	// Get the info from all the bots.
	// Put the default bot first.
	bots := make([]AIBotInfo, 0, len(p.bots))
	defaultBotName := p.getConfiguration().DefaultBotName
	for i, bot := range p.bots {
		// Don't return bots the user is excluded from using.
		if p.checkUsageRestrictionsForUser(bot, userID) != nil {
			continue
		}

		// Get the bot DM channel ID. To avoid creating the channel unless nessary
		/// we return "" if the channel doesn't exist.
		dmChannelID := ""
		channelName := model.GetDMNameFromIds(userID, bot.mmBot.UserId)
		botDMChannel, err := p.pluginAPI.Channel.GetByName("", channelName, false)
		if err == nil {
			dmChannelID = botDMChannel.Id
		}

		bots = append(bots, AIBotInfo{
			ID:                 bot.mmBot.UserId,
			DisplayName:        bot.mmBot.DisplayName,
			Username:           bot.mmBot.Username,
			LastIconUpdate:     bot.mmBot.LastIconUpdate,
			DMChannelID:        dmChannelID,
			ChannelAccessLevel: bot.cfg.ChannelAccessLevel,
			ChannelIDs:         bot.cfg.ChannelIDs,
			UserAccessLevel:    bot.cfg.UserAccessLevel,
			UserIDs:            bot.cfg.UserIDs,
		})
		if bot.mmBot.Username == defaultBotName {
			bots[0], bots[i] = bots[i], bots[0]
		}
	}

	// Check if search is enabled
	searchEnabled := p.search != nil && p.getConfiguration().EmbeddingSearchConfig.Type != ""

	response := AIBotsResponse{
		Bots:          bots,
		SearchEnabled: searchEnabled,
	}

	c.JSON(http.StatusOK, response)
}
