package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
)

type SimpleCompletionRequest struct {
	SystemPrompt    string         `json:"systemPrompt"`
	UserPrompt      string         `json:"userPrompt"`
	BotUsername     string         `json:"botUsername"`
	RequesterUserID string         `json:"requesterUserID"`
	Parameters      map[string]any `json:"parameters"`
}

func (p *Plugin) handleInterPluginSimpleCompletion(c *gin.Context) {
	var req SimpleCompletionRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	userID := req.RequesterUserID
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "requesterUserID is required"})
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

	// Execute the completion
	response, err := p.getLLM(bot.cfg).ChatCompletionNoStream(completionRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("completion failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}
