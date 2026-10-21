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
	// provider signature and requires alternating roles (Anthropic, Bedrock).
	strictMessages bool
	// promptCaching requests Anthropic automatic prompt caching (top-level
	// cache_control); Anthropic caches nothing unless asked, so every turn
	// would re-bill the whole prompt. It is set only when every hop is
	// Anthropic: Bifrost forwards the marker unstripped, and other providers
	// reject it.
	promptCaching bool
}

// requestChain resolves the chain request is tried on.
func (b *LLM) requestChain(request llm.CompletionRequest) requestChain {
	chain := requestChain{
		fallbacks:     b.requestFallbacks(request),
		promptCaching: true,
	}
	hops := []schemas.ModelProvider{b.baseProvider(b.provider)}
	for _, fb := range chain.fallbacks {
		hops = append(hops, b.baseProvider(fb.Provider))
	}

	imageCount := countRequestImages(request.Posts)
	for _, provider := range hops {
		if limit := maxImageDimension(provider, imageCount); limit > 0 &&
			(chain.imageDimensionLimit == 0 || limit < chain.imageDimensionLimit) {
			chain.imageDimensionLimit = limit
		}
		switch provider {
		case schemas.Anthropic, schemas.Bedrock, schemas.BedrockMantle:
			chain.strictMessages = true
		}
		if provider != schemas.Anthropic {
			chain.promptCaching = false
		}
	}
	return chain
}

// prepareRequest returns request with its posts normalized for its chain,
// and that chain. Both API paths build messages only from its results.
func (b *LLM) prepareRequest(request llm.CompletionRequest) (llm.CompletionRequest, requestChain) {
	chain := b.requestChain(request)
	request.Posts = normalizePosts(request.Posts, chain)
	return request, chain
}
