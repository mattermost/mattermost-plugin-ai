// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package evals

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/openai"
	"github.com/mattermost/mattermost-plugin-ai/prompts"
	"github.com/stretchr/testify/require"
)

type EvalT struct {
	*testing.T
	*Eval
}

type Eval struct {
	LLM       llm.LanguageModel
	GraderLLM llm.LanguageModel
	Prompts   *llm.Prompts

	runNumber int
}

func NewEval() (*Eval, error) {
	// Setup prompts
	prompts, err := llm.NewPrompts(prompts.PromptsFolder)
	if err != nil {
		return nil, err
	}

	// Setup real LLM
	httpClient := http.Client{}
	provider := openai.New(openai.Config{
		APIKey:           os.Getenv("OPENAI_API_KEY"),
		DefaultModel:     "gpt-4o",
		StreamingTimeout: 20 * time.Second,
	}, &httpClient)
	if provider == nil {
		return nil, errors.New("failed to create LLM provider")
	}

	return &Eval{
		Prompts:   prompts,
		LLM:       provider,
		GraderLLM: provider, // TODO: use a different LLM for grading
	}, nil
}

func NumEvalsOrSkip(t *testing.T) int {
	t.Helper()
	numEvals, err := strconv.Atoi(os.Getenv("GOEVALS"))
	if err != nil || numEvals < 1 {
		t.Skip("Skipping evals. Use GOEVALS=1 flag to run.")
	}

	return numEvals
}

func Run(t *testing.T, name string, f func(e *EvalT)) {
	numEvals := NumEvalsOrSkip(t)

	eval, err := NewEval()
	require.NoError(t, err)

	e := &EvalT{T: t, Eval: eval}

	t.Run(name, func(t *testing.T) {
		e.T = t
		for i := range numEvals {
			e.runNumber = i
			f(e)
		}
	})
}
