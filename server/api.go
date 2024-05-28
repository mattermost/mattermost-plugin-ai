package main

import (
	"fmt"
	"net/http"

	"errors"

	"github.com/gin-gonic/gin"
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
	router.Use(p.MattermostAuthorizationRequired)

	router.GET("/ai_threads", p.handleGetAIThreads)
	router.GET("/ai_bots", p.handleGetAIBots)

	botRequriedRouter := router.Group("")
	botRequriedRouter.Use(p.aiBotRequired)

	postRouter := botRequriedRouter.Group("/post/:postid")
	postRouter.Use(p.postAuthorizationRequired)
	postRouter.POST("/react", p.handleReact)
	postRouter.POST("/summarize", p.handleSummarize)
	postRouter.POST("/transcribe/file/:fileid", p.handleTranscribeFile)
	postRouter.POST("/summarize_transcription", p.handleSummarizeTranscription)
	postRouter.POST("/stop", p.handleStop)
	postRouter.POST("/regenerate", p.handleRegenerate)

	channelRouter := botRequriedRouter.Group("/channel/:channelid")
	channelRouter.Use(p.channelAuthorizationRequired)
	channelRouter.POST("/since", p.handleSince)

	adminRouter := router.Group("/admin")
	adminRouter.Use(p.mattermostAdminAuthorizationRequired)

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

func (p *Plugin) handleGetAIThreads(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	p.botsLock.RLock()
	defer p.botsLock.RUnlock()
	dmChannelIDs := []string{}
	for _, bot := range p.bots {
		botDMChannel, err := p.pluginAPI.Channel.GetDirect(userID, bot.mmBot.UserId)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get DM with AI bot: %w", err))
			return
		}

		// Extra permissions checks are not totally nessiary since a user should always have permission to read their own DMs
		if !p.pluginAPI.User.HasPermissionToChannel(userID, botDMChannel.Id, model.PermissionReadChannel) {
			c.AbortWithError(http.StatusForbidden, errors.New("user doesn't have permission to read channel"))
			return
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
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	Username       string `json:"username"`
	LastIconUpdate int64  `json:"lastIconUpdate"`
	DMChannelID    string `json:"dmChannelID"`
}

func (p *Plugin) handleGetAIBots(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	ownedBots, err := p.pluginAPI.Bot.List(0, 1000, pluginapi.BotOwner("mattermost-ai"))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get bots: %w", err))
		return
	}

	// Get the info from all the bots.
	// Put the default bot first.
	bots := make([]AIBotInfo, len(ownedBots))
	defaultBotName := p.getConfiguration().DefaultBotName
	for i, bot := range ownedBots {
		direct, err := p.pluginAPI.Channel.GetDirect(userID, bot.UserId)
		if err != nil {
			p.API.LogError("unable to get direct channel for bot", "error", err)
			continue
		}
		bots[i] = AIBotInfo{
			ID:             bot.UserId,
			DisplayName:    bot.DisplayName,
			Username:       bot.Username,
			LastIconUpdate: bot.LastIconUpdate,
			DMChannelID:    direct.Id,
		}
		if bot.Username == defaultBotName {
			bots[0], bots[i] = bots[i], bots[0]
		}
	}

	c.JSON(http.StatusOK, bots)
}
