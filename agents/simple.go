// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/llm"
)

// HandleInterPluginSimpleCompletion handles simple completion requests from other plugins
func (p *AgentsService) HandleInterPluginSimpleCompletion(systemPrompt, userPrompt, botUsername, userID string, parameters map[string]any) (string, error) {
	// If bot username is not provided, use the default bot
	if botUsername == "" {
		botUsername = p.getConfiguration().DefaultBotName
	}

	// Get the bot by username or use the first available bot
	bot := p.GetBotByUsernameOrFirst(botUsername)
	if bot == nil {
		return "", fmt.Errorf("failed to get bot: %s", botUsername)
	}

	// Get user information
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %v", err)
	}

	// Create a proper context for the LLM
	context := p.contextBuilder.BuildLLMContextUserRequest(
		bot,
		user,
		nil, // No channel for inter-plugin requests
		p.contextBuilder.WithLLMContextParameters(parameters),
	)

	// Add tools if not disabled
	if !bot.cfg.DisableTools {
		context.Tools = p.contextBuilder.GetToolsStoreForUser(bot, true, userID)
	}

	// Format system prompt using template
	formattedSystemPrompt, err := p.prompts.FormatString(systemPrompt, context)
	if err != nil {
		return "", fmt.Errorf("failed to format system prompt: %v", err)
	}

	formattedUserPrompt, err := p.prompts.FormatString(userPrompt, context)
	if err != nil {
		return "", fmt.Errorf("failed to format user prompt: %v", err)
	}

	// Create a completion request with system prompt and user prompt
	completionRequest := llm.CompletionRequest{
		Posts: []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: formattedSystemPrompt,
			},
			{
				Role:    llm.PostRoleUser,
				Message: formattedUserPrompt,
			},
		},
		Context: context,
	}

	// Execute the completion
	response, err := p.GetLLM(bot.cfg).ChatCompletionNoStream(completionRequest)
	if err != nil {
		return "", fmt.Errorf("completion failed: %v", err)
	}

	return response, nil
}
