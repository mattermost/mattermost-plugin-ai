// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitPlaintextOnSentences(t *testing.T) {
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
}
