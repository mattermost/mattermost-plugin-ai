package main

import (
	"math"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
)

const FunctionsTokenBudget = 200
const TokenLimitBufferSize = 0.9
const MinTokens = 100

type LLMTruncationWrapper struct {
	wrapped ai.LanguageModel
}

func NewLLMTruncationWrapper(llm ai.LanguageModel) *LLMTruncationWrapper {
	return &LLMTruncationWrapper{
		wrapped: llm,
	}
}

func (w *LLMTruncationWrapper) ChatCompletion(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (*ai.TextStreamResult, error) {
	tokenLimit := int(math.Max(math.Floor(float64(w.wrapped.InputTokenLimit()-FunctionsTokenBudget)*TokenLimitBufferSize), MinTokens))
	conversation.Truncate(tokenLimit, w.wrapped.CountTokens)
	return w.wrapped.ChatCompletion(conversation, opts...)
}

func (w *LLMTruncationWrapper) ChatCompletionNoStream(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (string, error) {
	tokenLimit := int(math.Max(math.Floor(float64(w.wrapped.InputTokenLimit()-FunctionsTokenBudget)*TokenLimitBufferSize), MinTokens))
	conversation.Truncate(tokenLimit, w.wrapped.CountTokens)
	return w.wrapped.ChatCompletionNoStream(conversation, opts...)
}

func (w *LLMTruncationWrapper) CountTokens(text string) int {
	return w.wrapped.CountTokens(text)
}

func (w *LLMTruncationWrapper) InputTokenLimit() int {
	return w.wrapped.InputTokenLimit()
}
