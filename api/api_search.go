// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-ai/agents"
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

func (a *API) handleRunSearch(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	bot := c.MustGet(ContextBotKey).(*agents.Bot)

	if !a.agents.IsSearchEnabled() {
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

	result, err := a.agents.HandleRunSearch(userID, bot, req.Query, req.TeamID, req.ChannelID, req.MaxResults)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (a *API) handleSearchQuery(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	bot := c.MustGet(ContextBotKey).(*agents.Bot)

	if !a.agents.IsSearchEnabled() {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("search functionality is not configured"))
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	response, err := a.agents.HandleSearchQuery(userID, bot, req.Query, req.TeamID, req.ChannelID, req.MaxResults)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, response)
}
