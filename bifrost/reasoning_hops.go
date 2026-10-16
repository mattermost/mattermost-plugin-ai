// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bifrost

import (
	"github.com/maximhq/bifrost/core/schemas"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

// hopReasoningKey carries a request's hopReasoning resolver on its Bifrost
// context. A Bifrost client is shared by every agent on a service, so the
// agent's reasoning settings travel with the request, not the plugin.
type hopReasoningKey struct{}

// hopReasoning rewrites one attempt's reasoning block for that attempt's
// provider and model.
type hopReasoning func(req *schemas.BifrostRequest)

// reasoningHopPlugin recomputes the reasoning block on every Bifrost attempt.
// Bifrost retries fallbacks with the primary request's parameters and only
// swaps the provider and model, but reasoning settings are model-specific:
// Opus 5.5 rejects the thinking:{type:"disabled"} that Sonnet 5 needs, Opus
// 4.6 rejects xhigh, and an effort meant to minimize an always-on model's
// thinking turns thinking on for a budget-based or Gemini fallback.
type reasoningHopPlugin struct{}

func (reasoningHopPlugin) GetName() string { return "mattermost-ai-hop-reasoning" }

func (reasoningHopPlugin) Cleanup() error { return nil }

func (reasoningHopPlugin) PreRequestHook(*schemas.BifrostContext, *schemas.BifrostRequest) error {
	return nil
}

// PreLLMHook runs once per attempt, with the fallback's provider and model
// already swapped in.
func (reasoningHopPlugin) PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	if resolve, ok := ctx.Value(hopReasoningKey{}).(hopReasoning); ok && req != nil {
		resolve(req)
	}
	return req, nil, nil
}

func (reasoningHopPlugin) PostLLMHook(_ *schemas.BifrostContext, resp *schemas.BifrostResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	return resp, bifrostErr, nil
}

// withHopReasoning attaches the per-attempt reasoning resolver for cfg.
func (b *LLM) withHopReasoning(ctx *schemas.BifrostContext, cfg llm.LanguageModelConfig) {
	ctx.SetValue(hopReasoningKey{}, hopReasoning(func(req *schemas.BifrostRequest) {
		b.applyHopReasoning(req, cfg)
	}))
}

// applyHopReasoning replaces the attempt's reasoning block with the one for
// its provider and model. Bifrost's fallback copy is shallow, so Params is
// shared across attempts and must be replaced rather than modified.
func (b *LLM) applyHopReasoning(req *schemas.BifrostRequest, cfg llm.LanguageModelConfig) {
	switch {
	case req.ChatRequest != nil && req.ChatRequest.Params != nil:
		cfg.Model = req.ChatRequest.Model
		params := *req.ChatRequest.Params
		params.Reasoning = b.chatReasoningFor(b.baseProvider(req.ChatRequest.Provider), cfg)
		req.ChatRequest.Params = &params
	case req.ResponsesRequest != nil && req.ResponsesRequest.Params != nil:
		cfg.Model = req.ResponsesRequest.Model
		params := *req.ResponsesRequest.Params
		params.Reasoning = b.responsesReasoningFor(b.baseProvider(req.ResponsesRequest.Provider), cfg)
		req.ResponsesRequest.Params = &params
	}
}

// baseProvider returns the provider type behind a Bifrost registration name.
func (b *LLM) baseProvider(name schemas.ModelProvider) schemas.ModelProvider {
	if base, ok := b.baseProviders[name]; ok {
		return base
	}
	return name
}
