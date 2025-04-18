// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"time"

	"github.com/mattermost/mattermost-plugin-ai/anthropic"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/openai"
)

const (
	StreamingTimeoutDefault = 10 * time.Second
)

func openaiConfigFromLLMService(llmService llm.ServiceConfig) openai.Config {
	streamingTimeout := StreamingTimeoutDefault
	if llmService.StreamingTimeoutSeconds > 0 {
		streamingTimeout = time.Duration(llmService.StreamingTimeoutSeconds) * time.Second
	}

	return openai.Config{
		APIKey:           llmService.APIKey,
		APIURL:           llmService.APIURL,
		OrgID:            llmService.OrgID,
		DefaultModel:     llmService.DefaultModel,
		InputTokenLimit:  llmService.InputTokenLimit,
		OutputTokenLimit: llmService.OutputTokenLimit,
		StreamingTimeout: streamingTimeout,
		SendUserID:       llmService.SendUserID,
	}
}

func (p *AgentsService) GetLLM(llmBotConfig llm.BotConfig) llm.LanguageModel {
	llmMetrics := p.metricsService.GetMetricsForAIService(llmBotConfig.Name)

	var result llm.LanguageModel
	switch llmBotConfig.Service.Type {
	case llm.ServiceTypeOpenAI:
		result = openai.New(openaiConfigFromLLMService(llmBotConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeOpenAICompatible:
		result = openai.NewCompatible(openaiConfigFromLLMService(llmBotConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeAzure:
		result = openai.NewAzure(openaiConfigFromLLMService(llmBotConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeAnthropic:
		result = anthropic.New(llmBotConfig.Service, p.llmUpstreamHTTPClient, llmMetrics)
	}

	cfg := p.getConfiguration()
	if cfg.EnableLLMTrace {
		result = NewLanguageModelLogWrapper(p.pluginAPI.Log, result)
	}

	result = NewLLMTruncationWrapper(result)

	return result
}

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
		return openai.New(openaiConfigFromLLMService(botConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeOpenAICompatible:
		return openai.NewCompatible(openaiConfigFromLLMService(botConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	case llm.ServiceTypeAzure:
		return openai.NewAzure(openaiConfigFromLLMService(botConfig.Service), p.llmUpstreamHTTPClient, llmMetrics)
	default:
		p.pluginAPI.Log.Error("Unsupported service type for transcript generator",
			"bot_name", botConfig.Name,
			"service_type", botConfig.Service.Type)
		return nil
	}
}
