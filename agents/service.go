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
	"github.com/mattermost/mattermost-plugin-ai/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/metrics"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/openai"
	"github.com/mattermost/mattermost-plugin-ai/providers"
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
	contextBuilder *LLMContextBuilder,
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
		contextBuilder:        contextBuilder,
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

	return agentsService, nil
}

func (p *AgentsService) GetPrompts() *llm.Prompts {
	return p.prompts
}

func (p *AgentsService) OnDeactivate() error {
	return nil
}

// SetAPI sets the API for testing
func (p *AgentsService) SetAPI(api plugin.API) {
	p.pluginAPI = pluginapi.NewClient(api, nil)
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

// GetLLM creates and returns a language model for the given bot configuration
func (p *AgentsService) GetLLM(botConfig llm.BotConfig) llm.LanguageModel {
	llmMetrics := p.metricsService.GetMetricsForAIService(botConfig.Name)

	result := providers.CreateLanguageModel(botConfig, p.llmUpstreamHTTPClient, llmMetrics)

	cfg := p.getConfiguration()
	if cfg.EnableLLMTrace {
		result = providers.NewLanguageModelLogWrapper(p.pluginAPI.Log, result)
	}

	result = providers.NewLLMTruncationWrapper(result)

	return result
}

// getTranscribe creates a transcriber for the configured transcript generator bot
func (p *AgentsService) getTranscribe() Transcriber {
	cfg := p.getConfiguration()
	var botConfig llm.BotConfig

	// Find the bot configuration for transcript generation
	found := false
	for _, bot := range cfg.Bots {
		if bot.Name == cfg.TranscriptGenerator {
			botConfig = bot
			found = true
			break
		}
	}

	// Check if a valid bot configuration was found
	if !found || cfg.TranscriptGenerator == "" {
		p.pluginAPI.Log.Error("No transcript generator bot found", "configured_generator", cfg.TranscriptGenerator)
		return nil
	}

	// Check if the service type is configured
	if botConfig.Service.Type == "" {
		p.pluginAPI.Log.Error("Transcript generator bot has no service type configured", "bot_name", botConfig.Name)
		return nil
	}

	llmMetrics := p.metricsService.GetMetricsForAIService(botConfig.Name)
	switch botConfig.Service.Type {
	case llm.ServiceTypeOpenAI:
		return openai.New(providers.OpenAIConfigFromServiceConfig(botConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeOpenAICompatible:
		return openai.NewCompatible(providers.OpenAIConfigFromServiceConfig(botConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeAzure:
		return openai.NewAzure(providers.OpenAIConfigFromServiceConfig(botConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	default:
		p.pluginAPI.Log.Error("Unsupported service type for transcript generator",
			"bot_name", botConfig.Name,
			"service_type", botConfig.Service.Type)
		return nil
	}
}
