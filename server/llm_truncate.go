package main

import (
	"math"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
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

func (w *LLMTruncationWrapper) ChatCompletion(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	tokenLimit := int(math.Max(math.Floor(float64(w.wrapped.TokenLimit()-FunctionsTokenBudget)*TokenLimitBufferSize), MinTokens))
	conversation.Truncate(tokenLimit, w.wrapped.CountTokens)
	return w.wrapped.ChatCompletion(conversation, opts...)
}

func (w *LLMTruncationWrapper) ChatCompletionNoStream(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (string, error) {
	tokenLimit := int(math.Max(math.Floor(float64(w.wrapped.TokenLimit()-FunctionsTokenBudget)*TokenLimitBufferSize), MinTokens))
	conversation.Truncate(tokenLimit, w.wrapped.CountTokens)
	return w.wrapped.ChatCompletionNoStream(conversation, opts...)
}

func (w *LLMTruncationWrapper) CountTokens(text string) int {
	return w.wrapped.CountTokens(text)
}

func (w *LLMTruncationWrapper) TokenLimit() int {
	return w.wrapped.TokenLimit()
}
