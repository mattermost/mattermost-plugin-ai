// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	// router.Use(p.MattermostAuthorizationRequired)
	router.Use(p.metricsMiddleware)

	router.GET("/ai_threads", p.handleGetAIThreads)
	router.GET("/ai_bots", p.handleGetAIBots)
	router.POST("/transform_webhook", p.handleTransformWebhook)
	router.POST("/smart-webhook/:id", p.handleSmartWebhook)

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

	// Get bot information
	botUsername := c.DefaultQuery("botUsername", p.getConfiguration().DefaultBotName)
	bot := p.GetBotByUsernameOrFirst(botUsername)
	if bot == nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get bot: %s", botUsername))
		return
	}

	// Read the JSON data from the request
	var rawJSON json.RawMessage
	if err := c.ShouldBindJSON(&rawJSON); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("failed to parse JSON: %w", err))
		return
	}

	// Validate and pretty print the JSON for better prompt formatting
	var jsonObj interface{}
	if err := json.Unmarshal(rawJSON, &jsonObj); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	prettyJSON, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to format JSON: %w", err))
		return
	}

	// Create the user context
	requestingUser, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get user: %w", err))
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
	conversation, err := p.prompts.ChatCompletion("incomming_webhook", conversationContext, p.getDefaultToolsStore(bot, false))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create AI conversation: %w", err))
		return
	}

	// Get the AI response using streaming
	resultStream, err := p.getLLM(bot.cfg).ChatCompletion(conversation)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Read all the streamed content
	result := resultStream.ReadAll()

	// Extract JSON between <webhook> and </webhook> tags
	start := "<webhook>"
	end := "</webhook>"
	startIndex := strings.Index(result, start)
	endIndex := strings.Index(result, end)

	if startIndex == -1 || endIndex == -1 || startIndex >= endIndex {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("AI response doesn't contain valid webhook markers: %s", result))
		return
	}

	// Extract the JSON content between the markers
	jsonContent := result[startIndex+len(start) : endIndex]
	jsonContent = strings.TrimSpace(jsonContent)

	// Validate that the extracted content is valid JSON
	var webhookPayload json.RawMessage
	if err := json.Unmarshal([]byte(jsonContent), &webhookPayload); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("AI generated invalid JSON between markers: %v\nContent: %s", err, jsonContent))
		return
	}

	// Create a direct message channel with the bot if it doesn't exist
	botDMChannel, err := p.pluginAPI.Channel.GetDirect(userID, bot.mmBot.UserId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get DM channel with bot: %w", err))
		return
	}

	// Parse the webhook payload
	var slackMsg map[string]interface{}
	if err := json.Unmarshal(webhookPayload, &slackMsg); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to parse webhook payload: %w", err))
		return
	}

	// Create a post using the transformed data
	post := &model.Post{
		UserId:    bot.mmBot.UserId,
		ChannelId: botDMChannel.Id,
		Message:   "",
	}

	// Add the text to the post message
	if text, ok := slackMsg["text"].(string); ok {
		post.Message = text
	}

	// Add the attachments directly to the post props
	if attachments, ok := slackMsg["attachments"]; ok {
		post.AddProp("attachments", attachments)
	}

	// Add the original JSON as a prop
	post.AddProp("original_json", string(prettyJSON))
	post.AddProp("transformed_json", string(webhookPayload))

	// Create the post
	if err := p.pluginAPI.Post.CreatePost(post); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create post: %w", err))
		return
	}

	// Return success with the post information
	result2 := struct {
		Message        string          `json:"message"`
		WebhookPayload json.RawMessage `json:"webhook_payload"`
		PostID         string          `json:"post_id"`
		ChannelID      string          `json:"channel_id"`
	}{
		Message:        "Successfully transformed and created post",
		WebhookPayload: webhookPayload,
		PostID:         post.Id,
		ChannelID:      post.ChannelId,
	}
	c.JSON(http.StatusOK, result2)
}

func (p *Plugin) handleSmartWebhook(c *gin.Context) {
	webhookID := c.Param("id")
	key := fmt.Sprintf("smart_webhook_%s", webhookID)

	// Get webhook data from KV store
	data, appErr := p.API.KVGet(key)
	if appErr != nil || data == nil {
		c.AbortWithError(http.StatusNotFound, fmt.Errorf("webhook not found"))
		return
	}

	// Parse the stored data
	parts := strings.Split(string(data), ",")
	if len(parts) != 3 {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("invalid webhook data format"))
		return
	}

	channelID := parts[0]
	username := parts[1]
	iconURL := parts[2]

	// Read JSON data from request
	var rawJSON json.RawMessage
	if err := c.ShouldBindJSON(&rawJSON); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("failed to parse JSON: %w", err))
		return
	}

	// Get bot information for AI transformation
	botUsername := p.getConfiguration().DefaultBotName
	bot := p.GetBotByUsernameOrFirst(botUsername)
	if bot == nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get bot"))
		return
	}

	// Pretty print JSON for better prompt formatting
	var jsonObj interface{}
	if err := json.Unmarshal(rawJSON, &jsonObj); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	prettyJSON, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to format JSON: %w", err))
		return
	}

	user, err := p.pluginAPI.User.Get(bot.mmBot.UserId)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid user"))
	}

	// Create AI context for transformation
	conversationContext := llm.ConversationContext{
		BotID:          bot.mmBot.UserId,
		Time:           time.Now().Format(time.RFC1123),
		RequestingUser: user,
		PromptParameters: map[string]string{
			"JSONData": string(prettyJSON),
		},
	}

	// Create AI conversation with the template
	conversation, err := p.prompts.ChatCompletion("incomming_webhook", conversationContext, p.getDefaultToolsStore(bot, false))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create AI conversation: %w", err))
		return
	}

	// Get AI response using streaming
	resultStream, err := p.getLLM(bot.cfg).ChatCompletion(conversation)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Read all streamed content
	result := resultStream.ReadAll()

	// Extract JSON between <webhook> and </webhook> tags
	start := "<webhook>"
	end := "</webhook>"
	startIndex := strings.Index(result, start)
	endIndex := strings.Index(result, end)

	if startIndex == -1 || endIndex == -1 || startIndex >= endIndex {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("AI response doesn't contain valid webhook markers: %s", result))
		return
	}

	// Extract the JSON content between the markers
	jsonContent := result[startIndex+len(start) : endIndex]
	jsonContent = strings.TrimSpace(jsonContent)

	// Validate the extracted content is valid JSON
	var webhookPayload json.RawMessage
	if err := json.Unmarshal([]byte(jsonContent), &webhookPayload); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("AI generated invalid JSON between markers: %v\nContent: %s", err, jsonContent))
		return
	}

	// Parse the webhook payload
	var slackMsg map[string]interface{}
	if err := json.Unmarshal(webhookPayload, &slackMsg); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to parse webhook payload: %w", err))
		return
	}

	// Create a post using the transformed data
	post := &model.Post{
		UserId:    bot.mmBot.UserId,
		ChannelId: channelID,
		Message:   "",
	}

	// Add the text to the post message
	if text, ok := slackMsg["text"].(string); ok {
		post.Message = text
	}

	// Add the attachments directly to the post props
	if attachments, ok := slackMsg["attachments"]; ok {
		post.AddProp("attachments", attachments)
	}

	// Set webhook props
	post.AddProp("from_webhook", "true")
	post.AddProp("override_username", username)
	if iconURL != "" {
		post.AddProp("override_icon_url", iconURL)
	}

	// Add the original JSON as a prop
	post.AddProp("original_json", string(prettyJSON))

	// Create the post
	if _, err := p.API.CreatePost(post); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create post: %w", err))
		return
	}

	// Return success
	c.JSON(http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Webhook processed successfully",
	})
}
