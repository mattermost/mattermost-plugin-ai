// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/conversations"
	"github.com/mattermost/mattermost-plugin-ai/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/metrics"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/providers"
	"github.com/mattermost/mattermost-plugin-ai/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	BotUsername = "ai"
)

type AgentsService struct { //nolint:revive
	configuration     *Config
	configurationLock sync.RWMutex

	pluginAPI *pluginapi.Client
	mmClient  mmapi.Client
	API       plugin.API

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

	// conversationService handles all conversation-related functionality
	conversationService *conversations.Conversations
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
	db *sqlx.DB,
	builder sq.StatementBuilderType,
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
		db:                    db,
		builder:               builder,
	}

	agentsService.licenseChecker = enterprise.NewLicenseChecker(agentsService.pluginAPI)

	// Initialize i18n - I18nInit doesn't return an error, but we should be consistent in handling it properly
	agentsService.i18n = i18n.Init()
	if agentsService.i18n == nil {
		return nil, fmt.Errorf("failed to initialize i18n bundle")
	}

	var err error
	agentsService.prompts, err = llm.NewPrompts(llm.PromptsFolder)
	if err != nil {
		return nil, err
	}

	// Initialize streaming service
	agentsService.streamingService = streaming.NewMMPostStreamService(
		agentsService.mmClient,
		agentsService.i18n,
		func(botid, userID string, post *model.Post, respondingToPostID string) {
			agentsService.modifyPostForBot(botid, userID, post, respondingToPostID)
		},
	)

	// Initialize conversations service
	agentsService.conversationService = conversations.New(
		agentsService.GetLLM,
		agentsService.prompts,
		agentsService.mmClient,
		agentsService.pluginAPI,
		agentsService.streamingService,
		agentsService.contextBuilder,
		agentsService.bots,
		agentsService.db,
		agentsService.builder,
		agentsService.licenseChecker,
		agentsService.i18n,
		func() string {
			return agentsService.getConfiguration().DefaultBotName
		},
		agentsService.checkUsageRestrictions,
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
	p.mmClient = mmapi.NewClient(p.pluginAPI)
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
	p.conversationService.SaveTitleAsync(threadID, title)
}

// saveTitleAsync is a compatibility wrapper for internal use
func (p *AgentsService) saveTitleAsync(threadID, title string) {
	p.conversationService.SaveTitleAsync(threadID, title)
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

// GetAllBots returns all bots
func (p *AgentsService) GetAllBots() []*bots.Bot {
	return p.bots.GetAllBots()
}

// GetDefaultBotName returns the default bot name
func (p *AgentsService) GetDefaultBotName() string {
	return p.getConfiguration().DefaultBotName
}

// CheckUsageRestrictionsForUser checks if a user can use a bot
func (p *AgentsService) CheckUsageRestrictionsForUser(bot *bots.Bot, userID string) error {
	return p.checkUsageRestrictionsForUser(bot, userID)
}

// SetBotsForTesting sets the bots instance for testing purposes only
func (p *AgentsService) SetBotsForTesting(botsInstance *bots.MMBots, pluginAPI *pluginapi.Client) {
	p.bots = botsInstance
	p.pluginAPI = pluginAPI

	// Initialize a minimal conversations service for testing if not already initialized
	if p.conversationService == nil {
		p.conversationService = conversations.New(
			func(cfg llm.BotConfig) llm.LanguageModel { return nil },
			&llm.Prompts{},
			nil,
			p.pluginAPI,
			nil,
			nil,
			botsInstance,
			nil,
			sq.StatementBuilder,
			nil,
			nil,
			func() string { return "ai" },
			// Provide a mock checkUsageRestrictions function for testing
			func(userID string, bot *bots.Bot, channel *model.Channel) error {
				// This is a simplified version for testing - the actual logic is in permissions.go
				return p.checkUsageRestrictions(userID, bot, channel)
			},
		)
	}
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

// Delegate methods to conversations service

// GetAIThreads delegates to the conversations service
func (p *AgentsService) GetAIThreads(userID string) ([]conversations.AIThread, error) {
	return p.conversationService.GetAIThreads(userID)
}

// IsBasicsLicensed delegates to the conversations service
func (p *AgentsService) IsBasicsLicensed() bool {
	return p.conversationService.IsBasicsLicensed()
}

// StopPostStreaming delegates to the conversations service
func (p *AgentsService) StopPostStreaming(postID string) {
	p.conversationService.StopPostStreaming(postID)
}

// CheckUsageRestrictions delegates to the conversations service
func (p *AgentsService) CheckUsageRestrictions(userID string, bot *bots.Bot, channel *model.Channel) error {
	return p.conversationService.CheckUsageRestrictions(userID, bot, channel)
}

// ProcessUserRequestToBot delegates to the conversations service
func (p *AgentsService) processUserRequestToBot(bot *bots.Bot, postingUser *model.User, channel *model.Channel, post *model.Post) (*llm.TextStreamResult, error) {
	return p.conversationService.ProcessUserRequest(bot, postingUser, channel, post)
}

// HandleRegenerate delegates to the conversations service
func (p *AgentsService) HandleRegenerate(userID string, post *model.Post, channel *model.Channel) error {
	return p.conversationService.HandleRegenerate(userID, post, channel)
}

// GetI18nBundle returns the i18n bundle for external use
func (p *AgentsService) GetI18nBundle() *i18n.Bundle {
	return p.i18n
}

func (p *AgentsService) GetI18n() *i18n.Bundle {
	return p.i18n
}

func (p *AgentsService) GetStreamingService() streaming.Service {
	return p.streamingService
}

// Public wrappers for methods needed by meetings service
func (p *AgentsService) BotDMNonResponse(botUserID, userID string, post *model.Post) error {
	return p.botDMNonResponse(botUserID, userID, post)
}

func (p *AgentsService) ModifyPostForBot(botID, userID string, post *model.Post, respondingToPostID string) {
	p.modifyPostForBot(botID, userID, post, respondingToPostID)
}

func (p *AgentsService) SaveTitle(postID, title string) error {
	return p.saveTitle(postID, title)
}

func (p *AgentsService) ExecBuilder(query sq.Sqlizer) (sql.Result, error) {
	return p.execBuilder(query)
}

// Constants moved to conversations package - re-export for compatibility
const (
	LLMRequesterUserID = conversations.LLMRequesterUserID
	NoRegen            = conversations.NoRegen
	RespondingToProp   = conversations.RespondingToProp
)

// Type aliases for compatibility
type AIThread = conversations.AIThread

// ExistingConversationToLLMPosts delegates to the conversations service
func (p *AgentsService) existingConversationToLLMPosts(bot *bots.Bot, conversation *mmapi.ThreadData, context *llm.Context) ([]llm.Post, error) {
	return p.conversationService.ExistingConversationToLLMPosts(bot, conversation, context)
}

// saveTitle saves a title for a thread
func (p *AgentsService) saveTitle(threadID, title string) error {
	// Delegate to conversations service
	return p.conversationService.SaveTitle(threadID, title)
}

// execBuilder is a helper for executing SQL builders
func (p *AgentsService) execBuilder(b interface {
	ToSql() (string, []interface{}, error)
}) (sql.Result, error) {
	sqlString, args, err := b.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build sql: %w", err)
	}

	sqlString = p.db.Rebind(sqlString)

	return p.db.Exec(sqlString, args...)
}
