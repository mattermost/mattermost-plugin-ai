// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bedrock

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/stretchr/testify/assert"
)

func TestConfigFromLLMService(t *testing.T) {
	testCases := []struct {
		name       string
		input      llm.ServiceConfig
		expected   Config
	}{
		{
			name: "Basic Config",
			input: llm.ServiceConfig{
				APIKey:       "test-api-key",
				OrgID:        "test-api-secret",
				APIURL:       "us-west-2",
				DefaultModel: "anthropic.claude-3-sonnet-20240229-v1:0",
				InputTokenLimit: 150000,
				OutputTokenLimit: 4096,
				StreamingTimeoutSeconds: 15,
				SendUserID: true,
			},
			expected: Config{
				APIKey:           "test-api-key",
				APISecret:        "test-api-secret",
				Region:           "us-west-2",
				DefaultModel:     "anthropic.claude-3-sonnet-20240229-v1:0",
				InputTokenLimit:  150000,
				OutputTokenLimit: 4096,
				StreamingTimeout: 15 * 1000000000, // 15 seconds in nanoseconds
				SendUserID:       true,
			},
		},
		{
			name: "Minimal Config",
			input: llm.ServiceConfig{
				APIKey: "test-api-key",
				OrgID:  "test-api-secret",
				APIURL: "us-east-1",
			},
			expected: Config{
				APIKey:           "test-api-key",
				APISecret:        "test-api-secret",
				Region:           "us-east-1",
				DefaultModel:     "anthropic.claude-3-sonnet-20240229-v1:0", // Default model
				InputTokenLimit:  0,
				OutputTokenLimit: 0,
				StreamingTimeout: 10 * 1000000000, // 10 seconds in nanoseconds
				SendUserID:       false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := configFromLLMService(tc.input)
			assert.Equal(t, tc.expected.APIKey, config.APIKey)
			assert.Equal(t, tc.expected.APISecret, config.APISecret)
			assert.Equal(t, tc.expected.Region, config.Region)
			assert.Equal(t, tc.expected.DefaultModel, config.DefaultModel)
			assert.Equal(t, tc.expected.InputTokenLimit, config.InputTokenLimit)
			assert.Equal(t, tc.expected.OutputTokenLimit, config.OutputTokenLimit)
			assert.Equal(t, tc.expected.SendUserID, config.SendUserID)
		})
	}
}

func TestInputTokenLimit(t *testing.T) {
	testCases := []struct {
		name          string
		config        Config
		expectedLimit int
	}{
		{
			name: "Config With Input Token Limit",
			config: Config{
				InputTokenLimit: 50000,
				DefaultModel:    "anthropic.claude-3-sonnet-20240229-v1:0",
			},
			expectedLimit: 50000,
		},
		{
			name: "Claude 3 Opus",
			config: Config{
				DefaultModel: "anthropic.claude-3-opus-20240229-v1:0",
			},
			expectedLimit: 200000,
		},
		{
			name: "Claude 3 Sonnet",
			config: Config{
				DefaultModel: "anthropic.claude-3-sonnet-20240229-v1:0",
			},
			expectedLimit: 180000,
		},
		{
			name: "Claude 3 Haiku",
			config: Config{
				DefaultModel: "anthropic.claude-3-haiku-20240307-v1:0",
			},
			expectedLimit: 150000,
		},
		{
			name: "Claude 2",
			config: Config{
				DefaultModel: "anthropic.claude-2",
			},
			expectedLimit: 100000,
		},
		{
			name: "Titan Model",
			config: Config{
				DefaultModel: "amazon.titan-text-express-v1",
			},
			expectedLimit: 32000,
		},
		{
			name: "Unknown Model",
			config: Config{
				DefaultModel: "something-else",
			},
			expectedLimit: 100000, // Default fallback
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := &Bedrock{config: tc.config}
			assert.Equal(t, tc.expectedLimit, b.InputTokenLimit())
		})
	}
}

func TestGetDefaultConfig(t *testing.T) {
	testCases := []struct {
		name               string
		config             Config
		expectedModel      string
		expectedMaxTokens  int
	}{
		{
			name: "With Output Token Limit",
			config: Config{
				DefaultModel:     "anthropic.claude-3-sonnet-20240229-v1:0",
				OutputTokenLimit: 8192,
			},
			expectedModel:     "anthropic.claude-3-sonnet-20240229-v1:0",
			expectedMaxTokens: 8192,
		},
		{
			name: "Without Output Token Limit",
			config: Config{
				DefaultModel: "anthropic.claude-3-sonnet-20240229-v1:0",
			},
			expectedModel:     "anthropic.claude-3-sonnet-20240229-v1:0",
			expectedMaxTokens: 4096, // Default value
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := &Bedrock{config: tc.config}
			cfg := b.GetDefaultConfig()
			assert.Equal(t, tc.expectedModel, cfg.Model)
			assert.Equal(t, tc.expectedMaxTokens, cfg.MaxGeneratedTokens)
		})
	}
}

func TestCountTokens(t *testing.T) {
	b := &Bedrock{}
	
	testCases := []struct {
		name  string
		input string
		minExpected int
		maxExpected int // Using a range because exact token counts can vary
	}{
		{
			name:  "Empty String",
			input: "",
			minExpected: 0,
			maxExpected: 0,
		},
		{
			name:  "Short Text",
			input: "Hello, world!",
			minExpected: 2,
			maxExpected: 4,
		},
		{
			name:  "Longer Text",
			input: "This is a longer text that should have more tokens than the short text above. It contains multiple sentences and should give us a more realistic token count.",
			minExpected: 20,
			maxExpected: 40,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			count := b.CountTokens(tc.input)
			assert.GreaterOrEqual(t, count, tc.minExpected)
			assert.LessOrEqual(t, count, tc.maxExpected)
		})
	}
}