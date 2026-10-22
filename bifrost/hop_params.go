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
// before any clamp to a service's or model's output limit.
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
		req.ChatRequest.Params = b.chatHopParams(*req.ChatRequest.Params, req.ChatRequest.Provider, req.ChatRequest.Model, cfg)
	case req.ResponsesRequest != nil && req.ResponsesRequest.Params != nil:
		req.ResponsesRequest.Params = b.responsesHopParams(*req.ResponsesRequest.Params, req.ResponsesRequest.Provider, req.ResponsesRequest.Model, cfg)
	}
}

// chatHopParams returns params with the max output tokens and reasoning block
// for model on the provider registered as name.
func (b *LLM) chatHopParams(params schemas.ChatParameters, name schemas.ModelProvider, model string, cfg llm.LanguageModelConfig) *schemas.ChatParameters {
	cfg = b.configForHop(cfg, name, model)
	params.MaxCompletionTokens = maxTokensParam(cfg)
	params.Reasoning = b.chatReasoningFor(name, cfg)
	return &params
}

// responsesHopParams is chatHopParams for the Responses API.
func (b *LLM) responsesHopParams(params schemas.ResponsesParameters, name schemas.ModelProvider, model string, cfg llm.LanguageModelConfig) *schemas.ResponsesParameters {
	cfg = b.configForHop(cfg, name, model)
	params.MaxOutputTokens = maxTokensParam(cfg)
	params.Reasoning = b.responsesReasoningFor(name, cfg)
	return &params
}

// configForHop returns cfg for model on the provider registered as name, with
// the requested MaxGeneratedTokens clamped to the model's output limit and,
// for a fallback, to its service's configured limit. Clamping from the requested value keeps a
// fallback with a higher limit than the primary from being capped by the
// primary's.
func (b *LLM) configForHop(cfg llm.LanguageModelConfig, name schemas.ModelProvider, model string) llm.LanguageModelConfig {
	cfg.Model = model
	if entry := b.registration(name); entry != nil && entry.outputTokenLimit > 0 {
		cfg.MaxGeneratedTokens = min(cfg.MaxGeneratedTokens, entry.outputTokenLimit)
	}
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
	if entry := b.registration(name); entry != nil {
		return entry.Provider
	}
	return name
}

// registration returns the service registered under name, or nil when no
// service is.
func (b *LLM) registration(name schemas.ModelProvider) *providerAccount {
	if b.account == nil {
		return nil
	}
	return b.account.entries[name]
}
