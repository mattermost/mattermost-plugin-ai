// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package interpluginclient provides a client for interacting with the Mattermost AI plugin
// from other Mattermost plugins.
package interpluginclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	// DefaultTimeout is the default timeout for all requests to the AI plugin
	DefaultTimeout = 30 * time.Second

	aiPluginID = "mattermost-ai"
)

// ErrAIPluginNotAvailable is returned when the AI plugin is not available or not properly configured
var ErrAIPluginNotAvailable = errors.New("AI plugin is not available or not properly configured")

// Client allows calling the AI plugin functions from other plugins
type Client struct {
	baseURL      string
	httpClient   *http.Client
	pluginSecret string
}

// CompletionRequest represents the data needed for an interplugin completion request
type CompletionRequest struct {
	// SystemPrompt is the text system prompt to send to the AI model
	SystemPrompt string `json:"systemPrompt"`

	// UserPrompt is the text user prompt to send to the AI model
	UserPrompt string `json:"userPrompt"`

	// BotUsername specifies which AI bot to use (optional, uses default bot if empty)
	BotUsername string `json:"botUsername,omitempty"`

	// RequesterUserID is the user ID of the user requesting the completion
	RequesterUserID string `json:"requesterUserID"`

	// Parameters allows customizing the completion behavior
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// CompletionResponse represents the response from an interplugin completion request
type CompletionResponse struct {
	Response string `json:"response"`
}

// CompletionParameters provides a type-safe way to configure completion requests
type CompletionParameters struct {
	// Model specifies which specific model to use
	Model string

	// MaxGeneratedTokens limits the maximum number of tokens generated in the response
	MaxGeneratedTokens int
}

// ToMap converts CompletionParameters to a map for the request
func (p CompletionParameters) ToMap() map[string]interface{} {
	params := map[string]interface{}{}
	if p.Model != "" {
		params["model"] = p.Model
	}
	if p.MaxGeneratedTokens > 0 {
		params["maxGeneratedTokens"] = float64(p.MaxGeneratedTokens)
	}
	return params
}

// CompletionWithContext sends a prompt to the AI plugin with context and returns the generated response
func (c *Client) CompletionWithContext(ctx context.Context, req CompletionRequest) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/inter-plugin/completion", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Mattermost-Plugin-Secret", c.pluginSecret)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var completionResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract the response from the map
	responseVal, ok := completionResp["response"]
	if !ok {
		return "", fmt.Errorf("response field missing from AI plugin response")
	}

	response, ok := responseVal.(string)
	if !ok {
		return "", fmt.Errorf("response field is not a string")
	}

	return response, nil
}

// Completion sends a prompt to the AI plugin and returns the generated response (with default timeout)
func (c *Client) Completion(req CompletionRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	return c.CompletionWithContext(ctx, req)
}

// NewClientFromPlugin creates a new Client using the plugin's API client
func NewClient(p *plugin.MattermostPlugin) (*Client, error) {
	// Get site URL from plugin config
	config := p.API.GetConfig()
	if config == nil || config.ServiceSettings.SiteURL == nil || *config.ServiceSettings.SiteURL == "" {
		return nil, errors.New("site URL not configured")
	}

	// Get the plugin secret from the KV store
	aiPluginConfig := config.PluginSettings.Plugins[aiPluginID]
	if aiPluginConfig == nil {
		return nil, errors.New("not inter plugin secret key found")
	}

	aiPluginConfig = aiPluginConfig["config"].(map[string]any)
	if aiPluginConfig == nil {
		return nil, errors.New("not inter plugin secret key found")
	}

	secret := aiPluginConfig["interPluginSecretKey"]
	if secret == nil {
		return nil, errors.New("not inter plugin secret key found")
	}

	baseURL, err := url.Parse(*config.ServiceSettings.SiteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid site URL: %w", err)
	}

	baseURL.Path = path.Join(baseURL.Path, "plugins", aiPluginID)

	return &Client{
		baseURL:      baseURL.String(),
		httpClient:   &http.Client{Timeout: DefaultTimeout},
		pluginSecret: secret.(string),
	}, nil
}
