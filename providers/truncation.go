// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package providers

import (
	"math"

	"github.com/mattermost/mattermost-plugin-ai/llm"
)

const FunctionsTokenBudget = 200
const TokenLimitBufferSize = 0.9
const MinTokens = 100

type LLMTruncationWrapper struct {
	wrapped llm.LanguageModel
}

func NewLLMTruncationWrapper(llm llm.LanguageModel) *LLMTruncationWrapper {
	return &LLMTruncationWrapper{
		wrapped: llm,
	}
}

func (w *LLMTruncationWrapper) ChatCompletion(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	tokenLimit := int(math.Max(math.Floor(float64(w.wrapped.InputTokenLimit()-FunctionsTokenBudget)*TokenLimitBufferSize), MinTokens))
	request.Truncate(tokenLimit, w.wrapped.CountTokens)
	return w.wrapped.ChatCompletion(request, opts...)
}

func (w *LLMTruncationWrapper) ChatCompletionNoStream(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (string, error) {
	tokenLimit := int(math.Max(math.Floor(float64(w.wrapped.InputTokenLimit()-FunctionsTokenBudget)*TokenLimitBufferSize), MinTokens))
	request.Truncate(tokenLimit, w.wrapped.CountTokens)
	return w.wrapped.ChatCompletionNoStream(request, opts...)
}

func (w *LLMTruncationWrapper) CountTokens(text string) int {
	return w.wrapped.CountTokens(text)
}

func (w *LLMTruncationWrapper) InputTokenLimit() int {
	return w.wrapped.InputTokenLimit()
}
