// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/streaming"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	SearchResultsProp = "search_results"
	SearchQueryProp   = "search_query"

	// Constants for post properties
	NoRegen             = "no_regen"
	LLMRequesterUserID  = "llm_requester_user_id"
	UnsafeLinksPostProp = "unsafe_links"
)

// Request represents a search query request
type Request struct {
	Query      string `json:"query"`
	TeamID     string `json:"teamId"`
	ChannelID  string `json:"channelId"`
	MaxResults int    `json:"maxResults"`
}

// Response represents a response to a search query
type Response struct {
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

type Search struct {
	embeddings.EmbeddingSearch
	pluginAPI             mmapi.Client
	prompts               *llm.Prompts
	streamingService      streaming.Service
	llmService            func(llm.BotConfig) llm.LanguageModel
	llmUpstreamHTTPClient *http.Client
	db                    *sqlx.DB
	licenseChecker        LicenseChecker
}

type LicenseChecker interface {
	IsBasicsLicensed() bool
}

func New(
	search embeddings.EmbeddingSearch,
	pluginAPI mmapi.Client,
	prompts *llm.Prompts,
	streamingService streaming.Service,
	llmService func(llm.BotConfig) llm.LanguageModel,
	llmUpstreamHTTPClient *http.Client,
	db *sqlx.DB,
	licenseChecker LicenseChecker,
) *Search {
	return &Search{
		EmbeddingSearch:       search,
		pluginAPI:             pluginAPI,
		prompts:               prompts,
		streamingService:      streamingService,
		llmService:            llmService,
		llmUpstreamHTTPClient: llmUpstreamHTTPClient,
		db:                    db,
		licenseChecker:        licenseChecker,
	}
}

// convertToRAGResults converts embeddings.EmbeddingSearchResult to RAGResult with enriched metadata
func (s *Search) convertToRAGResults(searchResults []embeddings.SearchResult) []RAGResult {
	var ragResults []RAGResult
	for _, result := range searchResults {
		// Get channel name
		var channelName string
		channel, chErr := s.pluginAPI.GetChannel(result.Document.ChannelID)
		if chErr != nil {
			s.pluginAPI.LogWarn("Failed to get channel", "error", chErr, "channelID", result.Document.ChannelID)
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
		user, userErr := s.pluginAPI.GetUser(result.Document.UserID)
		if userErr != nil {
			s.pluginAPI.LogWarn("Failed to get user", "error", userErr, "userID", result.Document.UserID)
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

// RunSearch initiates a search and sends results to a DM
func (s *Search) RunSearch(ctx context.Context, userID string, bot *bots.Bot, query, teamID, channelID string, maxResults int) (map[string]string, error) {
	if s.EmbeddingSearch == nil {
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
	if err := s.pluginAPI.DM(userID, bot.GetMMBot().UserId, questionPost); err != nil {
		return nil, fmt.Errorf("failed to create question post: %w", err)
	}

	// Start processing the search asynchronously
	go func(query, teamID, channelID string, maxResults int) {
		// Create response post as a reply
		responsePost := &model.Post{
			RootId: questionPost.Id,
		}
		responsePost.AddProp(NoRegen, "true")

		if err := s.botDMNonResponse(bot.GetMMBot().UserId, userID, responsePost); err != nil {
			// Not much point in retrying if this failed. (very unlikely beyond dev)
			s.pluginAPI.LogError("Error creating bot DM", "error", err)
			return
		}

		// Setup error handling to update the post on error
		var processingError error
		defer func() {
			if processingError != nil {
				responsePost.Message = "I encountered an error while searching. Please try again later. See server logs for details."
				if err := s.pluginAPI.UpdatePost(responsePost); err != nil {
					s.pluginAPI.LogError("Error updating post on error", "error", err)
				}
			}
		}()

		// Perform search
		if maxResults == 0 {
			maxResults = 5
		}

		searchResults, err := s.Search(context.Background(), query, embeddings.SearchOptions{
			Limit:     maxResults,
			TeamID:    teamID,
			ChannelID: channelID,
			UserID:    userID,
		})
		if err != nil {
			s.pluginAPI.LogError("Error performing search", "error", err)
			processingError = err
			return
		}

		ragResults := s.convertToRAGResults(searchResults)
		if len(ragResults) == 0 {
			responsePost.Message = "I couldn't find any relevant messages for your query. Please try a different search term."
			if updateErr := s.pluginAPI.UpdatePost(responsePost); updateErr != nil {
				s.pluginAPI.LogError("Error updating post on error", "error", updateErr)
			}
			return
		}

		// Create context for generating answer
		promptCtx := llm.NewContext()
		promptCtx.Parameters = map[string]interface{}{
			"Query":   query,
			"Results": ragResults,
		}

		systemMessage, err := s.prompts.Format("search_system", promptCtx)
		if err != nil {
			s.pluginAPI.LogError("Error formatting system message", "error", err)
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

		resultStream, err := s.llmService(bot.GetConfig()).ChatCompletion(prompt)
		if err != nil {
			s.pluginAPI.LogError("Error generating answer", "error", err)
			processingError = err
			return
		}

		resultsJSON, err := json.Marshal(ragResults)
		if err != nil {
			s.pluginAPI.LogError("Error marshaling results", "error", err)
			processingError = err
			return
		}

		// Update post to add sources
		responsePost.AddProp(SearchResultsProp, string(resultsJSON))
		if updateErr := s.pluginAPI.UpdatePost(responsePost); updateErr != nil {
			s.pluginAPI.LogError("Error updating post for search results", "error", updateErr)
			processingError = updateErr
			return
		}

		streamContext, err := s.streamingService.GetStreamingContext(context.Background(), responsePost.Id)
		if err != nil {
			s.pluginAPI.LogError("Error getting post streaming context", "error", err)
			processingError = err
			return
		}
		defer s.streamingService.FinishStreaming(responsePost.Id)
		s.streamingService.StreamToPost(streamContext, resultStream, responsePost, "")
	}(query, teamID, channelID, maxResults)

	return map[string]string{
		"PostID":    questionPost.Id,
		"ChannelID": questionPost.ChannelId,
	}, nil
}

// SearchQuery performs a search and returns results immediately
func (s *Search) SearchQuery(ctx context.Context, userID string, bot *bots.Bot, query, teamID, channelID string, maxResults int) (Response, error) {
	if s.EmbeddingSearch == nil {
		return Response{}, fmt.Errorf("search functionality is not configured")
	}

	if maxResults == 0 {
		maxResults = 5
	}

	// Search for relevant posts using embeddings
	searchResults, err := s.Search(ctx, query, embeddings.SearchOptions{
		Limit:     maxResults,
		TeamID:    teamID,
		ChannelID: channelID,
		UserID:    userID,
	})
	if err != nil {
		return Response{}, fmt.Errorf("search failed: %w", err)
	}

	ragResults := s.convertToRAGResults(searchResults)
	if len(ragResults) == 0 {
		return Response{
			Answer:  "I couldn't find any relevant messages for your query. Please try a different search term.",
			Results: []RAGResult{},
		}, nil
	}

	promptCtx := llm.NewContext()
	promptCtx.Parameters = map[string]interface{}{
		"Query":   query,
		"Results": ragResults,
	}

	systemMessage, err := s.prompts.Format("search_system", promptCtx)
	if err != nil {
		return Response{}, fmt.Errorf("failed to format system message: %w", err)
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

	answer, err := s.llmService(bot.GetConfig()).ChatCompletionNoStream(prompt)
	if err != nil {
		return Response{}, fmt.Errorf("failed to generate answer: %w", err)
	}

	return Response{
		Answer:  answer,
		Results: ragResults,
	}, nil
}

// Helper functions for post creation
func (s *Search) modifyPostForBot(botid string, requesterUserID string, post *model.Post) {
	post.UserId = botid
	post.Type = "custom_llmbot" // This must be the only place we add this type for security.
	post.AddProp(LLMRequesterUserID, requesterUserID)
	// This tags that the post has unsafe links since they could have been generated by a prompt injection.
	// This will prevent the server from making OpenGraph requests and markdown images being rendered.
	post.AddProp(UnsafeLinksPostProp, "true")
}

func (s *Search) botDMNonResponse(botid string, userID string, post *model.Post) error {
	s.modifyPostForBot(botid, userID, post)

	if err := s.pluginAPI.DM(botid, userID, post); err != nil {
		return fmt.Errorf("failed to post DM: %w", err)
	}

	return nil
}
