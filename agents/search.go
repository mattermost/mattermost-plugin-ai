// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	SearchResultsProp = "search_results"
	SearchQueryProp   = "search_query"
)

// SearchRequest represents a search query request
type SearchRequest struct {
	Query      string `json:"query"`
	TeamID     string `json:"teamId"`
	ChannelID  string `json:"channelId"`
	MaxResults int    `json:"maxResults"`
}

// SearchResponse represents a response to a search query
type SearchResponse struct {
	Answer    string      `json:"answer"`
	Results   []RAGResult `json:"results"`
	PostID    string      `json:"postId,omitempty"`
	ChannelID string      `json:"channelId,omitempty"`
}

// RAGResult represents an enriched search result with metadata
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
func (p *AgentsService) convertToRAGResults(searchResults []embeddings.SearchResult) []RAGResult {
	var ragResults []RAGResult
	for _, result := range searchResults {
		// Get channel name
		var channelName string
		channel, chErr := p.pluginAPI.Channel.Get(result.Document.ChannelID)
		if chErr != nil {
			p.pluginAPI.Log.Warn("Failed to get channel", "error", chErr, "channelID", result.Document.ChannelID)
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
		user, userErr := p.pluginAPI.User.Get(result.Document.UserID)
		if userErr != nil {
			p.pluginAPI.Log.Warn("Failed to get user", "error", userErr, "userID", result.Document.UserID)
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
func (p *AgentsService) formatSearchResults(results []embeddings.SearchResult) (string, error) {
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

// HandleRunSearch initiates a search and sends results to a DM
func (p *AgentsService) HandleRunSearch(userID string, bot *bots.Bot, query, teamID, channelID string, maxResults int) (map[string]string, error) {
	if p.search == nil {
		return nil, fmt.Errorf("search functionality is not configured")
	}

	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Create the initial question post
	questionPost := &model.Post{
		UserId:  userID,
		Message: query,
	}
	questionPost.AddProp(SearchQueryProp, "true")
	if err := p.pluginAPI.Post.DM(userID, bot.GetMMBot().UserId, questionPost); err != nil {
		return nil, fmt.Errorf("failed to create question post: %w", err)
	}

	// Start processing the search asynchronously
	go func(ctx context.Context, query, teamID, channelID string, maxResults int) {
		// Create response post as a reply
		responsePost := &model.Post{
			RootId: questionPost.Id,
		}
		responsePost.AddProp(NoRegen, "true")

		if err := p.botDMNonResponse(bot.GetMMBot().UserId, userID, responsePost); err != nil {
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
					p.pluginAPI.Log.Error("Error updating post on error", "error", err)
				}
			}
		}()

		// Perform search
		if maxResults == 0 {
			maxResults = 5
		}

		searchResults, err := p.search.Search(ctx, query, embeddings.SearchOptions{
			Limit:     maxResults,
			TeamID:    teamID,
			ChannelID: channelID,
			UserID:    userID,
		})
		if err != nil {
			p.pluginAPI.Log.Error("Error performing search", "error", err)
			processingError = err
			return
		}

		ragResults := p.convertToRAGResults(searchResults)
		if len(ragResults) == 0 {
			responsePost.Message = "I couldn't find any relevant messages for your query. Please try a different search term."
			if updateErr := p.pluginAPI.Post.UpdatePost(responsePost); updateErr != nil {
				p.pluginAPI.Log.Error("Error updating post on error", "error", updateErr)
			}
			return
		}

		// Create context for generating answer
		promptCtx := llm.NewContext()
		promptCtx.Parameters = map[string]interface{}{
			"Query":   query,
			"Results": ragResults,
		}

		systemMessage, err := p.prompts.Format("search_system", promptCtx)
		if err != nil {
			p.pluginAPI.Log.Error("Error formatting system message", "error", err)
			processingError = err
			return
		}

		prompt := llm.CompletionRequest{
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
			Context: promptCtx,
		}

		resultStream, err := p.GetLLM(bot.GetConfig()).ChatCompletion(prompt)
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
			p.pluginAPI.Log.Error("Error updating post for search results", "error", updateErr)
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
	}(context.Background(), query, teamID, channelID, maxResults)

	return map[string]string{
		"PostID":    questionPost.Id,
		"ChannelID": questionPost.ChannelId,
	}, nil
}

// HandleSearchQuery performs a search and returns results immediately
func (p *AgentsService) HandleSearchQuery(userID string, bot *bots.Bot, query, teamID, channelID string, maxResults int) (SearchResponse, error) {
	if p.search == nil {
		return SearchResponse{}, fmt.Errorf("search functionality is not configured")
	}

	if maxResults == 0 {
		maxResults = 5
	}

	// Search for relevant posts using embeddings
	searchResults, err := p.search.Search(context.Background(), query, embeddings.SearchOptions{
		Limit:     maxResults,
		TeamID:    teamID,
		ChannelID: channelID,
		UserID:    userID,
	})
	if err != nil {
		return SearchResponse{}, fmt.Errorf("search failed: %w", err)
	}

	ragResults := p.convertToRAGResults(searchResults)
	if len(ragResults) == 0 {
		return SearchResponse{
			Answer:  "I couldn't find any relevant messages for your query. Please try a different search term.",
			Results: []RAGResult{},
		}, nil
	}

	promptCtx := llm.NewContext()
	promptCtx.Parameters = map[string]interface{}{
		"Query":   query,
		"Results": ragResults,
	}

	systemMessage, err := p.prompts.Format("search_system", promptCtx)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("failed to format system message: %w", err)
	}

	prompt := llm.CompletionRequest{
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
		Context: promptCtx,
	}

	answer, err := p.GetLLM(bot.GetConfig()).ChatCompletionNoStream(prompt)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("failed to generate answer: %w", err)
	}

	return SearchResponse{
		Answer:  answer,
		Results: ragResults,
	}, nil
}

// HandleReindexPosts starts a post reindexing job
func (p *AgentsService) HandleReindexPosts() (JobStatus, error) {
	// Check if search is initialized
	if p.search == nil {
		return JobStatus{}, fmt.Errorf("search functionality is not configured")
	}

	// Check if a job is already running
	var jobStatus JobStatus
	err := p.pluginAPI.KV.Get(ReindexJobKey, &jobStatus)
	if err != nil && err.Error() != "not found" {
		return JobStatus{}, fmt.Errorf("failed to check job status: %w", err)
	}

	// If we have a valid job status and it's running, return conflict
	if jobStatus.Status == JobStatusRunning {
		return jobStatus, fmt.Errorf("job already running")
	}

	// Get an estimate of total posts for progress tracking
	var count int64
	dbErr := p.db.Get(&count, `SELECT COUNT(*) FROM Posts WHERE DeleteAt = 0 AND Message != '' AND Type = ''`)
	if dbErr != nil {
		p.pluginAPI.Log.Warn("Failed to get post count for progress tracking", "error", dbErr)
		count = 0 // Continue with zero estimate
	}

	// Create initial job status
	newJobStatus := JobStatus{
		Status:    JobStatusRunning,
		StartedAt: time.Now(),
		TotalRows: count,
	}

	// Save initial job status
	_, err = p.pluginAPI.KV.Set(ReindexJobKey, newJobStatus)
	if err != nil {
		return JobStatus{}, fmt.Errorf("failed to save job status: %w", err)
	}

	// Start the reindexing job in background
	go p.runReindexJob(&newJobStatus)

	return newJobStatus, nil
}

// GetJobStatus gets the status of the reindex job
func (p *AgentsService) GetJobStatus() (JobStatus, error) {
	var jobStatus JobStatus
	err := p.pluginAPI.KV.Get(ReindexJobKey, &jobStatus)
	if err != nil {
		return JobStatus{}, err
	}
	return jobStatus, nil
}

// CancelJob cancels a running reindex job
func (p *AgentsService) CancelJob() (JobStatus, error) {
	var jobStatus JobStatus
	err := p.pluginAPI.KV.Get(ReindexJobKey, &jobStatus)
	if err != nil {
		return JobStatus{}, err
	}

	if jobStatus.Status != JobStatusRunning {
		return JobStatus{}, fmt.Errorf("not running")
	}

	// Update status to canceled
	jobStatus.Status = JobStatusCanceled
	jobStatus.CompletedAt = time.Now()

	// Save updated status
	_, err = p.pluginAPI.KV.Set(ReindexJobKey, jobStatus)
	if err != nil {
		return JobStatus{}, fmt.Errorf("failed to save job status: %w", err)
	}

	return jobStatus, nil
}
