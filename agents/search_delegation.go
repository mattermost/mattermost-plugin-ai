// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/search"
	"github.com/mattermost/mattermost/server/public/model"
)

// HandleRunSearch delegates to the search service
func (p *AgentsService) HandleRunSearch(userID string, bot *bots.Bot, query, teamID, channelID string, maxResults int) (map[string]string, error) {
	if p.searchService == nil {
		return nil, fmt.Errorf("search functionality is not configured")
	}
	return p.searchService.RunSearch(context.Background(), userID, bot, query, teamID, channelID, maxResults)
}

// HandleSearchQuery delegates to the search service
func (p *AgentsService) HandleSearchQuery(userID string, bot *bots.Bot, query, teamID, channelID string, maxResults int) (search.SearchResponse, error) {
	if p.searchService == nil {
		return search.SearchResponse{}, fmt.Errorf("search functionality is not configured")
	}
	return p.searchService.SearchQuery(context.Background(), userID, bot, query, teamID, channelID, maxResults)
}

// formatSearchResults formats search results for internal tool use
func (p *AgentsService) formatSearchResults(results []embeddings.SearchResult) (string, error) {
	// Create a temporary context for formatting
	ragResults := make([]search.RAGResult, 0, len(results))
	for _, result := range results {
		// Get channel name
		var channelName string
		channel, chErr := p.mmClient.GetChannel(result.Document.ChannelID)
		if chErr != nil {
			p.mmClient.LogWarn("Failed to get channel", "error", chErr, "channelID", result.Document.ChannelID)
			channelName = "Unknown Channel"
		} else {
			switch channel.Type {
			case model.ChannelTypeDirect:
				channelName = "Direct Message"
			case model.ChannelTypeGroup:
				channelName = "Group Message"
			default:
				channelName = channel.DisplayName
			}
		}

		// Get username
		var username string
		user, userErr := p.mmClient.GetUser(result.Document.UserID)
		if userErr != nil {
			p.mmClient.LogWarn("Failed to get user", "error", userErr, "userID", result.Document.UserID)
			username = "Unknown User"
		} else {
			username = user.Username
		}

		// Determine the correct content to show
		content := result.Document.Content

		// Handle additional metadata for chunks
		var chunkInfo string
		if result.Document.IsChunk {
			chunkInfo = fmt.Sprintf(" (Chunk %d of %d)",
				result.Document.ChunkIndex+1,
				result.Document.TotalChunks)
		}

		ragResults = append(ragResults, search.RAGResult{
			PostID:      result.Document.PostID,
			ChannelID:   result.Document.ChannelID,
			ChannelName: channelName + chunkInfo,
			UserID:      result.Document.UserID,
			Username:    username,
			Content:     content,
			Score:       result.Score,
		})
	}

	// Create context for the prompt
	ctx := llm.NewContext()
	ctx.Parameters = map[string]interface{}{
		"Results": ragResults,
	}

	// Format using the search_results template
	formatted, err := p.prompts.Format("search_results", ctx)
	if err != nil {
		return "", fmt.Errorf("failed to format search results: %w", err)
	}

	return formatted, nil
}
