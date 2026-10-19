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
			var primaryBody, fallbackBody atomic.Value
			var localHits atomic.Int32
			primaryServer := failingCaptureServer(t, &primaryBody)
			fallbackServer := failingCaptureServer(t, &fallbackBody)
			localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				localHits.Add(1)
				chatCompletionSSE(w, "from-local")
			}))
			t.Cleanup(localServer.Close)

			primarySvc := llm.ServiceConfig{
				ID: "primary", Type: llm.ServiceTypeAnthropic, APIKey: "primary-key", APIURL: primaryServer.URL,
				DefaultModel: tt.primaryModel, FallbackServiceID: "fallback",
			}
			fallbackSvc := llm.ServiceConfig{
				ID: "fallback", Type: tt.fallbackType, APIKey: "fallback-key", APIURL: fallbackServer.URL,
				DefaultModel: tt.fallbackModel, FallbackServiceID: "local",
			}
			localSvc := llm.ServiceConfig{
				ID: "local", Type: llm.ServiceTypeOpenAICompatible, APIURL: localServer.URL, DefaultModel: "llama3",
			}
			bot := llm.BotConfig{
				ID: "bot-1", Name: "ai", DisplayName: "AI", ServiceID: "primary",
				ReasoningEnabled: tt.reasoningEnabled, ReasoningEffort: tt.effort,
			}
			if tt.nativeWebSearch {
				bot.EnabledNativeTools = []string{"web_search"}
			}

			llmInstance, err := NewFromServiceConfig(primarySvc, bot, []llm.ServiceConfig{fallbackSvc, localSvc})
			require.NoError(t, err)
			t.Cleanup(llmInstance.Shutdown)

			_, err = llmInstance.ChatCompletionNoStream(context.Background(), llm.CompletionRequest{
				Posts: []llm.Post{{Role: llm.PostRoleUser, Message: "hi"}},
			}, llm.WithToolsDisabled())
			require.NoError(t, err)
			require.Positive(t, localHits.Load(), "the request must fail over through both hops")

			assertHop := func(label string, captured any, want wantAnthropic) {
				body, ok := captured.(map[string]any)
				require.True(t, ok, "%s hop must have been called", label)
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

			assertHop("primary", primaryBody.Load(), tt.wantPrimary)
			if tt.fallbackType == llm.ServiceTypeGemini {
				body, ok := fallbackBody.Load().(map[string]any)
				require.True(t, ok, "Gemini hop must have been called")
				generationConfig, _ := body["generationConfig"].(map[string]any)
				thinkingConfig, _ := generationConfig["thinkingConfig"].(map[string]any)
				includeThoughts, _ := thinkingConfig["includeThoughts"].(bool)
				assert.Equal(t, tt.wantGeminiThink, includeThoughts, "Gemini includeThoughts")
				return
			}
			assertHop("fallback", fallbackBody.Load(), tt.wantFallback)
		})
	}
}

// TestApplyHopReasoningLeavesSharedParamsIntact guards the shallow-copy
// hazard: Bifrost's fallback request shares Params with the primary, so the
// hook must replace Params instead of writing through the shared pointer.
func TestApplyHopReasoningLeavesSharedParamsIntact(t *testing.T) {
	b := &LLM{
		provider:         schemas.Anthropic,
		reasoningEffort:  "xhigh",
		reasoningEnabled: true,
		baseProviders: map[schemas.ModelProvider]schemas.ModelProvider{
			schemas.Anthropic: schemas.Anthropic,
			customProviderName(schemas.Anthropic, "svc2"): schemas.Anthropic,
		},
	}
	cfg := llm.LanguageModelConfig{Model: "claude-opus-4-7", MaxGeneratedTokens: 8192}
	primary := &schemas.BifrostRequest{ChatRequest: b.convertToBifrostRequest(llm.CompletionRequest{
		Posts: []llm.Post{{Role: llm.PostRoleUser, Message: "hi"}},
	}, cfg)}
	sharedParams := primary.ChatRequest.Params

	hop := *primary.ChatRequest
	hop.Provider = customProviderName(schemas.Anthropic, "svc2")
	hop.Model = "claude-opus-4-6"
	b.applyHopReasoning(&schemas.BifrostRequest{ChatRequest: &hop}, cfg)

	require.NotNil(t, hop.Params.Reasoning)
	assert.Equal(t, "high", *hop.Params.Reasoning.Effort, "the Opus 4.6 hop must get an effort it accepts")
	assert.Same(t, sharedParams, primary.ChatRequest.Params)
	assert.Equal(t, "xhigh", *sharedParams.Reasoning.Effort, "the primary's shared params must be untouched")
}
