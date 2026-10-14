// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package evals

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSummarizeFailure(t *testing.T) {
	tests := []struct {
		name    string
		failure string
		want    string
	}{
		{
			name: "testify error with message",
			failure: "\n\tError Trace:\t/repo/conversations/eval_harness_test.go:160\n" +
				"\t            \t/repo/evals/evals.go:290\n" +
				"\tError:      \tReceived unexpected error:\n" +
				"\t            \tbifrost error: model not found (status=404)\n" +
				"\tTest:       \tTestReactEval/[openai]_react_cat_message\n" +
				"\tMessages:   \tfailed to read the agent's reply stream\n",
			want: "failed to read the agent's reply stream: Received unexpected error: bifrost error: model not found (status=404)",
		},
		{
			name: "testify error without message",
			failure: "\n\tError Trace:\t/repo/react/react_test.go:118\n" +
				"\tError:      \tReceived unexpected error:\n" +
				"\t            \tfailed to get emoji from LLM: timeout\n" +
				"\tTest:       \tTestReactEval/[openai]_react_cat_message\n",
			want: "Received unexpected error: failed to get emoji from LLM: timeout",
		},
		{
			name:    "plain message",
			failure: "  agent did not reply  ",
			want:    "agent did not reply",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, summarizeFailure(tc.failure))
		})
	}
}

func TestRedactSecrets(t *testing.T) {
	t.Setenv("EVAL_TEST_API_KEY", "sk-test-1234567890")
	t.Setenv("EVAL_TEST_ENDPOINT", "https://secret-resource.openai.azure.com")
	t.Setenv("EVAL_TEST_MODEL", "gpt-5.6-terra")

	got := redactSecrets("POST https://secret-resource.openai.azure.com with key sk-test-1234567890 for model gpt-5.6-terra")

	assert.Equal(t, "POST [REDACTED] with key [REDACTED] for model gpt-5.6-terra", got)
}
