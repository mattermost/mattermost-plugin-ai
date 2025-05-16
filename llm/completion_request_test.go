// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompletionRequestTruncate(t *testing.T) {
	// Mock token counting function that simply counts characters divided by 4
	// This is a simplified approximation of how token counting works
	mockTokenCounter := func(text string) int {
		return len(text) / 4
	}

	t.Run("No truncation needed", func(t *testing.T) {
		// Create a request with messages that are well under the token limit
		req := CompletionRequest{
			Posts: []Post{
				{Role: PostRoleSystem, Message: "You are a helpful assistant."},
				{Role: PostRoleUser, Message: "Hello, how are you?"},
				{Role: PostRoleBot, Message: "I'm doing well, thank you for asking!"},
			},
		}

		// Token count: ~24 tokens, limit is 50
		wasTruncated := req.Truncate(50, mockTokenCounter)

		assert.False(t, wasTruncated, "Should not need truncation")
		assert.Equal(t, 3, len(req.Posts), "Should have same number of posts")
		assert.Equal(t, "You are a helpful assistant.", req.Posts[0].Message, "First message should be unchanged")
	})

	t.Run("Remove oldest messages", func(t *testing.T) {
		// Create a request with more messages than the token limit allows
		req := CompletionRequest{
			Posts: []Post{
				{Role: PostRoleSystem, Message: "You are a helpful assistant that provides concise answers."},
				{Role: PostRoleUser, Message: "What is the capital of France?"},
				{Role: PostRoleBot, Message: "The capital of France is Paris."},
				{Role: PostRoleUser, Message: "What is the population of Paris?"},
				{Role: PostRoleBot, Message: "The population of Paris is approximately 2.2 million in the city proper."},
			},
		}

		// Token count: ~72 tokens, limit is 40
		wasTruncated := req.Truncate(40, mockTokenCounter)

		assert.True(t, wasTruncated, "Should truncate messages")
		assert.Less(t, len(req.Posts), 5, "Should have fewer posts")

		// The system message and earliest conversation should be dropped
		lastUserMsg := "What is the population of Paris?"
		lastBotMsg := "The population of Paris is approximately 2.2 million in the city proper."

		// Check that the most recent messages are preserved
		found := false
		for _, post := range req.Posts {
			if post.Message == lastUserMsg || post.Message == lastBotMsg {
				found = true
				break
			}
		}
		assert.True(t, found, "Recent messages should be preserved")
	})

	t.Run("Truncate single message", func(t *testing.T) {
		// Create a request with a single message that exceeds the token limit
		longMessage := strings.Repeat("This is a very long message that needs to be truncated. ", 10)
		req := CompletionRequest{
			Posts: []Post{
				{Role: PostRoleUser, Message: longMessage},
			},
		}

		// Limit is much smaller than the message
		wasTruncated := req.Truncate(20, mockTokenCounter)

		assert.True(t, wasTruncated, "Should truncate message")
		assert.Equal(t, 1, len(req.Posts), "Should still have one post")
		assert.NotEqual(t, longMessage, req.Posts[0].Message, "Message should be truncated")
		assert.Less(t, len(req.Posts[0].Message), len(longMessage), "Truncated message should be shorter")

		// Verify token count is within limit
		tokenCount := mockTokenCounter(req.Posts[0].Message)
		assert.LessOrEqual(t, tokenCount, 20, "Truncated message should be within token limit")
	})
}
