// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package providers

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type LanguageModelLogWrapper struct {
	log     pluginapi.LogService
	wrapped llm.LanguageModel
}

func NewLanguageModelLogWrapper(log pluginapi.LogService, wrapped llm.LanguageModel) *LanguageModelLogWrapper {
	return &LanguageModelLogWrapper{
		log:     log,
		wrapped: wrapped,
	}
}

func (w *LanguageModelLogWrapper) logInput(request llm.CompletionRequest, opts ...llm.LanguageModelOption) {
	prompt := fmt.Sprintf("\n%v", request)
	w.log.Info("LLM Call", "prompt", prompt)
}

func (w *LanguageModelLogWrapper) ChatCompletion(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	w.logInput(request, opts...)
	return w.wrapped.ChatCompletion(request, opts...)
}

func (w *LanguageModelLogWrapper) ChatCompletionNoStream(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (string, error) {
	w.logInput(request, opts...)
	return w.wrapped.ChatCompletionNoStream(request, opts...)
}

func (w *LanguageModelLogWrapper) CountTokens(text string) int {
	return w.wrapped.CountTokens(text)
}

func (w *LanguageModelLogWrapper) InputTokenLimit() int {
	return w.wrapped.InputTokenLimit()
}
