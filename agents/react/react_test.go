// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package react_test

import (
	"errors"
	"testing"

	"github.com/mattermost/mattermost-plugin-ai/agents/react"
	"github.com/mattermost/mattermost-plugin-ai/evals"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/llm/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReactResolve(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Setup
		mockLLM := mocks.NewMockLanguageModel(t)
		prompts, err := llm.NewPrompts(llm.PromptsFolder)
		assert.NoError(t, err)

		mockLLM.EXPECT().ChatCompletionNoStream(mock.Anything, mock.Anything).Return("thumbsup", nil)

		r := react.New(mockLLM, prompts)
		ctx := llm.NewContext()

		// Execute
		emoji, err := r.Resolve("Great job on the presentation!", ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "thumbsup", emoji)
	})

	t.Run("invalid emoji", func(t *testing.T) {
		// Setup
		mockLLM := mocks.NewMockLanguageModel(t)
		prompts, err := llm.NewPrompts(llm.PromptsFolder)
		assert.NoError(t, err)

		// Return an invalid emoji response that doesn't exist in standard emoji list
		mockLLM.EXPECT().ChatCompletionNoStream(mock.Anything, mock.Anything).Return("not_an_emoji", nil)

		r := react.New(mockLLM, prompts)
		ctx := llm.NewContext()

		// Execute
		_, err = r.Resolve("Great job on the presentation!", ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LLM returned something other than emoji")
	})

	t.Run("llm error", func(t *testing.T) {
		// Setup
		mockLLM := mocks.NewMockLanguageModel(t)
		prompts, err := llm.NewPrompts(llm.PromptsFolder)
		assert.NoError(t, err)

		mockLLM.EXPECT().ChatCompletionNoStream(mock.Anything, mock.Anything).Return("", errors.New("llm error"))

		r := react.New(mockLLM, prompts)
		ctx := llm.NewContext()

		// Execute
		_, err = r.Resolve("Great job on the presentation!", ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get emoji from LLM")
	})
}

func TestReactEval(t *testing.T) {
	evals.Run(t, "react", func(t *evals.EvalT) {
		// Create a new React instance
		r := react.New(t.LLM, t.Prompts)
		llmContext := llm.NewContext()
		result, err := r.Resolve("Great job on the presentation! How is it going with yours?", llmContext)
		require.NoError(t, err)

		// TODO: Add proper tests if the emoji makes sense
		assert.NotEmpty(t, result, "Expected a non-empty emoji reaction")
	})
}
