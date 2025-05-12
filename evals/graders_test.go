// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package evals

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMRubric(t *testing.T) {
	SkipWithoutEvalsFlag(t)
	eval, err := NewEval()
	require.NoError(t, err)

	rubric := "The output is in spanish"
	output := "La comida es muy rico"

	result, err := eval.LLMRubric(rubric, output)
	require.NoError(t, err)

	t.Log("Rubric result:", result)
	assert.Equal(t, true, result.Pass)
	assert.GreaterOrEqual(t, result.Score, 0.9)
	assert.NotEmpty(t, result.Reasoning)
}
