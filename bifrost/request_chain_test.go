// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bifrost

import (
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

func TestRequestChain(t *testing.T) {
	customAnthropic := customProviderName(schemas.Anthropic, "svc2")
	baseProviders := map[schemas.ModelProvider]schemas.ModelProvider{
		schemas.OpenAI:    schemas.OpenAI,
		schemas.Anthropic: schemas.Anthropic,
		schemas.Gemini:    schemas.Gemini,
		schemas.Bedrock:   schemas.Bedrock,
		customAnthropic:   schemas.Anthropic,
	}
	tests := []struct {
		name            string
		provider        schemas.ModelProvider
		fallbacks       []schemas.Fallback
		imageCount      int
		wantImageLimit  int
		wantStrict      bool
		wantPromptCache bool
	}{
		{
			name:     "Gemini alone has no image limit or strict rules",
			provider: schemas.Gemini,
		},
		{
			name:           "OpenAI alone uses its own limit",
			provider:       schemas.OpenAI,
			wantImageLimit: openaiMaxImageDimension,
		},
		{
			name:           "custom Anthropic fallback name resolves to Anthropic",
			provider:       schemas.Gemini,
			fallbacks:      []schemas.Fallback{{Provider: customAnthropic, Model: "claude"}},
			wantImageLimit: anthropicMaxImageDimension,
			wantStrict:     true,
		},
		{
			name:           "lowest limit across hops wins",
			provider:       schemas.OpenAI,
			fallbacks:      []schemas.Fallback{{Provider: schemas.Anthropic, Model: "claude"}},
			imageCount:     anthropicManyImageThreshold + 1,
			wantImageLimit: anthropicManyImageDimension,
			wantStrict:     true,
		},
		{
			name:           "Bedrock fallback makes the chain strict",
			provider:       schemas.OpenAI,
			fallbacks:      []schemas.Fallback{{Provider: schemas.Bedrock, Model: "anthropic.claude"}},
			wantImageLimit: anthropicMaxImageDimension,
			wantStrict:     true,
		},
		{
			name:            "all-Anthropic chain caches prompts",
			provider:        schemas.Anthropic,
			fallbacks:       []schemas.Fallback{{Provider: customAnthropic, Model: "claude"}},
			wantImageLimit:  anthropicMaxImageDimension,
			wantStrict:      true,
			wantPromptCache: true,
		},
		{
			name:           "Bedrock fallback disables Anthropic prompt caching",
			provider:       schemas.Anthropic,
			fallbacks:      []schemas.Fallback{{Provider: schemas.Bedrock, Model: "anthropic.claude"}},
			wantImageLimit: anthropicMaxImageDimension,
			wantStrict:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LLM{provider: tt.provider, fallbacks: testHops(tt.fallbacks, nil), baseProviders: baseProviders}
			files := make([]llm.File, tt.imageCount)
			for i := range files {
				files[i] = llm.File{MimeType: "image/png"}
			}
			request := llm.CompletionRequest{Posts: []llm.Post{{Role: llm.PostRoleUser, Message: "hi", Files: files}}}

			chain := b.requestChain(request)

			assert.Equal(t, tt.fallbacks, chain.fallbacks)
			assert.Equal(t, tt.wantImageLimit, chain.imageDimensionLimit)
			assert.Equal(t, tt.wantStrict, chain.strictMessages)
			assert.Equal(t, tt.wantPromptCache, chain.promptCaching)
		})
	}
}
