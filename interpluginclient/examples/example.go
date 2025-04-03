// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// This is an example of how to use the interpluginclient package
// from another Mattermost plugin.

package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/interpluginclient"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// ExamplePlugin shows how to use the AI plugin from another plugin
type ExamplePlugin struct {
	plugin.MattermostPlugin
	aiClient *interpluginclient.Client
}

func (p *ExamplePlugin) OnActivate() error {
	// Initialize the AI client
	client, err := interpluginclient.NewClient(&p.MattermostPlugin)
	if err != nil {
		return fmt.Errorf("failed to create AI client: %w", err)
	}
	p.aiClient = client

	return nil
}

// SimpleCompletion shows a basic completion request
func (p *ExamplePlugin) SimpleCompletion(userID, prompt string) (string, error) {
	request := interpluginclient.CompletionRequest{
		UserPrompt:      prompt,
		RequesterUserID: userID,
	}

	return p.aiClient.Completion(request)
}

// AdvancedCompletion shows a more advanced completion request with custom parameters
func (p *ExamplePlugin) AdvancedCompletion(userID, prompt string) (string, error) {
	// Set custom parameters
	params := interpluginclient.CompletionParameters{
		Model:              "gpt-4",
		MaxGeneratedTokens: 1000,
	}

	request := interpluginclient.CompletionRequest{
		UserPrompt:      prompt,
		BotUsername:     "research-bot", // Use a specific bot if configured
		RequesterUserID: userID,
		Parameters:      params.ToMap(),
	}

	// Use context for timeout control
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	return p.aiClient.CompletionWithContext(ctx, request)
}

// HandleCommand processes a slash command using AI
func (p *ExamplePlugin) HandleCommand(userID, text string) string {
	prompt := fmt.Sprintf("Respond to this command: %s", text)

	response, err := p.SimpleCompletion(userID, prompt)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return response
}

// ProcessDocumentWithAI demonstrates processing a document with AI
func (p *ExamplePlugin) ProcessDocumentWithAI(userID, document string) (string, error) {
	// Create a prompt that includes instructions for processing the document
	prompt := fmt.Sprintf("Please analyze this document and extract the key points:\n\n%s", document)

	// Use the default timeout and bot
	return p.SimpleCompletion(userID, prompt)
}

// HandleErrorGracefully shows a more robust way to handle potential AI errors
func (p *ExamplePlugin) HandleErrorGracefully(userID, prompt string) string {
	response, err := p.SimpleCompletion(userID, prompt)

	if err != nil {
		// Check for specific error types
		if err == interpluginclient.ErrAIPluginNotAvailable {
			return "The AI service is currently unavailable. Please try again later."
		}

		// Handle timeouts or other errors
		return fmt.Sprintf("Sorry, I couldn't process your request: %v", err)
	}

	return response
}
