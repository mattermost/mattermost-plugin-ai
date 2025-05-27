// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mcp"
)

type Config struct {
	Services                 []llm.ServiceConfig              `json:"services"`
	Bots                     []llm.BotConfig                  `json:"bots"`
	DefaultBotName           string                           `json:"defaultBotName"`
	TranscriptGenerator      string                           `json:"transcriptBackend"`
	EnableLLMTrace           bool                             `json:"enableLLMTrace"`
	AllowedUpstreamHostnames string                           `json:"allowedUpstreamHostnames"`
	EmbeddingSearchConfig    embeddings.EmbeddingSearchConfig `json:"embeddingSearchConfig"`
	MCP                      mcp.Config                       `json:"mcp"`
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *Config) Clone() *Config {
	var clone = *c
	return &clone
}

func (p *AgentsService) getConfiguration() *Config {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &Config{}
	}

	return p.configuration
}

func (p *AgentsService) SetConfiguration(configuration *Config) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *AgentsService) OnConfigurationChange() error {
	// Reinitialize MCP client based on new configuration
	if p.mcpClientManager != nil {
		// Close existing client first
		closeErr := p.mcpClientManager.Close()
		if closeErr != nil {
			p.pluginAPI.Log.Error("Failed to close MCP client manager", "error", closeErr)
		}
	}

	mcpClient, err := mcp.NewClientManager(p.getConfiguration().MCP, p.pluginAPI.Log)
	if err != nil {
		// Log the error but don't fail plugin configuration
		p.pluginAPI.Log.Error("Failed to initialize MCP client manager, MCP tools will be disabled", "error", err)
		p.mcpClientManager = nil
	} else {
		p.mcpClientManager = mcpClient
		p.pluginAPI.Log.Debug("MCP client manager reinitialized successfully")
	}

	return nil
}
