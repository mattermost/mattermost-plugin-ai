// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"database/sql"
	"fmt"
	"net/http"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/conversations"
	"github.com/mattermost/mattermost-plugin-ai/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/llmcontext"
	"github.com/mattermost/mattermost-plugin-ai/metrics"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/streaming"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	BotUsername = "ai"
)

type AgentsService struct { //nolint:revive
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

	contextBuilder *llmcontext.LLMContextBuilder

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
	bots *bots.MMBots,
	contextBuilder *llmcontext.LLMContextBuilder,
	db *sqlx.DB,
	builder sq.StatementBuilderType,
	conversationService *conversations.Conversations,
) (*AgentsService, error) {
	agentsService := &AgentsService{
		API:                   originalAPI,
		pluginAPI:             api,
		mmClient:              mmapi.NewClient(api),
		llmUpstreamHTTPClient: llmUpstreamHTTPClient,
		untrustedHTTPClient:   untrustedHTTPClient,
		metricsService:        metricsService,
		bots:                  bots,
		contextBuilder:        contextBuilder,
		db:                    db,
		builder:               builder,
		conversationService:   conversationService,
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

	return agentsService, nil
}

// SetConversationService updates the conversation service (used during initialization)
func (p *AgentsService) SetConversationService(service *conversations.Conversations) {
	p.conversationService = service
}

// SetBotsForTesting sets the bots service for testing purposes only
func (p *AgentsService) SetBotsForTesting(botsService *bots.MMBots, client *pluginapi.Client) {
	p.bots = botsService
	p.pluginAPI = client
	p.mmClient = mmapi.NewClient(client)
}

func (p *AgentsService) ExecBuilder(query sq.Sqlizer) (sql.Result, error) {
	sqlString, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build sql: %w", err)
	}

	sqlString = p.db.Rebind(sqlString)
	return p.db.Exec(sqlString, args...)
}
