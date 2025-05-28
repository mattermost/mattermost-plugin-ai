// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mmtools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	MinSearchTermLength = 3
	MaxSearchTermLength = 300
)

type SearchServerArgs struct {
	Term string `jsonschema_description:"The terms to search for in the server. Must be more than 3 and less than 300 characters."`
}

func (p *MMToolProvider) toolSearchServer(llmContext *llm.Context, argsGetter llm.ToolArgumentGetter) (string, error) {
	var args SearchServerArgs
	err := argsGetter(&args)
	if err != nil {
		return "invalid parameters to function", fmt.Errorf("failed to get arguments for tool SearchServer: %w", err)
	}

	// Validate input
	if len(args.Term) < MinSearchTermLength {
		return "search term too short", errors.New("search term too short")
	}
	if len(args.Term) > MaxSearchTermLength {
		return "search term too long", errors.New("search term too long")
	}

	// Check if search service is available
	if p.search == nil {
		return "search functionality is not configured", errors.New("search is not configured")
	}

	// Perform the search
	ctx := context.Background()
	searchResults, err := p.search.Search(ctx, args.Term, embeddings.SearchOptions{
		Limit:  10,
		UserID: llmContext.RequestingUser.Id,
	})
	if err != nil {
		return "there was an error performing the search", fmt.Errorf("search failed: %w", err)
	}

	// Format the results
	formatted := p.formatSearchResults(searchResults, llmContext.RequestingUser.Id)

	return formatted, nil
}

// formatSearchResults formats search results into a readable string
func (p *MMToolProvider) formatSearchResults(results []embeddings.SearchResult, requestingUserID string) string {
	if len(results) == 0 {
		return "No relevant messages found."
	}

	var builder strings.Builder
	builder.WriteString("Found the following relevant messages:\n\n")

	for i, result := range results {
		// Get channel name
		channel, err := p.pluginAPI.GetChannel(result.Document.ChannelID)
		channelName := "Unknown Channel"
		if err == nil {
			switch channel.Type {
			case model.ChannelTypeDirect:
				channelName = "Direct Message"
			case model.ChannelTypeGroup:
				channelName = "Group Message"
			default:
				channelName = channel.DisplayName
				if channelName == "" {
					channelName = channel.Name
				}
			}
		}

		// Get username
		user, err := p.pluginAPI.GetUser(result.Document.UserID)
		username := "Unknown User"
		if err == nil {
			username = user.Username
		}

		// Format the result
		builder.WriteString(fmt.Sprintf("%d. **%s** in ~%s (Score: %.2f)\n",
			i+1, username, channelName, result.Score))

		// Add message content (truncate if too long)
		message := result.Document.Content
		if len(message) > 500 {
			message = message[:497] + "..."
		}
		builder.WriteString(fmt.Sprintf("   %s\n\n", message))
	}

	return builder.String()
}
