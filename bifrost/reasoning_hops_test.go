// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bifrost

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

// failingCaptureServer records the last request body it receives and rejects
// every request so Bifrost moves on to the next hop. A 400 fails over without
// Bifrost's retry backoff for server errors.
func failingCaptureServer(t *testing.T, body *atomic.Value) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(data, &parsed)
		body.Store(parsed)
		http.Error(w, `{"type":"error","error":{"type":"invalid_request_error","message":"rejected"}}`, http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)
	return server
}

// anthropicThinkingOnWire returns the thinking object and output_config.effort
// of a captured Anthropic Messages request.
func anthropicThinkingOnWire(body map[string]any) (thinking map[string]any, effort string) {
	thinking, _ = body["thinking"].(map[string]any)
	if outputConfig, ok := body["output_config"].(map[string]any); ok {
		effort, _ = outputConfig["effort"].(string)
	}
	return thinking, effort
}

// TestReasoningIsResolvedPerFallbackHop drives real failovers through Bifrost
// and checks the reasoning each hop receives. Bifrost reuses the primary's
// request parameters for fallbacks, so without per-hop resolution a setting
// valid for the primary model is sent unchanged to a fallback model that
// rejects it or interprets it differently.
func TestReasoningIsResolvedPerFallbackHop(t *testing.T) {
	type wantAnthropic struct {
		thinkingType string // "" means no thinking parameter
		budget       float64
		effort       string // "" means no effort
		display      string
	}
	tests := []struct {
		name             string
		primaryModel     string
		fallbackType     string
		fallbackModel    string
		reasoningEnabled bool
		effort           string
		nativeWebSearch  bool
		wantPrimary      wantAnthropic
		wantFallback     wantAnthropic
		wantGeminiThink  bool
	}{
		{
			name:         "Sonnet 5 off disables thinking but Opus 5.5 fallback gets low effort",
			primaryModel: "claude-sonnet-5", fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-opus-5-5",
			wantPrimary:  wantAnthropic{thinkingType: "disabled"},
			wantFallback: wantAnthropic{thinkingType: "adaptive", effort: "low", display: "omitted"},
		},
		{
			name:         "Opus 5.5 off uses low effort but Haiku fallback thinks only when asked",
			primaryModel: "claude-opus-5-5", fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-haiku-4-5-20251001",
			wantPrimary:  wantAnthropic{thinkingType: "adaptive", effort: "low", display: "omitted"},
			wantFallback: wantAnthropic{},
		},
		{
			name:         "xhigh is kept for Opus 4.7 and stepped down for an Opus 4.6 fallback",
			primaryModel: "claude-opus-4-7", fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-opus-4-6",
			reasoningEnabled: true, effort: "xhigh",
			wantPrimary:  wantAnthropic{thinkingType: "adaptive", effort: "xhigh", display: "summarized"},
			wantFallback: wantAnthropic{thinkingType: "adaptive", effort: "high"},
		},
		{
			name:         "max on a budget-only fallback uses the capped budget table",
			primaryModel: "claude-opus-4-7", fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-haiku-4-5-20251001",
			reasoningEnabled: true, effort: "max",
			wantPrimary:  wantAnthropic{thinkingType: "adaptive", effort: "max", display: "summarized"},
			wantFallback: wantAnthropic{thinkingType: "enabled", budget: 4096},
		},
		{
			name:         "Responses path resolves the fallback hop too",
			primaryModel: "claude-sonnet-5", fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-opus-5-5",
			nativeWebSearch: true,
			wantPrimary:     wantAnthropic{thinkingType: "disabled"},
			wantFallback:    wantAnthropic{thinkingType: "adaptive", effort: "low", display: "omitted"},
		},
		{
			name:         "Opus 5.5 off does not ask a Gemini fallback for thoughts",
			primaryModel: "claude-opus-5-5", fallbackType: llm.ServiceTypeGemini, fallbackModel: "gemini-2.5-flash",
			wantPrimary:     wantAnthropic{thinkingType: "adaptive", effort: "low", display: "omitted"},
			wantGeminiThink: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot := llm.BotConfig{ReasoningEnabled: tt.reasoningEnabled, ReasoningEffort: tt.effort}
			if tt.nativeWebSearch {
				bot.EnabledNativeTools = []string{"web_search"}
			}
			primaryBody, fallbackBody := captureFailoverHops(t,
				llm.ServiceConfig{Type: llm.ServiceTypeAnthropic, DefaultModel: tt.primaryModel},
				llm.ServiceConfig{Type: tt.fallbackType, DefaultModel: tt.fallbackModel},
				bot,
			)

			assertHop := func(label string, body map[string]any, want wantAnthropic) {
				thinking, effort := anthropicThinkingOnWire(body)
				assert.Equal(t, want.effort, effort, "%s effort", label)
				if want.thinkingType == "" {
					assert.Nil(t, thinking, "%s must not send thinking", label)
					return
				}
				require.NotNil(t, thinking, "%s thinking", label)
				assert.Equal(t, want.thinkingType, thinking["type"], "%s thinking type", label)
				if want.budget > 0 {
					assert.InDelta(t, want.budget, thinking["budget_tokens"], 0, "%s budget", label)
				}
				if want.display != "" {
					assert.Equal(t, want.display, thinking["display"], "%s display", label)
				}
			}

			assertHop("primary", primaryBody, tt.wantPrimary)
			if tt.fallbackType == llm.ServiceTypeGemini {
				generationConfig, _ := fallbackBody["generationConfig"].(map[string]any)
				thinkingConfig, _ := generationConfig["thinkingConfig"].(map[string]any)
				includeThoughts, _ := thinkingConfig["includeThoughts"].(bool)
				assert.Equal(t, tt.wantGeminiThink, includeThoughts, "Gemini includeThoughts")
				return
			}
			assertHop("fallback", fallbackBody, tt.wantFallback)
		})
	}
}

// captureFailoverHops sends one completion through the production wiring for
// a primary -> fallback -> local chain whose first two hops reject the
// request, and returns the request bodies the primary and fallback received.
// The service IDs, URLs, keys and fallback links are filled in here.
func captureFailoverHops(t *testing.T, primarySvc, fallbackSvc llm.ServiceConfig, bot llm.BotConfig) (primaryBody, fallbackBody map[string]any) {
	t.Helper()
	var primaryCapture, fallbackCapture atomic.Value
	var localHits atomic.Int32
	primaryServer := failingCaptureServer(t, &primaryCapture)
	fallbackServer := failingCaptureServer(t, &fallbackCapture)
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localHits.Add(1)
		chatCompletionSSE(w, "from-local")
	}))
	t.Cleanup(localServer.Close)

	primarySvc.ID, primarySvc.APIKey, primarySvc.APIURL, primarySvc.FallbackServiceID = "primary", "primary-key", primaryServer.URL, "fallback"
	fallbackSvc.ID, fallbackSvc.APIKey, fallbackSvc.APIURL, fallbackSvc.FallbackServiceID = "fallback", "fallback-key", fallbackServer.URL, "local"
	localSvc := llm.ServiceConfig{
		ID: "local", Type: llm.ServiceTypeOpenAICompatible, APIURL: localServer.URL, DefaultModel: "llama3",
	}
	bot.ID, bot.Name, bot.DisplayName, bot.ServiceID = "bot-1", "ai", "AI", "primary"

	llmInstance, err := NewFromServiceConfig(primarySvc, bot, []llm.ServiceConfig{fallbackSvc, localSvc})
	require.NoError(t, err)
	t.Cleanup(llmInstance.Shutdown)

	_, err = llmInstance.ChatCompletionNoStream(context.Background(), llm.CompletionRequest{
		Posts: []llm.Post{{Role: llm.PostRoleUser, Message: "hi"}},
	}, llm.WithToolsDisabled())
	require.NoError(t, err)
	require.Positive(t, localHits.Load(), "the request must fail over through both hops")

	primaryBody, ok := primaryCapture.Load().(map[string]any)
	require.True(t, ok, "primary hop must have been called")
	fallbackBody, ok = fallbackCapture.Load().(map[string]any)
	require.True(t, ok, "fallback hop must have been called")
	return primaryBody, fallbackBody
}

// maxTokensOnWire returns the max output tokens field of a captured request:
// max_tokens (Anthropic), max_completion_tokens (OpenAI chat) or
// max_output_tokens (OpenAI Responses).
func maxTokensOnWire(body map[string]any) (float64, bool) {
	for _, key := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens"} {
		if v, ok := body[key].(float64); ok {
			return v, true
		}
	}
	return 0, false
}

// TestMaxOutputTokensAreClampedPerFallbackHop drives real failovers through
// Bifrost and checks the max output tokens each hop receives. Bifrost reuses
// the primary's max tokens for fallbacks and never clamps them to the model,
// so a value valid for the primary is rejected by a fallback model with a
// lower output limit. Bifrost v1.7.15 knows output limits only for Claude
// models; other models keep the requested value.
func TestMaxOutputTokensAreClampedPerFallbackHop(t *testing.T) {
	tests := []struct {
		name             string
		primaryType      string
		primaryModel     string
		outputTokenLimit int
		fallbackType     string
		fallbackModel    string
		reasoningEnabled bool
		effort           string
		nativeWebSearch  bool
		wantPrimary      float64
		wantFallback     float64
		// wantFallbackBudget is the fallback's thinking budget; 0 means unchecked.
		wantFallbackBudget float64
	}{
		{
			name:        "chat path clamps a fallback with a lower limit than the primary",
			primaryType: llm.ServiceTypeAnthropic, primaryModel: "claude-opus-4-6", outputTokenLimit: 128000,
			fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-3-5-haiku-20241022",
			wantPrimary: 128000, wantFallback: 8192,
		},
		{
			name:        "Responses path clamps a fallback with a lower limit than the primary",
			primaryType: llm.ServiceTypeAnthropic, primaryModel: "claude-opus-4-6", outputTokenLimit: 128000,
			fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-3-5-haiku-20241022",
			nativeWebSearch: true,
			wantPrimary:     128000, wantFallback: 8192,
		},
		{
			name:        "OpenAI Responses primary clamps an Anthropic fallback",
			primaryType: llm.ServiceTypeOpenAI, primaryModel: "gpt-4o", outputTokenLimit: 64000,
			fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-3-7-sonnet-20250219",
			wantPrimary: 64000, wantFallback: 8192,
		},
		{
			name:        "fallback with a higher limit is not capped to the primary's",
			primaryType: llm.ServiceTypeAnthropic, primaryModel: "claude-3-5-haiku-20241022", outputTokenLimit: 64000,
			fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-sonnet-4-6",
			wantPrimary: 8192, wantFallback: 64000,
		},
		{
			name:        "thinking budget fits under the clamped fallback max_tokens",
			primaryType: llm.ServiceTypeAnthropic, primaryModel: "claude-opus-4-6", outputTokenLimit: 128000,
			fallbackType: llm.ServiceTypeAnthropic, fallbackModel: "claude-3-7-sonnet-20250219",
			reasoningEnabled: true, effort: "max",
			wantPrimary: 128000, wantFallback: 8192, wantFallbackBudget: 4096,
		},
		{
			name:        "fallback model without a known limit keeps the requested value",
			primaryType: llm.ServiceTypeAnthropic, primaryModel: "claude-3-5-haiku-20241022", outputTokenLimit: 64000,
			fallbackType: llm.ServiceTypeOpenAI, fallbackModel: "gpt-4o",
			wantPrimary: 8192, wantFallback: 64000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot := llm.BotConfig{ReasoningEnabled: tt.reasoningEnabled, ReasoningEffort: tt.effort}
			if tt.nativeWebSearch {
				bot.EnabledNativeTools = []string{"web_search"}
			}
			primaryBody, fallbackBody := captureFailoverHops(t,
				llm.ServiceConfig{Type: tt.primaryType, DefaultModel: tt.primaryModel, OutputTokenLimit: tt.outputTokenLimit},
				llm.ServiceConfig{Type: tt.fallbackType, DefaultModel: tt.fallbackModel},
				bot,
			)

			primaryMax, ok := maxTokensOnWire(primaryBody)
			require.True(t, ok, "primary must send max output tokens")
			assert.InDelta(t, tt.wantPrimary, primaryMax, 0, "primary max output tokens")
			fallbackMax, ok := maxTokensOnWire(fallbackBody)
			require.True(t, ok, "fallback must send max output tokens")
			assert.InDelta(t, tt.wantFallback, fallbackMax, 0, "fallback max output tokens")

			if tt.wantFallbackBudget > 0 {
				thinking, _ := anthropicThinkingOnWire(fallbackBody)
				require.NotNil(t, thinking, "fallback thinking")
				budget, _ := thinking["budget_tokens"].(float64)
				assert.InDelta(t, tt.wantFallbackBudget, budget, 0, "fallback thinking budget")
				assert.Less(t, budget, fallbackMax, "Anthropic requires budget_tokens below max_tokens")
			}
		})
	}
}

// TestApplyHopParamsLeavesSharedParamsIntact guards the shallow-copy
// hazard: Bifrost's fallback request shares Params with the primary, so the
// hook must replace Params instead of writing through the shared pointer.
func TestApplyHopParamsLeavesSharedParamsIntact(t *testing.T) {
	b := &LLM{
		provider:         schemas.Anthropic,
		reasoningEffort:  "xhigh",
		reasoningEnabled: true,
		baseProviders: map[schemas.ModelProvider]schemas.ModelProvider{
			schemas.Anthropic: schemas.Anthropic,
			customProviderName(schemas.Anthropic, "svc2"): schemas.Anthropic,
		},
	}
	cfg := llm.LanguageModelConfig{Model: "claude-opus-4-7", MaxGeneratedTokens: 200000}
	primary := &schemas.BifrostRequest{ChatRequest: b.convertToBifrostRequest(llm.CompletionRequest{
		Posts: []llm.Post{{Role: llm.PostRoleUser, Message: "hi"}},
	}, cfg)}
	sharedParams := primary.ChatRequest.Params

	hop := *primary.ChatRequest
	hop.Provider = customProviderName(schemas.Anthropic, "svc2")
	hop.Model = "claude-opus-4-6"
	b.applyHopParams(&schemas.BifrostRequest{ChatRequest: &hop}, cfg)

	require.NotNil(t, hop.Params.Reasoning)
	assert.Equal(t, "high", *hop.Params.Reasoning.Effort, "the Opus 4.6 hop must get an effort it accepts")
	require.NotNil(t, hop.Params.MaxCompletionTokens)
	assert.Equal(t, 128000, *hop.Params.MaxCompletionTokens, "the Opus 4.6 hop must get its own output limit")
	assert.Same(t, sharedParams, primary.ChatRequest.Params)
	assert.Equal(t, "xhigh", *sharedParams.Reasoning.Effort, "the primary's shared params must be untouched")
	assert.Equal(t, 200000, *sharedParams.MaxCompletionTokens, "the primary's shared params must be untouched")
}
