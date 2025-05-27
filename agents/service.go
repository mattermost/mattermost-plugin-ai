// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/httpexternal"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/indexer"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mcp"
	"github.com/mattermost/mattermost-plugin-ai/metrics"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/search"
	"github.com/mattermost/mattermost-plugin-ai/streaming"
	"github.com/mattermost/mattermost/server/public/model"
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

	pluginAPI *pluginapi.Client
	mmClient  mmapi.Client
	API       plugin.API

	ffmpegPath string

	db      *sqlx.DB
	builder sq.StatementBuilderType

	prompts *llm.Prompts

	// streamingService handles all post streaming functionality
	streamingService streaming.Service

	licenseChecker *enterprise.LicenseChecker
	metricsService metrics.Metrics

	i18n *i18n.Bundle

	llmUpstreamHTTPClient *http.Client
	untrustedHTTPClient   *http.Client
	search                embeddings.EmbeddingSearch

	// Services
	indexingService *indexer.Indexer
	searchService   *search.Search

	mcpClientManager *mcp.ClientManager

	contextBuilder *LLMContextBuilder

	bots *bots.MMBots
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
	bots *bots.MMBots,
) (*AgentsService, error) {
	agentsService := &AgentsService{
		API:                   originalAPI,
		pluginAPI:             api,
		mmClient:              mmapi.NewClient(api),
		llmUpstreamHTTPClient: llmUpstreamHTTPClient,
		untrustedHTTPClient:   untrustedHTTPClient,
		metricsService:        metricsService,
		configuration:         configuration,
		bots:                  bots,
	}

	agentsService.licenseChecker = enterprise.NewLicenseChecker(agentsService.pluginAPI)

	// Initialize i18n - I18nInit doesn't return an error, but we should be consistent in handling it properly
	agentsService.i18n = i18n.Init()
	if agentsService.i18n == nil {
		return nil, fmt.Errorf("failed to initialize i18n bundle")
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

	// Initialize streaming service
	agentsService.streamingService = streaming.NewMMPostStreamService(
		agentsService.mmClient,
		agentsService.i18n,
		func(botid, userID string, post *model.Post, respondingToPostID string) {
			agentsService.modifyPostForBot(botid, userID, post, respondingToPostID)
		},
	)

	// Initialize search if configured
	searchConfig := search.Config{
		EmbeddingSearchConfig: configuration.EmbeddingSearchConfig,
	}
	agentsService.search, err = search.InitSearch(agentsService.db, agentsService.llmUpstreamHTTPClient, searchConfig, agentsService.licenseChecker)
	if err != nil {
		// Only log the error but don't fail plugin activation
		agentsService.pluginAPI.Log.Error("Failed to initialize search, search features will be disabled", "error", err)
		agentsService.search = nil
	}

	// Initialize search service if search is configured
	if agentsService.search != nil {
		agentsService.searchService = search.New(
			agentsService.search,
			agentsService.mmClient,
			agentsService.prompts,
			agentsService.streamingService,
			agentsService.GetLLM,
			agentsService.llmUpstreamHTTPClient,
			agentsService.db,
			agentsService.licenseChecker,
		)
	}

	// Initialize indexing service with a bots adapter
	agentsService.indexingService = indexer.New(
		agentsService.search,
		agentsService.mmClient,
		agentsService.bots,
		agentsService.db,
	)

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

// GetMMClient returns the mmapi client for external use
func (p *AgentsService) GetMMClient() mmapi.Client {
	return p.mmClient
}

// StreamResultToNewDM streams result to a new direct message (exported wrapper)
func (p *AgentsService) StreamResultToNewDM(botid string, stream *llm.TextStreamResult, userID string, post *model.Post, respondingToPostID string) error {
	return p.streamingService.StreamToNewDM(context.TODO(), botid, stream, userID, post, respondingToPostID)
}

// SaveTitleAsync saves a title asynchronously (exported wrapper)
func (p *AgentsService) SaveTitleAsync(threadID, title string) {
	p.saveTitleAsync(threadID, title)
}

// GetEnableLLMTrace returns whether LLM tracing is enabled
func (p *AgentsService) GetEnableLLMTrace() bool {
	return p.getConfiguration().EnableLLMTrace
}

// IsAnyBot returns true if the given user is an AI bot.
func (p *AgentsService) IsAnyBot(userID string) bool {
	return p.bots.IsAnyBot(userID)
}

// GetBotByUsernameOrFirst retrieves the bot associated with the given bot username or the first bot if not found
func (p *AgentsService) GetBotByUsernameOrFirst(botUsername string) *bots.Bot {
	return p.bots.GetBotByUsernameOrFirst(botUsername)
}

// GetBotByID retrieves the bot associated with the given bot ID
func (p *AgentsService) GetBotByID(botID string) *bots.Bot {
	return p.bots.GetBotByID(botID)
}

// SetBotsForTesting sets the bots instance for testing purposes only
func (p *AgentsService) SetBotsForTesting(bots *bots.MMBots) {
	p.bots = bots
}
