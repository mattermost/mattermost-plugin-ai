// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-ai/server/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	SearchResultsProp = "search_results"
	SearchQueryProp   = "search_query"
)

type SearchRequest struct {
	Query      string `json:"query"`
	TeamID     string `json:"teamId"`
	ChannelID  string `json:"channelId"`
	MaxResults int    `json:"maxResults"`
}

type SearchResponse struct {
	Answer    string      `json:"answer"`
	Results   []RAGResult `json:"results"`
	PostID    string      `json:"postId,omitempty"`
	ChannelID string      `json:"channelId,omitempty"`
}

type RAGResult struct {
	PostID      string  `json:"postId"`
	ChannelID   string  `json:"channelId"`
	ChannelName string  `json:"channelName"`
	UserID      string  `json:"userId"`
	Username    string  `json:"username"`
	Content     string  `json:"content"`
	Score       float32 `json:"score"`
}

// convertToRAGResults converts embeddings.SearchResult to RAGResult with enriched metadata
func (p *Plugin) convertToRAGResults(searchResults []embeddings.SearchResult) []RAGResult {
	var ragResults []RAGResult
	for _, result := range searchResults {
		// Get channel name
		var channelName string
		channel, chErr := p.pluginAPI.Channel.Get(result.Document.ChannelID)
		if chErr != nil {
			p.pluginAPI.Log.Warn("Failed to get channel", "error", chErr, "channelID", result.Document.ChannelID)
			channelName = "Unknown Channel"
		} else {
			if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
				channelName = "Direct Message"
			} else {
				channelName = channel.DisplayName
			}
		}

		// Get username
		var username string
		user, userErr := p.pluginAPI.User.Get(result.Document.UserID)
		if userErr != nil {
			p.pluginAPI.Log.Warn("Failed to get user", "error", userErr, "userID", result.Document.UserID)
			username = "Unknown User"
		} else {
			username = user.Username
			// If we have the user's first and last name, use those instead
			if user.FirstName != "" || user.LastName != "" {
				username = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
				username = strings.TrimSpace(username)
			}
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

		ragResults = append(ragResults, RAGResult{
			PostID:      result.Document.PostID,
			ChannelID:   result.Document.ChannelID,
			ChannelName: channelName + chunkInfo,
			UserID:      result.Document.UserID,
			Username:    username,
			Content:     content,
			Score:       result.Score,
		})
	}

	return ragResults
}

// formatSearchResults formats search results using the template system
func (p *Plugin) formatSearchResults(results []embeddings.SearchResult) (string, error) {
	ragResults := p.convertToRAGResults(results)

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

// performSearch searches for posts using the given query and returns enriched RAGResult objects
func (p *Plugin) performSearch(ctx context.Context, req SearchRequest, userID string) ([]RAGResult, error) {
	if req.MaxResults == 0 {
		req.MaxResults = 5
	}

	if p.search == nil {
		return nil, fmt.Errorf("search functionality is not configured")
	}

	// Search for relevant posts using embeddings
	searchResults, err := p.search.Search(ctx, req.Query, embeddings.SearchOptions{
		Limit:     req.MaxResults,
		TeamID:    req.TeamID,
		ChannelID: req.ChannelID,
		UserID:    userID,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return p.convertToRAGResults(searchResults), nil
}

// searchAndCreatePrompt combines search and prompt creation into a single operation
func (p *Plugin) searchAndCreatePrompt(ctx context.Context, req SearchRequest, userID string) ([]RAGResult, llm.CompletionRequest, error) {
	ragResults, err := p.performSearch(ctx, req, userID)
	if err != nil {
		return nil, llm.CompletionRequest{}, err
	}

	if len(ragResults) == 0 {
		return ragResults, llm.CompletionRequest{}, nil
	}

	promptCtx := llm.NewContext()
	promptCtx.Parameters = map[string]interface{}{
		"Query":   req.Query,
		"Results": ragResults,
	}

	systemMessage, err := p.prompts.Format("search_system", promptCtx)
	if err != nil {
		return ragResults, llm.CompletionRequest{}, fmt.Errorf("failed to format system message: %w", err)
	}

	prompt := llm.CompletionRequest{
		Posts: []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: systemMessage,
			},
			{
				Role:    llm.PostRoleUser,
				Message: req.Query,
			},
		},
		Context: promptCtx,
	}

	return ragResults, prompt, nil
}

func (p *Plugin) handleRunSearch(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	bot := c.MustGet(ContextBotKey).(*Bot)

	if p.search == nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("search functionality is not configured"))
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	if req.Query == "" {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("query cannot be empty"))
		return
	}

	// Create the initial question post
	questionPost := &model.Post{
		UserId:  userID,
		Message: req.Query,
	}
	questionPost.AddProp(SearchQueryProp, "true")
	if err := p.pluginAPI.Post.DM(userID, bot.mmBot.UserId, questionPost); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create question post: %w", err))
		return
	}

	// Return the response early to update the UI immediately
	c.JSON(http.StatusOK, SearchResponse{
		PostID:    questionPost.Id,
		ChannelID: questionPost.ChannelId,
		Results:   []RAGResult{},
	})

	// Process the rest of the search asynchronously
	go func(ctx context.Context, requestCopy SearchRequest) {
		// Create response post as a reply
		responsePost := &model.Post{
			RootId: questionPost.Id,
		}
		responsePost.AddProp(NoRegen, "true")

		if err := p.botDM(bot.mmBot.UserId, userID, responsePost); err != nil {
			// Not much point in retrying if this failed. (very unlikely beyond dev)
			p.pluginAPI.Log.Error("Error creating bot DM", "error", err)
			return
		}

		// Setup error handling to update the post on error
		var processingError error
		defer func() {
			if processingError != nil {
				responsePost.Message = "I encountered an error while searching. Please try again later. See server logs for details."
				if err := p.pluginAPI.Post.UpdatePost(responsePost); err != nil {
					p.API.LogError("Error updating post on error", "error", err)
				}
			}
		}()

		// Perform search and create prompt in one step
		ragResults, prompt, err := p.searchAndCreatePrompt(ctx, requestCopy, userID)
		if err != nil {
			p.pluginAPI.Log.Error("Error performing search and creating prompt", "error", err)
			processingError = err
			return
		}

		if len(ragResults) == 0 {
			responsePost.Message = "I couldn't find any relevant messages for your query. Please try a different search term."
			if updateErr := p.pluginAPI.Post.UpdatePost(responsePost); updateErr != nil {
				p.API.LogError("Error updating post on error", "error", updateErr)
			}
			return
		}

		resultStream, err := p.getLLM(bot.cfg).ChatCompletion(prompt)
		if err != nil {
			p.pluginAPI.Log.Error("Error generating answer", "error", err)
			processingError = err
			return
		}

		resultsJSON, err := json.Marshal(ragResults)
		if err != nil {
			p.pluginAPI.Log.Error("Error marshaling results", "error", err)
			processingError = err
			return
		}

		// Update post to add sources
		responsePost.AddProp(SearchResultsProp, string(resultsJSON))
		if updateErr := p.pluginAPI.Post.UpdatePost(responsePost); updateErr != nil {
			p.API.LogError("Error updating post for search results", "error", updateErr)
			processingError = updateErr
			return
		}

		streamContext, err := p.getPostStreamingContext(ctx, responsePost.Id)
		if err != nil {
			p.pluginAPI.Log.Error("Error getting post streaming context", "error", err)
			processingError = err
			return
		}
		defer p.finishPostStreaming(responsePost.Id)
		p.streamResultToPost(streamContext, resultStream, responsePost, "")
	}(c.Request.Context(), req)
}

func (p *Plugin) handleSearchQuery(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	bot := c.MustGet(ContextBotKey).(*Bot)

	if p.search == nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("search functionality is not configured"))
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	ragResults, prompt, err := p.searchAndCreatePrompt(c.Request.Context(), req, userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if len(ragResults) == 0 {
		c.JSON(http.StatusOK, SearchResponse{
			Answer:  "I couldn't find any relevant messages for your query. Please try a different search term.",
			Results: []RAGResult{},
		})
		return
	}

	answer, err := p.getLLM(bot.cfg).ChatCompletionNoStream(prompt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to generate answer: %w", err))
		return
	}

	c.JSON(http.StatusOK, SearchResponse{
		Answer:  answer,
		Results: ragResults,
	})
}
