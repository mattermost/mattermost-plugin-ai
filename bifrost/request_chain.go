// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bifrost

import (
	"github.com/maximhq/bifrost/core/schemas"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

// requestChain is every service one request may be tried on: the primary,
// then the fallbacks the requester may use. Bifrost replays the primary's
// message body unchanged to each fallback, so message shaping must hold for
// every hop.
type requestChain struct {
	fallbacks []schemas.Fallback

	// imageDimensionLimit is the lowest per-side pixel limit of any hop, or
	// zero when no hop rejects images on dimensions.
	imageDimensionLimit int
	// strictMessages is set when a hop rejects replayed reasoning without a
	// provider signature and requires alternating roles (Anthropic, Bedrock,
	// and Claude on Vertex or Azure).
	strictMessages bool
	// promptCaching requests Anthropic automatic prompt caching (top-level
	// cache_control); Anthropic caches nothing unless asked, so every turn
	// would re-bill the whole prompt. It is set only when every hop is
	// Anthropic: Bifrost forwards the marker unstripped, and other providers
	// reject it.
	promptCaching bool
}

// requestChain resolves the chain request is tried on when primaryModel is
// the primary's model.
func (b *LLM) requestChain(request llm.CompletionRequest, primaryModel string) requestChain {
	chain := requestChain{
		fallbacks:     b.requestFallbacks(request),
		promptCaching: true,
	}
	hops := []schemas.Fallback{{Provider: b.baseProvider(b.provider), Model: primaryModel}}
	for _, fb := range chain.fallbacks {
		hops = append(hops, schemas.Fallback{Provider: b.baseProvider(fb.Provider), Model: fb.Model})
	}

	imageCount := countRequestImages(request.Posts)
	for _, hop := range hops {
		messagesAPI := messagesAPIProvider(hop.Provider, hop.Model)
		if limit := maxImageDimension(messagesAPI, imageCount); limit > 0 &&
			(chain.imageDimensionLimit == 0 || limit < chain.imageDimensionLimit) {
			chain.imageDimensionLimit = limit
		}
		switch messagesAPI {
		case schemas.Anthropic, schemas.Bedrock, schemas.BedrockMantle:
			chain.strictMessages = true
		}
		if hop.Provider != schemas.Anthropic {
			chain.promptCaching = false
		}
	}
	return chain
}

// messagesAPIProvider returns the provider whose message rules model follows
// on base: Bifrost sends Claude models on Vertex and Azure to Anthropic's
// Messages API.
func messagesAPIProvider(base schemas.ModelProvider, model string) schemas.ModelProvider {
	if (base == schemas.Vertex || base == schemas.Azure) && schemas.IsAnthropicModel(model) {
		return schemas.Anthropic
	}
	return base
}

// prepareRequest returns request with its posts normalized for its chain,
// and that chain. Both API paths build messages only from its results.
func (b *LLM) prepareRequest(request llm.CompletionRequest, primaryModel string) (llm.CompletionRequest, requestChain) {
	chain := b.requestChain(request, primaryModel)
	request.Posts = normalizePosts(request.Posts, chain)
	return request, chain
}
