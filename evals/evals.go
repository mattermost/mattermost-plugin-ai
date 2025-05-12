// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package evals

import (
	"errors"
	"flag"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/openai"
	"github.com/stretchr/testify/require"
)

var runEvals = flag.Bool("eval", false, "Run LLM evals")
var numEvals = flag.Int("num", 1, "Number of times to run each eval")

type EvalT struct {
	*testing.T
	*Eval
}

type Eval struct {
	LLM       llm.LanguageModel
	GraderLLM llm.LanguageModel
	Prompts   *llm.Prompts
}

func NewEval() (*Eval, error) {
	// Setup prompts
	prompts, err := llm.NewPrompts(llm.PromptsFolder)
	if err != nil {
		return nil, err
	}

	// Setup real LLM
	httpClient := http.Client{}
	provider := openai.New(openai.Config{
		APIKey:           os.Getenv("OPENAI_API_KEY"),
		DefaultModel:     "gpt-4o",
		StreamingTimeout: 20 * time.Second,
	}, &httpClient, nil)
	if provider == nil {
		return nil, errors.New("failed to create LLM provider")
	}

	return &Eval{
		Prompts:   prompts,
		LLM:       provider,
		GraderLLM: provider, // TODO: use a different LLM for grading
	}, nil
}

func SkipWithoutEvalsFlag(t *testing.T) {
	if !*runEvals {
		t.Skip("Skipping evals. Use -eval flag to run.")
	}
}

func Run(t *testing.T, name string, f func(e *EvalT)) {
	SkipWithoutEvalsFlag(t)

	eval, err := NewEval()
	require.NoError(t, err)

	e := &EvalT{T: t, Eval: eval}

	t.Run(name, func(t *testing.T) {
		for range *numEvals {
			f(e)
		}
	})
}
