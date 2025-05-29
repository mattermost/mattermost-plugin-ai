// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package meetings

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/metrics"
	"github.com/mattermost/mattermost-plugin-ai/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	CallsRecordingPostType = "custom_calls_recording"
	CallsBotUsername       = "calls"
	ZoomBotUsername        = "zoom"
)

// Service handles meeting summarization and transcription functionality
type Service struct {
	pluginAPI        *pluginapi.Client
	streamingService streaming.Service
	prompts          *llm.Prompts
	bots             *bots.MMBots
	i18n             *i18n.Bundle
	metricsService   metrics.Metrics
	ffmpegPath       string
	db               *sqlx.DB
	builder          sq.StatementBuilderType
	contextBuilder   ContextBuilder

	// Configuration access - we'll need this for getTranscribe
	getConfiguration func() Config
	// LLM service provider access
	getLLM func(config llm.BotConfig) llm.LanguageModel
	// Function for botDMNonResponse
	botDMNonResponse func(botUserID, userID string, post *model.Post) error
	// Function for modifying posts
	modifyPostForBot func(botID, userID string, post *model.Post, respondingToPostID string)
	// Function for saving titles
	saveTitle func(postID, title string) error
	// Function for saving titles async
	saveTitleAsync func(postID, title string)
	// Function for getting bot by ID
	getBotByID func(userID string) *bots.Bot
	// Function for executing database queries
	execBuilder func(query sq.Sqlizer) (sql.Result, error)
}

// Config represents the configuration needed by the meetings service
type Config interface {
	GetTranscriptGenerator() string
	GetBots() []llm.BotConfig
}

// ContextBuilder represents the interface for building LLM contexts
type ContextBuilder interface {
	BuildLLMContextUserRequest(bot *bots.Bot, user *model.User, channel *model.Channel, options ...llm.ContextOption) *llm.Context
	WithLLMContextDefaultTools(bot *bots.Bot, isDM bool) llm.ContextOption
}

// NewService creates a new meetings service
func NewService(
	pluginAPI *pluginapi.Client,
	streamingService streaming.Service,
	prompts *llm.Prompts,
	bots *bots.MMBots,
	i18n *i18n.Bundle,
	metricsService metrics.Metrics,
	db *sqlx.DB,
	builder sq.StatementBuilderType,
	getConfiguration func() Config,
	getLLM func(config llm.BotConfig) llm.LanguageModel,
	contextBuilder ContextBuilder,
	botDMNonResponse func(botUserID, userID string, post *model.Post) error,
	modifyPostForBot func(botID, userID string, post *model.Post, respondingToPostID string),
	saveTitle func(postID, title string) error,
	saveTitleAsync func(postID, title string),
	getBotByID func(userID string) *bots.Bot,
	execBuilder func(query sq.Sqlizer) (sql.Result, error),
) *Service {
	service := &Service{
		pluginAPI:        pluginAPI,
		streamingService: streamingService,
		prompts:          prompts,
		bots:             bots,
		i18n:             i18n,
		metricsService:   metricsService,
		db:               db,
		builder:          builder,
		getConfiguration: getConfiguration,
		getLLM:           getLLM,
		contextBuilder:   contextBuilder,
		botDMNonResponse: botDMNonResponse,
		modifyPostForBot: modifyPostForBot,
		saveTitle:        saveTitle,
		saveTitleAsync:   saveTitleAsync,
		getBotByID:       getBotByID,
		execBuilder:      execBuilder,
	}

	service.ffmpegPath = resolveFFMPEGPath()
	if service.ffmpegPath == "" {
		service.pluginAPI.Log.Error("ffmpeg not installed, transcriptions will be disabled.")
	}

	return service
}
