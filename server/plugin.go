// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"embed"
	"encoding/json"
	"fmt"

	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/server/anthropic"
	"github.com/mattermost/mattermost-plugin-ai/server/bedrock"
	"github.com/mattermost/mattermost-plugin-ai/server/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/server/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
	"github.com/mattermost/mattermost-plugin-ai/server/openai"
	"github.com/mattermost/mattermost-plugin-ai/server/postgres"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/shared/httpservice"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	BotUsername = "ai"

	CallsRecordingPostType = "custom_calls_recording"
	CallsBotUsername       = "calls"
	ZoomBotUsername        = "zoom"

	ffmpegPluginPath = "./plugins/mattermost-ai/server/dist/ffmpeg"
)

//go:embed llm/prompts
var promptsFolder embed.FS

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	pluginAPI *pluginapi.Client

	ffmpegPath string

	db      *sqlx.DB
	builder sq.StatementBuilderType

	prompts *llm.Prompts

	streamingContexts      map[string]PostStreamContext
	streamingContextsMutex sync.Mutex

	licenseChecker *enterprise.LicenseChecker
	metricsService metrics.Metrics
	metricsHandler http.Handler

	botsLock sync.RWMutex
	bots     []*Bot

	i18n *i18n.Bundle

	llmUpstreamHTTPClient *http.Client
	search                embeddings.EmbeddingSearch
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

func (p *Plugin) OnActivate() error {
	p.pluginAPI = pluginapi.NewClient(p.API, p.Driver)

	p.licenseChecker = enterprise.NewLicenseChecker(p.pluginAPI)

	p.metricsService = metrics.NewMetrics(metrics.InstanceInfo{
		InstallationID: os.Getenv("MM_CLOUD_INSTALLATION_ID"),
		PluginVersion:  manifest.Version,
	})
	p.metricsHandler = metrics.NewMetricsHandler(p.GetMetrics())

	p.i18n = i18nInit()

	p.llmUpstreamHTTPClient = httpservice.MakeHTTPServicePlugin(p.API).MakeClient(true)
	p.llmUpstreamHTTPClient.Timeout = time.Minute * 10 // LLM requests can be slow

	if err := p.MigrateServicesToBots(); err != nil {
		p.pluginAPI.Log.Error("failed to migrate services to bots", "error", err)
		// Don't fail on migration errors
	}

	if err := p.EnsureBots(); err != nil {
		p.pluginAPI.Log.Error("Failed to ensure bots", "error", err)
		// Don't fail on ensure bots errors as this leaves the plugin in an awkward state
		// where it can't be configured from the system console.
	}

	if err := p.SetupDB(); err != nil {
		return err
	}

	var err error
	p.prompts, err = llm.NewPrompts(promptsFolder)
	if err != nil {
		return err
	}

	p.ffmpegPath = resolveffmpegPath()
	if p.ffmpegPath == "" {
		p.pluginAPI.Log.Error("ffmpeg not installed, transcriptions will be disabled.", "error", err)
	}

	p.streamingContexts = map[string]PostStreamContext{}

	// Initialize search if configured
	p.search, err = p.initSearch()
	if err != nil {
		// Only log the error but don't fail plugin activation
		p.pluginAPI.Log.Error("Failed to initialize search, search features will be disabled", "error", err)
	}

	return nil
}

// NewVectorStore creates a new vector store based on the provided configuration
func (p *Plugin) newVectorStore(config embeddings.UpstreamConfig, dimensions int) (embeddings.VectorStore, error) {
	switch config.Type { //nolint:gocritic
	case "pgvector":
		pgVectorConfig := postgres.PGVectorConfig{
			Dimensions: dimensions,
		}
		if err := json.Unmarshal(config.Parameters, &pgVectorConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pgvector config: %w", err)
		}
		return postgres.NewPGVector(p.db, pgVectorConfig)
	}

	return nil, fmt.Errorf("unsupported vector store type: %s", config.Type)
}

// NewEmbeddingProvider creates a new embedding provider based on the provided configuration
func (p *Plugin) newEmbeddingProvider(config embeddings.UpstreamConfig) (embeddings.EmbeddingProvider, error) {
	switch config.Type {
	case "openai-compatible":
		compatibleConfig := openai.Config{}
		if err := json.Unmarshal(config.Parameters, &compatibleConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal OpenAI-compatible config: %w", err)
		}
		return openai.NewCompatibleEmbeddings(compatibleConfig, p.llmUpstreamHTTPClient), nil
	case "openai":
		var openaiConfig openai.Config
		if err := json.Unmarshal(config.Parameters, &openaiConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal OpenAI config: %w", err)
		}
		return openai.NewCompatibleEmbeddings(openaiConfig, p.llmUpstreamHTTPClient), nil
	}

	return nil, fmt.Errorf("unsupported embedding provider type: %s", config.Type)
}

func (p *Plugin) initSearch() (embeddings.EmbeddingSearch, error) {
	cfg := p.getConfiguration()

	if cfg.EmbeddingSearchConfig.Type == "" {
		return nil, fmt.Errorf("search is disabled")
	}

	if !p.licenseChecker.IsBasicsLicensed() {
		return nil, fmt.Errorf("search is unavailable without a valid license")
	}

	switch cfg.EmbeddingSearchConfig.Type { //nolint:gocritic
	case "composite":
		vector, err := p.newVectorStore(cfg.EmbeddingSearchConfig.VectorStore, cfg.EmbeddingSearchConfig.Dimensions)
		if err != nil {
			return nil, err
		}
		embeddor, err := p.newEmbeddingProvider(cfg.EmbeddingSearchConfig.EmbeddingProvider)
		if err != nil {
			return nil, err
		}

		// Check if we have specific chunking options configured
		chunkingOpts := cfg.EmbeddingSearchConfig.ChunkingOptions
		if chunkingOpts.ChunkSize == 0 {
			chunkingOpts = embeddings.DefaultChunkingOptions()
		}

		return embeddings.NewCompositeSearch(vector, embeddor, chunkingOpts), nil
	}

	return nil, fmt.Errorf("unsupported search type: %s", cfg.EmbeddingSearchConfig.Type)
}

func (p *Plugin) getLLM(llmBotConfig llm.BotConfig) llm.LanguageModel {
	llmMetrics := p.metricsService.GetMetricsForAIService(llmBotConfig.Name)

	var result llm.LanguageModel
	switch llmBotConfig.Service.Type {
	case llm.ServiceTypeOpenAI:
		result = openai.New(llmBotConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeOpenAICompatible:
		result = openai.NewCompatible(llmBotConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeAzure:
		result = openai.NewAzure(llmBotConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeAnthropic:
		result = anthropic.New(llmBotConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeBedrock:
		result = bedrock.New(llmBotConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	}

	cfg := p.getConfiguration()
	if cfg.EnableLLMTrace {
		result = NewLanguageModelLogWrapper(p.pluginAPI.Log, result)
	}

	result = NewLLMTruncationWrapper(result)

	return result
}

func (p *Plugin) getTranscribe() Transcriber {
	cfg := p.getConfiguration()
	var botConfig llm.BotConfig
	for _, bot := range cfg.Bots {
		if bot.Name == cfg.TranscriptGenerator {
			botConfig = bot
			break
		}
	}
	llmMetrics := p.metricsService.GetMetricsForAIService(botConfig.Name)
	switch botConfig.Service.Type {
	case "openai":
		return openai.New(botConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	case "openaicompatible":
		return openai.NewCompatible(botConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	case "azure":
		return openai.NewAzure(botConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	}
	return nil
}
