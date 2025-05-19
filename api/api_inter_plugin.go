// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SimpleCompletionRequest struct {
	SystemPrompt    string         `json:"systemPrompt"`
	UserPrompt      string         `json:"userPrompt"`
	BotUsername     string         `json:"botUsername"`
	RequesterUserID string         `json:"requesterUserID"`
	Parameters      map[string]any `json:"parameters"`
}

func (a *API) handleInterPluginSimpleCompletion(c *gin.Context) {
	var req SimpleCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	userID := req.RequesterUserID
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "requesterUserID is required"})
		return
	}

	response, err := a.agents.HandleInterPluginSimpleCompletion(req.SystemPrompt, req.UserPrompt, req.BotUsername, userID, req.Parameters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}
