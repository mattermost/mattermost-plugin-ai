// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"fmt"
	"net/http"
	"os/exec"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/httpexternal"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mcp"
	"github.com/mattermost/mattermost-plugin-ai/metrics"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	BotUsername = "ai"

	CallsRecordingPostType = "custom_calls_recording"
	CallsBotUsername       = "calls"
	ZoomBotUsername        = "zoom"

	ffmpegPluginPath = "./plugins/mattermost-ai/dist/ffmpeg"
)

type AgentsService struct { //nolint:revive
	configuration     *Config
	configurationLock sync.RWMutex

	// Streamlined API client
	pluginAPI *pluginapi.Client
	API       plugin.API

	ffmpegPath string

	db      *sqlx.DB
	builder sq.StatementBuilderType

	prompts *llm.Prompts

	// streamingContexts maps post IDs to their streaming contexts
	// All operations on this map MUST be protected by streamingContextsMutex
	// The map is initialized in OnActivate and its accesses are protected
	// in stopPostStreaming, getPostStreamingContext, and finishPostStreaming
	streamingContexts      map[string]PostStreamContext
	streamingContextsMutex sync.Mutex

	licenseChecker *enterprise.LicenseChecker
	metricsService metrics.Metrics

	botsLock sync.RWMutex
	bots     []*Bot

	i18n *i18n.Bundle

	llmUpstreamHTTPClient *http.Client
	untrustedHTTPClient   *http.Client
	search                embeddings.EmbeddingSearch

	mcpClientManager *mcp.ClientManager

	contextBuilder *LLMContextBuilder
}

func resolveffmpegPath() string {
	_, standardPathErr := exec.LookPath("ffmpeg")
	if standardPathErr != nil {
		_, pluginPathErr := exec.LookPath(ffmpegPluginPath)
		if pluginPathErr != nil {
			return ""
		}
		return ffmpegPluginPath
	}

	return "ffmpeg"
}

func NewAgentsService(
	originalAPI plugin.API,
	api *pluginapi.Client,
	llmUpstreamHTTPClient *http.Client,
	untrustedHTTPClient *http.Client,
	metricsService metrics.Metrics,
	configuration *Config,
) (*AgentsService, error) {
	agentsService := &AgentsService{
		API:                   originalAPI,
		pluginAPI:             api,
		llmUpstreamHTTPClient: llmUpstreamHTTPClient,
		untrustedHTTPClient:   untrustedHTTPClient,
		metricsService:        metricsService,
		configuration:         configuration,
	}

	agentsService.licenseChecker = enterprise.NewLicenseChecker(agentsService.pluginAPI)

	// Initialize i18n - I18nInit doesn't return an error, but we should be consistent in handling it properly
	agentsService.i18n = i18n.Init()
	if agentsService.i18n == nil {
		return nil, fmt.Errorf("failed to initialize i18n bundle")
	}

	if err := agentsService.MigrateServicesToBots(); err != nil {
		agentsService.pluginAPI.Log.Error("failed to migrate services to bots", "error", err)
		// Don't fail on migration errors
	}

	if err := agentsService.EnsureBots(); err != nil {
		agentsService.pluginAPI.Log.Error("Failed to ensure bots", "error", err)
		// Don't fail on ensure bots errors as this leaves the plugin in an awkward state
		// where it can't be configured from the system console.
	}

	if err := agentsService.SetupDB(); err != nil {
		return nil, err
	}

	var err error
	agentsService.prompts, err = llm.NewPrompts(llm.PromptsFolder)
	if err != nil {
		return nil, err
	}

	agentsService.ffmpegPath = resolveffmpegPath()
	if agentsService.ffmpegPath == "" {
		agentsService.pluginAPI.Log.Error("ffmpeg not installed, transcriptions will be disabled.", "error", err)
	}

	agentsService.streamingContexts = map[string]PostStreamContext{}

	// Initialize search if configured
	agentsService.search, err = agentsService.initSearch()
	if err != nil {
		// Only log the error but don't fail plugin activation
		agentsService.pluginAPI.Log.Error("Failed to initialize search, search features will be disabled", "error", err)
	}

	// Initialize MCP client manager
	cfg := agentsService.getConfiguration()
	mcpClient, err := mcp.NewClientManager(cfg.MCP, agentsService.pluginAPI.Log)
	if err != nil {
		agentsService.pluginAPI.Log.Error("Failed to initialize MCP client manager, MCP tools will be disabled", "error", err)
	} else {
		agentsService.mcpClientManager = mcpClient
	}

	// Determine which MCP tool provider to use
	var mcpToolProvider MCPToolProvider
	if agentsService.mcpClientManager != nil {
		mcpToolProvider = agentsService.mcpClientManager
	}

	agentsService.contextBuilder = NewLLMContextBuilder(
		agentsService.pluginAPI,
		agentsService,   // builtInProvider
		mcpToolProvider, // mcpToolProvider - only pass if not nil
		agentsService,   // configProvider
	)

	return agentsService, nil
}

func (p *AgentsService) GetPrompts() *llm.Prompts {
	return p.prompts
}

func (p *AgentsService) OnDeactivate() error {
	// Clean up MCP client manager if it exists
	if p.mcpClientManager != nil {
		if err := p.mcpClientManager.Close(); err != nil {
			p.pluginAPI.Log.Error("Failed to close MCP client manager during deactivation", "error", err)
		}
	}

	return nil
}

// SetAPI sets the API for testing
func (p *AgentsService) SetAPI(api plugin.API) {
	p.pluginAPI = pluginapi.NewClient(api, nil)
}

func (p *AgentsService) createExternalHTTPClient() *http.Client {
	return httpexternal.CreateRestrictedClient(p.untrustedHTTPClient, httpexternal.ParseAllowedHostnames(p.getConfiguration().AllowedUpstreamHostnames))
}

// GetContextBuilder returns the context builder for external use
func (p *AgentsService) GetContextBuilder() *LLMContextBuilder {
	return p.contextBuilder
}

// GetEnableLLMTrace returns whether LLM tracing is enabled
func (p *AgentsService) GetEnableLLMTrace() bool {
	return p.getConfiguration().EnableLLMTrace
}
