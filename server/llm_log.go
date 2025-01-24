// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
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

func (w *LanguageModelLogWrapper) logInput(conversation llm.BotConversation, opts ...llm.LanguageModelOption) {
	prompt := fmt.Sprintf("\n%v", conversation)
	w.log.Info("LLM Call", "prompt", prompt)
}

func (w *LanguageModelLogWrapper) ChatCompletion(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	w.logInput(conversation, opts...)
	return w.wrapped.ChatCompletion(conversation, opts...)
}

func (w *LanguageModelLogWrapper) ChatCompletionNoStream(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (string, error) {
	w.logInput(conversation, opts...)
	return w.wrapped.ChatCompletionNoStream(conversation, opts...)
}

func (w *LanguageModelLogWrapper) CountTokens(text string) int {
	return w.wrapped.CountTokens(text)
}

func (w *LanguageModelLogWrapper) InputTokenLimit() int {
	return w.wrapped.InputTokenLimit()
}
