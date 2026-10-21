// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bifrost

import (
	providerutils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

// hopParamsKey carries a request's hopParams resolver on its Bifrost
// context. A Bifrost client is shared by every agent on a service, so the
// agent's settings travel with the request, not the plugin.
type hopParamsKey struct{}

// hopParams rewrites one attempt's model-specific parameters for that
// attempt's provider and model.
type hopParams func(req *schemas.BifrostRequest)

// hopParamsPlugin recomputes the max output tokens and reasoning block on
// every Bifrost attempt. Bifrost retries fallbacks with the primary request's
// parameters and only swaps the provider and model, but both are
// model-specific: a primary's output limit can exceed what a fallback model
// accepts, Opus 5.5 rejects the thinking:{type:"disabled"} that Sonnet 5
// needs, Opus 4.6 rejects xhigh, and an effort meant to minimize an always-on
// model's thinking turns thinking on for a budget-based or Gemini fallback.
type hopParamsPlugin struct{}

func (hopParamsPlugin) GetName() string { return "mattermost-ai-hop-params" }

func (hopParamsPlugin) Cleanup() error { return nil }

func (hopParamsPlugin) PreRequestHook(*schemas.BifrostContext, *schemas.BifrostRequest) error {
	return nil
}

// PreLLMHook runs once per attempt, with the fallback's provider and model
// already swapped in.
func (hopParamsPlugin) PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	if resolve, ok := ctx.Value(hopParamsKey{}).(hopParams); ok && req != nil {
		resolve(req)
	}
	return req, nil, nil
}

func (hopParamsPlugin) PostLLMHook(_ *schemas.BifrostContext, resp *schemas.BifrostResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	return resp, bifrostErr, nil
}

// withHopParams attaches the per-attempt resolver for cfg as requested,
// before any clamp to a model's output limit.
func (b *LLM) withHopParams(ctx *schemas.BifrostContext, cfg llm.LanguageModelConfig) {
	ctx.SetValue(hopParamsKey{}, hopParams(func(req *schemas.BifrostRequest) {
		b.applyHopParams(req, cfg)
	}))
}

// applyHopParams replaces the attempt's max output tokens and reasoning block
// with the ones for its provider and model. Bifrost's fallback copy is
// shallow, so Params is shared across attempts and must be replaced rather
// than modified.
func (b *LLM) applyHopParams(req *schemas.BifrostRequest, cfg llm.LanguageModelConfig) {
	switch {
	case req.ChatRequest != nil && req.ChatRequest.Params != nil:
		cfg = configForModel(cfg, req.ChatRequest.Model)
		params := *req.ChatRequest.Params
		params.MaxCompletionTokens = maxTokensParam(cfg)
		params.Reasoning = b.chatReasoningFor(b.baseProvider(req.ChatRequest.Provider), cfg)
		req.ChatRequest.Params = &params
	case req.ResponsesRequest != nil && req.ResponsesRequest.Params != nil:
		cfg = configForModel(cfg, req.ResponsesRequest.Model)
		params := *req.ResponsesRequest.Params
		params.MaxOutputTokens = maxTokensParam(cfg)
		params.Reasoning = b.responsesReasoningFor(b.baseProvider(req.ResponsesRequest.Provider), cfg)
		req.ResponsesRequest.Params = &params
	}
}

// configForModel returns cfg for model with the requested MaxGeneratedTokens
// clamped to that model's output limit. Clamping from the requested value
// keeps a fallback with a higher limit than the primary from being capped by
// the primary's.
func configForModel(cfg llm.LanguageModelConfig, model string) llm.LanguageModelConfig {
	cfg.Model = model
	cfg.MaxGeneratedTokens = clampToModelOutputLimit(model, cfg.MaxGeneratedTokens)
	return cfg
}

// clampToModelOutputLimit lowers requested to model's max output tokens.
// Bifrost sends max tokens as given and the provider rejects a value above the
// model's limit. Models without a known limit keep requested.
func clampToModelOutputLimit(model string, requested int) int {
	return min(requested, providerutils.GetMaxOutputTokensOrDefault(model, requested))
}

// maxTokensParam returns the max output tokens parameter for cfg, or nil
// when none should be sent.
func maxTokensParam(cfg llm.LanguageModelConfig) *int {
	if cfg.MaxGeneratedTokens <= 0 {
		return nil
	}
	return new(cfg.MaxGeneratedTokens)
}

// baseProvider returns the provider type behind a Bifrost registration name.
func (b *LLM) baseProvider(name schemas.ModelProvider) schemas.ModelProvider {
	if base, ok := b.baseProviders[name]; ok {
		return base
	}
	return name
}
