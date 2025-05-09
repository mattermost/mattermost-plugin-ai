// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitPlaintextOnSentences(t *testing.T) {
	// Original test cases
	for i, test := range []struct {
		input  string
		size   int
		output []string
	}{
		{
			"Hello. How are you! I'm doing well. Thanks!",
			10,
			[]string{"Hello. How", "are you!", "I'm doing", "well. Than", "ks!"},
		},
		{
			"Hello. How are you! I'm doing well.",
			20,
			[]string{"Hello. How are you!", "I'm doing well."},
		},
		{
			"Hello. How are you! I'm doing well.",
			25,
			[]string{"Hello. How are you!", "I'm doing well."},
		},
		{
			"Hello. How are you! I'm doing well.",
			32,
			[]string{"Hello. How are you! I'm doing we", "ll."},
		},
	} {
		t.Run("test "+strconv.Itoa(i), func(t *testing.T) {
			actual := splitPlaintextOnSentences(test.input, test.size)
			require.Equal(t, test.output, actual)
		})
	}

	// Additional test cases testing the intended behavior
	t.Run("Empty string", func(t *testing.T) {
		chunks := splitPlaintextOnSentences("", 100)
		assert.Equal(t, 1, len(chunks), "Should return a single chunk for empty string")
		assert.Equal(t, "", chunks[0], "Empty string should return empty chunk")
	})

	t.Run("Text with various sentence boundaries", func(t *testing.T) {
		input := "This is a statement. Is this a question? Yes, it is! This ends with ellipsis..."
		chunks := splitPlaintextOnSentences(input, 20)

		// Find at least one chunk ending with each type of sentence boundary
		foundPeriod := false
		foundQuestion := false
		foundExclamation := false

		for _, chunk := range chunks {
			if strings.HasSuffix(chunk, ".") {
				foundPeriod = true
			}
			if strings.HasSuffix(chunk, "?") {
				foundQuestion = true
			}
			if strings.HasSuffix(chunk, "!") {
				foundExclamation = true
			}
		}

		// Assert we found at least some sentence boundaries
		assert.True(t, foundPeriod || foundQuestion || foundExclamation,
			"Should preserve at least some sentence boundaries")

		// Check that no chunk exceeds the maximum size
		for i, chunk := range chunks {
			assert.LessOrEqual(t, len(chunk), 20, "Chunk %d exceeds maximum size", i)
		}
	})

	t.Run("Very long sentence beyond chunk size", func(t *testing.T) {
		input := "This is an extremely long sentence without any sentence boundaries that should be split based purely on the chunk size limit and not on sentence boundaries because there are none to be found here"
		chunkSize := 30
		chunks := splitPlaintextOnSentences(input, chunkSize)

		// Verify no chunk exceeds the maximum size
		for i, chunk := range chunks {
			assert.LessOrEqual(t, len(chunk), chunkSize, "Chunk %d exceeds maximum size", i)
		}

		// Verify we get back the full text (ignoring whitespace differences)
		combined := strings.Join(chunks, " ")
		assert.Equal(t, len(strings.ReplaceAll(input, " ", "")), len(strings.ReplaceAll(combined, " ", "")),
			"Combined chunks should contain all input text")
	})

	t.Run("Respects minimum chunk size", func(t *testing.T) {
		input := "Short. Another. Third. Fourth. Fifth. A slightly longer sentence to end with."
		chunkSize := 30
		minSize := int(float64(chunkSize) * 0.75)
		chunks := splitPlaintextOnSentences(input, chunkSize)

		// Verify that chunks (except possibly the last one) meet the minimum size
		for i, chunk := range chunks[:len(chunks)-1] {
			assert.GreaterOrEqual(t, len(chunk), minSize,
				"Chunk %d should meet minimum size requirement: %q", i, chunk)
		}
	})
}
