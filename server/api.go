// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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
	router.Use(p.MattermostAuthorizationRequired)
	router.Use(p.metricsMiddleware)

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
	channelRouter.POST("/since", p.handleSince)

	adminRouter := router.Group("/admin")
	adminRouter.Use(p.mattermostAdminAuthorizationRequired)
	adminRouter.POST("/transform_webhook", p.handleTransformWebhook)

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

	c.JSON(http.StatusOK, bots)
}

func (p *Plugin) handleTransformWebhook(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	// Check if webhook URL is configured
	webhookURL := p.getConfiguration().IncomingWebhookURL
	if webhookURL == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "Incoming webhook URL not configured"})
		return
	}

	// Get bot information
	botUsername := c.DefaultQuery("botUsername", p.getConfiguration().DefaultBotName)
	bot := p.GetBotByUsernameOrFirst(botUsername)
	if bot == nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to get bot: %s", botUsername)})
		return
	}

	// Read the JSON data from the request
	var rawJSON json.RawMessage
	if err := c.ShouldBindJSON(&rawJSON); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Failed to parse JSON: %v", err)})
		return
	}

	// Validate and pretty print the JSON for better prompt formatting
	var jsonObj interface{}
	if err := json.Unmarshal(rawJSON, &jsonObj); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid JSON: %v", err)})
		return
	}
	prettyJSON, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to format JSON: %v", err)})
		return
	}

	// Create the user context
	requestingUser, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to get user: %v", err)})
		return
	}

	// Create the conversation context
	conversationContext := llm.ConversationContext{
		BotID:          bot.mmBot.UserId,
		Time:           time.Now().Format(time.RFC1123),
		RequestingUser: requestingUser,
		PromptParameters: map[string]string{
			"JSONData": string(prettyJSON),
		},
	}

	// Create the AI conversation with the template
	conversation, err := p.prompts.ChatCompletion("incomming_webhook", conversationContext, llm.ToolStore{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to create AI conversation: %v", err)})
		return
	}

	// Get the AI response
	result, err := bot.Complete(conversation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to complete AI conversation: %v", err)})
		return
	}

	// Validate that the AI response is valid JSON
	var webhookPayload json.RawMessage
	if err := json.Unmarshal([]byte(result), &webhookPayload); err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("AI generated invalid JSON: %v", err)})
		return
	}

	// Send the transformed JSON to the webhook URL
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(webhookPayload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to send to webhook: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusBadGateway, map[string]interface{}{
			"error":    fmt.Sprintf("Webhook returned error: %s", resp.Status),
			"response": string(body),
		})
		return
	}

	// Return success with the transformed JSON
	c.JSON(http.StatusOK, map[string]interface{}{
		"message":         "Successfully transformed and sent webhook",
		"webhook_payload": json.RawMessage(webhookPayload),
	})
}
