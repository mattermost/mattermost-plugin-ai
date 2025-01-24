// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests truncation using words as tokens
func TestBotConversationTruncate(t *testing.T) {
	tests := []struct {
		name               string
		conversation       BotConversation
		resultConversation BotConversation
		maxTokens          int
		isTruncated        bool
	}{
		{
			name: "Truncate to 0",
			conversation: BotConversation{
				Posts: []Post{
					{
						Message: "Hello",
						Role:    PostRoleUser,
					},
					{
						Message: "Hello",
						Role:    PostRoleBot,
					},
				},
			},
			maxTokens:   0,
			isTruncated: true,
			resultConversation: BotConversation{
				Posts: []Post{},
			},
		},
		{
			name: "Truncate removes first post",
			conversation: BotConversation{
				Posts: []Post{
					{
						Message: "Hello",
						Role:    PostRoleUser,
					},
					{
						Message: "Hello",
						Role:    PostRoleBot,
					},
				},
			},
			maxTokens:   1,
			isTruncated: true,
			resultConversation: BotConversation{
				Posts: []Post{
					{
						Message: "Hello",
						Role:    PostRoleBot,
					},
				},
			},
		},
		{
			name: "No truncation",
			conversation: BotConversation{
				Posts: []Post{
					{
						Message: "Hello",
						Role:    PostRoleUser,
					},
					{
						Message: "Hello",
						Role:    PostRoleBot,
					},
				},
			},
			maxTokens:   2,
			isTruncated: false,
			resultConversation: BotConversation{
				Posts: []Post{
					{
						Message: "Hello",
						Role:    PostRoleUser,
					},
					{
						Message: "Hello",
						Role:    PostRoleBot,
					},
				},
			},
		},
		{
			name: "Partial Truncation",
			conversation: BotConversation{
				Posts: []Post{
					{
						Message: "one two three",
						Role:    PostRoleUser,
					},
					{
						Message: "four five six",
						Role:    PostRoleBot,
					},
				},
			},
			maxTokens:   5,
			isTruncated: true,
			resultConversation: BotConversation{
				Posts: []Post{
					{
						Message: "two three",
						Role:    PostRoleUser,
					},
					{
						Message: "four five six",
						Role:    PostRoleBot,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wordsAsTokensCounter := func(str string) int { return len(strings.Fields(str)) }
			assert.Equal(t, tt.isTruncated, tt.conversation.Truncate(tt.maxTokens, wordsAsTokensCounter))
			assert.Equal(t, tt.resultConversation, tt.conversation)
		})
	}
}
