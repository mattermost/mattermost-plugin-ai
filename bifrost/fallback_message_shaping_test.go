// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bifrost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm/llmtest"
)

// succeedingCaptureServer records the last request body it receives and
// answers it, ending the fallback chain.
func succeedingCaptureServer(t *testing.T, body *atomic.Value) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(data, &parsed)
		body.Store(parsed)
		if strings.HasSuffix(r.URL.Path, "/responses") {
			responsesSSE(w, "ok")
			return
		}
		chatCompletionSSE(w, "ok")
	}))
	t.Cleanup(server.Close)
	return server
}

// adjacentSameRoleMessages returns the index of each message in a chat-style
// body ("messages" array) that repeats the previous message's role. Tool
// results are exempt: OpenAI sends one tool message per result, and Anthropic
// carries them in a user message of tool_result blocks.
func adjacentSameRoleMessages(body map[string]any) []int {
	messages, _ := body["messages"].([]any)
	var repeats []int
	prevRole := ""
	for i, raw := range messages {
		msg, _ := raw.(map[string]any)
		role, _ := msg["role"].(string)
		if blocks, ok := msg["content"].([]any); ok && len(blocks) > 0 {
			onlyToolResults := true
			for _, rawBlock := range blocks {
				block, _ := rawBlock.(map[string]any)
				onlyToolResults = onlyToolResults && block["type"] == "tool_result"
			}
			if onlyToolResults {
				role = "tool"
			}
		}
		if role != "tool" && role == prevRole {
			repeats = append(repeats, i)
		}
		prevRole = role
	}
	return repeats
}

// TestMessageShapingCoversEveryFallbackHop drives real failovers through
// Bifrost and checks the message body every tried hop receives. Bifrost
// replays one built body to each fallback, so the body must satisfy the
// strictest provider that the request may reach, not just the primary.
func TestMessageShapingCoversEveryFallbackHop(t *testing.T) {
	type hop struct {
		serviceType string
		model       string
		succeed     bool
	}
	openAI := hop{serviceType: llm.ServiceTypeOpenAI, model: "gpt-4o"}
	openAICompatible := hop{serviceType: llm.ServiceTypeOpenAICompatible, model: "gpt-4o"}
	anthropicHop := hop{serviceType: llm.ServiceTypeAnthropic, model: "claude-sonnet-4-6"}
	gemini := hop{serviceType: llm.ServiceTypeGemini, model: "gemini-2.5-flash"}
	local := hop{serviceType: llm.ServiceTypeOpenAICompatible, model: "llama3", succeed: true}

	oversized := encodeTestImage(t, "image/png", anthropicMaxImageDimension+1000, 1)
	oversizedBase64 := base64.StdEncoding.EncodeToString(oversized)
	mislabeled := encodeTestImage(t, "image/png", 4, 4)
	const unsignedReasoning = "partial thinking captured before the stream failed"

	posts := []llm.Post{
		{Role: llm.PostRoleSystem, Message: "You are helpful."},
		{Role: llm.PostRoleSystem, Message: " \n"},
		{Role: llm.PostRoleUser, Message: "look at these", Files: []llm.File{
			{MimeType: "image/png", Data: oversized},
			{MimeType: "image/jpeg", Data: mislabeled},
		}},
		{Role: llm.PostRoleBot, Message: "Let me check.", Reasoning: unsignedReasoning},
		{Role: llm.PostRoleBot, Message: "  ", ToolUse: []llm.ToolCall{
			{ID: "call_empty", Name: "search", Arguments: json.RawMessage(`{"q":"x"}`), Result: " \n"},
			{ID: "call_missing", Name: "lookup", Arguments: json.RawMessage(`{}`)},
			{ID: "orphan", Result: "stray result for a tool_use that lives elsewhere"},
		}},
		{Role: llm.PostRoleBot, Message: "Here is what I found.", AssistantSegments: []llm.TurnSegment{
			{Kind: llm.TurnSegmentText, Text: " "},
			{Kind: llm.TurnSegmentText, Text: "Here is what I found."},
		}},
		{Role: llm.PostRoleBot},
		{Role: llm.PostRoleUser, Message: " \t"},
		{Role: llm.PostRoleUser, Message: "thanks"},
		{Role: llm.PostRoleUser, Message: "one more question"},
	}

	tests := []struct {
		name            string
		hops            []hop
		nativeWebSearch bool
		// allowed restricts the fallbacks to these hop indexes; nil leaves the
		// chain unrestricted.
		allowed []int
		// wantReached is how many hops, in order, receive the request.
		wantReached      int
		wantImageOmitted bool
		// wantStrict is whether the body follows Anthropic's message rules:
		// no unsigned reasoning, and (on the chat path) no repeated roles.
		wantStrict bool
		chatPath   bool
	}{
		{
			name:        "OpenAI Responses primary with an Anthropic fallback (#1042)",
			hops:        []hop{openAI, anthropicHop, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true,
		},
		{
			name:        "OpenAI-compatible chat primary with an Anthropic fallback",
			hops:        []hop{openAICompatible, anthropicHop, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "Gemini chat primary with an Anthropic fallback",
			hops:        []hop{gemini, anthropicHop, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "Gemini Responses primary with an Anthropic fallback",
			hops:        []hop{gemini, anthropicHop, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, nativeWebSearch: true,
		},
		{
			name:        "Anthropic chat primary with an OpenAI fallback",
			hops:        []hop{anthropicHop, openAI, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "Anthropic Responses primary with an OpenAI fallback",
			hops:        []hop{anthropicHop, openAI, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, nativeWebSearch: true,
		},
		{
			name:        "Anthropic hop behind the serving hop still shapes the body",
			hops:        []hop{openAICompatible, local, anthropicHop},
			wantReached: 2, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "Anthropic hop trimmed by access does not shape the chat body",
			hops:        []hop{openAICompatible, local, anthropicHop},
			allowed:     []int{1},
			wantReached: 2, chatPath: true,
		},
		{
			name:        "Anthropic hop trimmed by access does not shape the Responses body",
			hops:        []hop{openAI, local, anthropicHop},
			allowed:     []int{1},
			wantReached: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodies := make([]atomic.Value, len(tt.hops))
			services := make([]llm.ServiceConfig, len(tt.hops))
			for i, h := range tt.hops {
				captureServer := failingCaptureServer
				if h.succeed {
					captureServer = succeedingCaptureServer
				}
				services[i] = llm.ServiceConfig{
					ID: fmt.Sprintf("hop%d", i), Type: h.serviceType, APIKey: fmt.Sprintf("key-%d", i),
					APIURL: captureServer(t, &bodies[i]).URL, DefaultModel: h.model,
				}
			}
			bot := llm.BotConfig{ID: "bot-1", Name: "ai", DisplayName: "AI", ServiceID: services[0].ID}
			if tt.nativeWebSearch {
				bot.EnabledNativeTools = []string{llm.NativeToolWebSearch}
			}
			request := llm.CompletionRequest{Posts: posts}
			if tt.allowed != nil {
				request.RestrictFallbacks = true
				for _, i := range tt.allowed {
					request.AllowedFallbackServiceIDs = append(request.AllowedFallbackServiceIDs, services[i].ID)
				}
			}

			llmInstance, err := NewFromServiceConfig(services[0], bot, services[1:])
			require.NoError(t, err)
			t.Cleanup(llmInstance.Shutdown)
			require.Equal(t, !tt.chatPath, llmInstance.shouldUseResponsesAPI(llmInstance.GetDefaultConfig()), "row must exercise the intended API path")

			_, err = llmInstance.ChatCompletionNoStream(context.Background(), request, llm.WithToolsDisabled())
			require.NoError(t, err)

			for i, h := range tt.hops {
				label := fmt.Sprintf("hop %d (%s)", i, h.serviceType)
				parsed, reached := bodies[i].Load().(map[string]any)
				if i >= tt.wantReached {
					assert.False(t, reached, "%s must not be tried", label)
					continue
				}
				require.True(t, reached, "%s must be tried", label)
				raw, err := json.Marshal(parsed)
				require.NoError(t, err)
				body := string(raw)

				assert.Equal(t, tt.wantImageOmitted, strings.Contains(body, "Image omitted"), "%s oversized image placeholder", label)
				assert.Equal(t, !tt.wantImageOmitted, strings.Contains(body, oversizedBase64), "%s oversized image bytes", label)
				assert.NotContains(t, body, "image/jpeg", "%s must label the PNG with its sniffed type (#1100)", label)
				assert.Contains(t, body, base64.StdEncoding.EncodeToString(mislabeled), "%s must still send the mislabeled image", label)
				assert.Contains(t, body, emptyToolResultPlaceholder, "%s blank tool results get the placeholder (#995)", label)
				assert.NotContains(t, body, "stray result", "%s nameless orphan results are dropped", label)
				if tt.wantStrict {
					assert.NotContains(t, body, unsignedReasoning, "%s must not replay unsigned reasoning", label)
				}
				if tt.chatPath {
					if _, isChat := parsed["messages"]; isChat {
						assert.Equal(t, tt.wantStrict, len(adjacentSameRoleMessages(parsed)) == 0,
							"%s consecutive same-role messages merged only for a strict chain: %s", label, body)
					}
				}
				if h.serviceType == llm.ServiceTypeAnthropic {
					assert.Empty(t, llmtest.AnthropicRequestViolations(raw), "%s would be rejected by Anthropic: %s", label, body)
				}
			}
		})
	}
}
