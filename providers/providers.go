// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package providers

import (
	"net/http"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/anthropic"
	"github.com/mattermost/mattermost-plugin-ai/asage"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/metrics"
	"github.com/mattermost/mattermost-plugin-ai/openai"
)

const (
	StreamingTimeoutDefault = 10 * time.Second
)

func OpenAIConfigFromServiceConfig(serviceConfig llm.ServiceConfig) openai.Config {
	streamingTimeout := StreamingTimeoutDefault
	if serviceConfig.StreamingTimeoutSeconds > 0 {
		streamingTimeout = time.Duration(serviceConfig.StreamingTimeoutSeconds) * time.Second
	}

	return openai.Config{
		APIKey:           serviceConfig.APIKey,
		APIURL:           serviceConfig.APIURL,
		OrgID:            serviceConfig.OrgID,
		DefaultModel:     serviceConfig.DefaultModel,
		InputTokenLimit:  serviceConfig.InputTokenLimit,
		OutputTokenLimit: serviceConfig.OutputTokenLimit,
		StreamingTimeout: streamingTimeout,
		SendUserID:       serviceConfig.SendUserID,
	}
}

// CreateLanguageModel creates a language model based on the bot configuration
func CreateLanguageModel(botConfig llm.BotConfig, httpClient *http.Client, llmMetrics metrics.LLMetrics) llm.LanguageModel {
	var result llm.LanguageModel
	switch botConfig.Service.Type {
	case llm.ServiceTypeOpenAI:
		result = openai.New(OpenAIConfigFromServiceConfig(botConfig.Service), httpClient, llmMetrics)
	case llm.ServiceTypeOpenAICompatible:
		result = openai.NewCompatible(OpenAIConfigFromServiceConfig(botConfig.Service), httpClient, llmMetrics)
	case llm.ServiceTypeAzure:
		result = openai.NewAzure(OpenAIConfigFromServiceConfig(botConfig.Service), httpClient, llmMetrics)
	case llm.ServiceTypeAnthropic:
		result = anthropic.New(botConfig.Service, httpClient, llmMetrics)
	case llm.ServiceTypeASage:
		result = asage.New(botConfig.Service, httpClient, llmMetrics)
	}

	return result
}
