// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bifrost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm/llmtest"
)

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

// anthropicToolVersion matches the date suffix of an Anthropic server tool
// type, such as web_search_20260209.
var anthropicToolVersion = regexp.MustCompile(`_\d{8}$`)

// nativeToolsOnWire returns the provider-executed tools of a captured request:
// the tool types of an OpenAI or Anthropic request, without Anthropic's
// version suffix, and the tool keys of a Gemini request. Function tools are
// left out.
func nativeToolsOnWire(body map[string]any) []string {
	tools, _ := body["tools"].([]any)
	var native []string
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if toolType, ok := tool["type"].(string); ok {
			if toolType != "function" && toolType != "custom" {
				native = append(native, anthropicToolVersion.ReplaceAllString(toolType, ""))
			}
			continue
		}
		if _, isAnthropicFunction := tool["input_schema"]; isAnthropicFunction {
			continue
		}
		for key := range tool {
			if key != "functionDeclarations" {
				native = append(native, key)
			}
		}
	}
	return native
}

// TestMessageShapingCoversEveryFallbackHop drives real failovers through
// Bifrost and checks the message body every tried hop receives. Bifrost
// replays one built body to each fallback, so the body must satisfy the
// strictest provider that the request may reach, not just the primary.
func TestMessageShapingCoversEveryFallbackHop(t *testing.T) {
	failingHop := func(serviceType, model string) captureHop {
		return captureHop{svc: llm.ServiceConfig{Type: serviceType, DefaultModel: model}}
	}
	openAI := failingHop(llm.ServiceTypeOpenAI, "gpt-4o")
	openAICompatible := failingHop(llm.ServiceTypeOpenAICompatible, "gpt-4o")
	anthropicHop := failingHop(llm.ServiceTypeAnthropic, "claude-sonnet-4-6")
	gemini := failingHop(llm.ServiceTypeGemini, "gemini-2.5-flash")
	mistral := failingHop(llm.ServiceTypeMistral, "mistral-large-latest")
	azureClaude := failingHop(llm.ServiceTypeAzure, "claude-sonnet-4-6")
	local := localHop
	webSearch := []string{llm.NativeToolWebSearch}
	allNativeTools := []string{llm.NativeToolWebSearch, llm.NativeToolWebFetch, llm.NativeToolCodeInterpreter}

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
		name        string
		hops        []captureHop
		nativeTools []string
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
		// wantNativeTools is, per reached hop, the native tools its body
		// carries; nil skips the check.
		wantNativeTools [][]string
	}{
		{
			name:        "OpenAI Responses primary with an Anthropic fallback (#1042)",
			hops:        []captureHop{openAI, anthropicHop, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true,
		},
		{
			name:        "OpenAI-compatible chat primary with an Anthropic fallback",
			hops:        []captureHop{openAICompatible, anthropicHop, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "Gemini chat primary with an Anthropic fallback",
			hops:        []captureHop{gemini, anthropicHop, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "Gemini Responses primary with an Anthropic fallback",
			hops:        []captureHop{gemini, anthropicHop, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, nativeTools: webSearch,
		},
		{
			name:        "Anthropic chat primary with an OpenAI fallback",
			hops:        []captureHop{anthropicHop, openAI, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "Anthropic Responses primary with an OpenAI fallback",
			hops:        []captureHop{anthropicHop, openAI, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, nativeTools: webSearch,
		},
		{
			name:        "OpenAI-compatible chat primary with a Claude-on-Azure fallback",
			hops:        []captureHop{openAICompatible, azureClaude, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "OpenAI Responses primary with a Claude-on-Azure fallback",
			hops:        []captureHop{openAI, azureClaude, local},
			wantReached: 3, wantImageOmitted: true, wantStrict: true,
		},
		{
			// Anthropic adds code_execution itself when web search or fetch
			// is present, so Bifrost leaves it out of the request.
			name:        "Anthropic primary's native tools reach each fallback only as that hop supports them",
			hops:        []captureHop{anthropicHop, openAI, gemini, openAICompatible, mistral, local},
			nativeTools: allNativeTools,
			wantReached: 6, wantImageOmitted: true, wantStrict: true,
			wantNativeTools: [][]string{
				{"web_search", "web_fetch"},
				{"web_search", "code_interpreter"},
				{"googleSearch"},
				{}, {}, {},
			},
		},
		{
			name:        "OpenAI primary's native tools reach each fallback only as that hop supports them",
			hops:        []captureHop{openAI, anthropicHop, gemini, azureClaude, local},
			nativeTools: allNativeTools,
			wantReached: 5, wantImageOmitted: true, wantStrict: true,
			wantNativeTools: [][]string{
				{"web_search", "code_interpreter"},
				{"web_search"},
				{"googleSearch"},
				{"web_search"},
				{},
			},
		},
		{
			name:        "Anthropic hop behind the serving hop still shapes the body",
			hops:        []captureHop{openAICompatible, local, anthropicHop},
			wantReached: 2, wantImageOmitted: true, wantStrict: true, chatPath: true,
		},
		{
			name:        "Anthropic hop trimmed by access does not shape the chat body",
			hops:        []captureHop{openAICompatible, local, anthropicHop},
			allowed:     []int{1},
			wantReached: 2, chatPath: true,
		},
		{
			name:        "Anthropic hop trimmed by access does not shape the Responses body",
			hops:        []captureHop{openAI, local, anthropicHop},
			allowed:     []int{1},
			wantReached: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot := llm.BotConfig{EnabledNativeTools: tt.nativeTools}
			request := llm.CompletionRequest{Posts: posts}
			if tt.allowed != nil {
				request.RestrictFallbacks = true
				for _, i := range tt.allowed {
					request.AllowedFallbackServiceIDs = append(request.AllowedFallbackServiceIDs, hopServiceID(i))
				}
			}

			llmInstance, bodies := captureFailover(t, tt.hops, bot, request)
			require.Equal(t, !tt.chatPath, llmInstance.shouldUseResponsesAPI(llmInstance.GetDefaultConfig()), "row must exercise the intended API path")

			for i, h := range tt.hops {
				label := fmt.Sprintf("hop %d (%s)", i, h.svc.Type)
				parsed := bodies[i]
				reached := parsed != nil
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
				if tt.wantStrict && tt.chatPath {
					if _, isChat := parsed["messages"]; isChat {
						assert.Empty(t, adjacentSameRoleMessages(parsed), "%s must not repeat a role: %s", label, body)
					}
				}
				if tt.wantNativeTools != nil {
					assert.ElementsMatch(t, tt.wantNativeTools[i], nativeToolsOnWire(parsed), "%s native tools", label)
				}
				if h.svc.Type == llm.ServiceTypeAnthropic || strings.HasPrefix(h.svc.DefaultModel, "claude") {
					assert.Empty(t, llmtest.AnthropicRequestViolations(raw), "%s would be rejected by Anthropic: %s", label, body)
				}
			}
		})
	}
}
