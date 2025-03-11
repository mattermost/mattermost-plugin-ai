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

func formatSearchResults(results []embeddings.SearchResult) string {
	var result strings.Builder
	result.WriteString("<posts>\n")
	for _, r := range results {
		result.WriteString(fmt.Sprintf(`<post id="%s" channelID="%s" teamID="%s">%s</post>%s`, r.Document.Post.Id, r.Document.ChannelID, r.Document.TeamID, r.Document.Content, "\n"))
	}
	result.WriteString("</posts>\n")

	return result.String()
}

func (p *Plugin) performSearch(ctx context.Context, req SearchRequest) ([]RAGResult, error) {
	if req.MaxResults == 0 {
		req.MaxResults = 5
	}

	// Search for relevant posts using embeddings
	searchResults, err := p.search.Search(ctx, req.Query, embeddings.SearchOptions{
		Limit:     req.MaxResults,
		TeamID:    req.TeamID,
		ChannelID: req.ChannelID,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

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

		ragResults = append(ragResults, RAGResult{
			PostID:      result.Document.Post.Id,
			ChannelID:   result.Document.ChannelID,
			ChannelName: channelName,
			UserID:      result.Document.UserID,
			Username:    username,
			Content:     result.Document.Content,
			Score:       result.Score,
		})
	}

	return ragResults, nil
}

func (p *Plugin) createSearchPrompt(results []RAGResult, query string) (llm.CompletionRequest, error) {
	// Create context for the prompt
	ctx := llm.NewContext()
	ctx.Parameters = map[string]interface{}{
		"Query":   query,
		"Results": results,
	}

	// Format the system message with search results
	systemMessage, err := p.prompts.Format("search_system", ctx)
	if err != nil {
		return llm.CompletionRequest{}, fmt.Errorf("failed to format system message: %w", err)
	}

	// Create the completion request with system and user messages
	return llm.CompletionRequest{
		Posts: []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: systemMessage,
			},
			{
				Role:    llm.PostRoleUser,
				Message: query,
			},
		},
		Context: ctx,
	}, nil
}

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

func (p *Plugin) handleRunSearch(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	bot := c.MustGet(ContextBotKey).(*Bot)

	var req SearchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	// Validate the request
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
		// Perform the search
		ragResults, err := p.performSearch(ctx, requestCopy)
		if err != nil {
			p.pluginAPI.Log.Error("Error performing search", "error", err)
			responsePost := &model.Post{
				UserId:    bot.mmBot.UserId,
				ChannelId: questionPost.ChannelId,
				RootId:    questionPost.Id,
				Message:   "I encountered an error while searching. Please try again later.",
			}
			responsePost.AddProp(NoRegen, "true")
			if postErr := p.pluginAPI.Post.CreatePost(responsePost); postErr != nil {
				p.pluginAPI.Log.Error("Error creating error response post", "error", postErr)
			}
			return
		}

		// Handle case with no results
		if len(ragResults) == 0 {
			responsePost := &model.Post{
				UserId:    bot.mmBot.UserId,
				ChannelId: questionPost.ChannelId,
				RootId:    questionPost.Id,
				Message:   "I couldn't find any relevant messages for your query. Please try a different search term.",
			}
			responsePost.AddProp(NoRegen, "true")
			if postErr := p.pluginAPI.Post.CreatePost(responsePost); postErr != nil {
				p.pluginAPI.Log.Error("Error creating empty results response post", "error", postErr)
			}
			return
		}

		// Create search prompt with results
		prompt, promptErr := p.createSearchPrompt(ragResults, requestCopy.Query)
		if promptErr != nil {
			p.pluginAPI.Log.Error("Error creating search prompt", "error", promptErr)
			return
		}

		resultStream, streamErr := p.getLLM(bot.cfg).ChatCompletion(prompt)
		if streamErr != nil {
			p.pluginAPI.Log.Error("Error generating answer", "error", streamErr)
			return
		}

		resultsJSON, jsonErr := json.Marshal(ragResults)
		if jsonErr != nil {
			p.pluginAPI.Log.Error("Error marshaling results", "error", jsonErr)
			return
		}

		// Create response post as a reply
		responsePost := &model.Post{
			RootId: questionPost.Id,
		}
		responsePost.AddProp(NoRegen, "true")
		responsePost.AddProp(SearchResultsProp, string(resultsJSON))

		if streamErr := p.streamResultToNewDM(bot.mmBot.UserId, resultStream, userID, responsePost); streamErr != nil {
			p.pluginAPI.Log.Error("Error streaming result", "error", streamErr)
			return
		}
	}(c.Request.Context(), req)
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

func (p *Plugin) handleChannelSearch(c *gin.Context) {
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*Bot)

	var req SearchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	// Force channel ID to be the current channel
	req.ChannelID = channel.Id

	ragResults, err := p.performSearch(c.Request.Context(), req)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	prompt, err := p.createSearchPrompt(ragResults, req.Query)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create search prompt: %w", err))
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

func (p *Plugin) handleSearchQuery(c *gin.Context) {
	bot := c.MustGet(ContextBotKey).(*Bot)

	var req SearchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	ragResults, err := p.performSearch(c.Request.Context(), req)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	prompt, err := p.createSearchPrompt(ragResults, req.Query)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create search prompt: %w", err))
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
