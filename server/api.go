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

	interPluginRoute := router.Group("/inter-plugin")
	interPluginRoute.Use(p.interPluginAuthorizationRequired)
	interPluginRoute.POST("/completion", p.handleInterPluginCompletion)

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
	pluginSecret := c.GetHeader("Mattermost-Plugin-Secret")
	if pluginSecret != "" {
		if p.pluginSecret == pluginSecret {
			return
		}
	}
	c.AbortWithStatus(http.StatusUnauthorized)
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

		// Extra permissions checks are not totally necessary since a user should always have permission to read their own DMs
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
		direct, err := p.pluginAPI.Channel.GetDirect(userID, bot.mmBot.UserId)
		if err != nil {
			p.API.LogError("unable to get direct channel for bot", "error", err)
			continue
		}
		bots = append(bots, AIBotInfo{
			ID:                 bot.mmBot.UserId,
			DisplayName:        bot.mmBot.DisplayName,
			Username:           bot.mmBot.Username,
			LastIconUpdate:     bot.mmBot.LastIconUpdate,
			DMChannelID:        direct.Id,
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

func (p *Plugin) handleInterPluginCompletion(c *gin.Context) {
	type CompletionRequest struct {
		SystemPrompt    string                 `json:"systemPrompt"`
		UserPrompt      string                 `json:"userPrompt"`
		BotUsername     string                 `json:"botUsername"`
		RequesterUserID string                 `json:"requesterUserID"`
		Parameters      map[string]interface{} `json:"parameters"`
	}

	var req CompletionRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	// If bot username is not provided, use the default bot
	if req.BotUsername == "" {
		req.BotUsername = p.getConfiguration().DefaultBotName
	}

	// Get the bot by username or use the first available bot
	bot := p.GetBotByUsernameOrFirst(req.BotUsername)
	if bot == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get bot: %s", req.BotUsername)})
		return
	}

	userID := req.RequesterUserID
	if userID == "" {
		userID = bot.mmBot.UserId
	}

	// Get user information
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get user: %v", err)})
		return
	}

	// Create a proper context for the LLM
	context := p.BuildLLMContextUserRequest(
		bot,
		user,
		nil, // No channel for inter-plugin requests
		p.WithLLMContextParameters(req.Parameters),
	)

	// Add tools if not disabled
	if !bot.cfg.DisableTools {
		context.Tools = p.getDefaultToolsStore(bot, true)
	}

	// Format system prompt using template
	systemPrompt, err := p.prompts.FormatString(req.SystemPrompt, context)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to format system prompt: %v", err)})
		return
	}

	userPrompt, err := p.prompts.FormatString(req.UserPrompt, context)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to format user prompt: %v", err)})
		return
	}

	// Create a completion request with system prompt and user prompt
	completionRequest := llm.CompletionRequest{
		Posts: []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: systemPrompt,
			},
			{
				Role:    llm.PostRoleUser,
				Message: userPrompt,
			},
		},
		Context: context,
	}

	// Apply any custom parameters for the model
	options := []llm.LanguageModelOption{}
	if model, ok := req.Parameters["model"].(string); ok && model != "" {
		options = append(options, llm.WithModel(model))
	}
	if maxTokens, ok := req.Parameters["maxGeneratedTokens"].(float64); ok && maxTokens > 0 {
		options = append(options, llm.WithMaxGeneratedTokens(int(maxTokens)))
	}

	// Execute the completion
	response, err := p.getLLM(bot.cfg).ChatCompletionNoStream(completionRequest, options...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("completion failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}
