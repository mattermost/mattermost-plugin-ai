// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"reflect"

	"github.com/mattermost/mattermost-plugin-ai/agents"
	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/indexer"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mcp"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/search"
	"github.com/mattermost/mattermost-plugin-ai/streaming"
	"github.com/mattermost/mattermost/server/public/shared/httpservice"
)

// configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type configuration struct {
	agents.Config `json:"config"`
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	var clone = *c
	return &clone
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return fmt.Errorf("failed to load plugin configuration: %w", err)
	}

	p.setConfiguration(configuration)

	// If OnActivate hasn't run yet then don't do the change tasks
	if p.pluginAPI == nil {
		return nil
	}

	if err := p.bots.EnsureBots(configuration.Bots); err != nil {
		return fmt.Errorf("failed to ensure bots: %w", err)
	}

	if p.agentsService != nil {
		p.agentsService.SetConfiguration(&configuration.Config)
		if err := p.agentsService.OnConfigurationChange(); err != nil {
			return err
		}
	}

	// Handle MCP configuration changes
	oldMCPConfig := p.configuration.MCP
	newMCPConfig := configuration.MCP
	if !reflect.DeepEqual(oldMCPConfig, newMCPConfig) {
		// Close existing MCP client manager
		if p.mcpClientManager != nil {
			if err := p.mcpClientManager.Close(); err != nil {
				p.pluginAPI.Log.Error("Failed to close MCP client manager during configuration change", "error", err)
			}
		}

		// Reinitialize MCP client manager with new configuration
		mcpClient, err := mcp.NewClientManager(newMCPConfig, p.pluginAPI.Log)
		if err != nil {
			p.pluginAPI.Log.Error("Failed to reinitialize MCP client manager, MCP tools will be disabled", "error", err)
			p.mcpClientManager = nil
		} else {
			p.mcpClientManager = mcpClient
			p.pluginAPI.Log.Debug("MCP client manager reinitialized successfully")
		}
	}

	// Recreate search/indexer services if embedding configuration changed
	oldEmbedConfig := p.configuration.EmbeddingSearchConfig
	newEmbedConfig := configuration.EmbeddingSearchConfig

	if !reflect.DeepEqual(oldEmbedConfig, newEmbedConfig) {
		// Reinitialize search infrastructure
		var searchInfrastructure embeddings.EmbeddingSearch
		licenseChecker := enterprise.NewLicenseChecker(p.pluginAPI)
		if newEmbedConfig.Type != "" {
			untrustedHTTPClient := httpservice.MakeHTTPServicePlugin(p.API).MakeClient(false)

			var err error
			searchInfrastructure, err = search.InitSearch(p.db, untrustedHTTPClient, search.Config{
				EmbeddingSearchConfig: newEmbedConfig,
			}, licenseChecker)
			if err != nil {
				p.pluginAPI.Log.Error("failed to reinitialize search infrastructure", "error", err)
				// Continue without search functionality
			}
		}

		// Recreate indexer service
		mmClient := mmapi.NewClient(p.pluginAPI)
		p.indexerService = indexer.New(searchInfrastructure, mmClient, p.bots, p.db)

		// Recreate search service if search infrastructure is available
		if searchInfrastructure != nil {
			prompts, err := llm.NewPrompts(llm.PromptsFolder)
			if err != nil {
				p.pluginAPI.Log.Error("failed to initialize prompts", "error", err)
				return err
			}
			i18nBundle := i18n.Init()
			streamingService := streaming.NewMMPostStreamService(mmClient, i18nBundle, nil)

			p.searchService = search.New(
				searchInfrastructure,
				mmClient,
				prompts,
				streamingService,
				p.agentsService.GetLLM,
				p.llmUpstreamHTTPClient,
				p.db,
				licenseChecker,
			)
		} else {
			p.searchService = nil
		}
	}

	return nil
}
