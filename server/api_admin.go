// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"net/http"
	"time"

	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost/server/public/model"
)

// handleReindexPosts starts a background job to reindex all posts
func (p *Plugin) handleReindexPosts(c *gin.Context) {
	// Check if search is initialized
	if p.search == nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("search functionality is not configured"))
		return
	}

	// Check if a job is already running
	data, appErr := p.API.KVGet(ReindexJobKey)
	if appErr != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to check job status: %w", appErr))
		return
	}

	if data != nil {
		var jobStatus JobStatus
		if err := json.Unmarshal(data, &jobStatus); err == nil {
			if jobStatus.Status == JobStatusRunning {
				c.JSON(http.StatusConflict, jobStatus)
				return
			}
		}
	}

	// Get an estimate of total posts for progress tracking
	var count int64
	err := p.db.Get(&count, `SELECT COUNT(*) FROM Posts WHERE DeleteAt = 0 AND Message != '' AND Type = ''`)
	if err != nil {
		p.pluginAPI.Log.Warn("Failed to get post count for progress tracking", "error", err)
		count = 0 // Continue with zero estimate
	}

	// Create initial job status
	jobStatus := &JobStatus{
		Status:    JobStatusRunning,
		StartedAt: time.Now(),
		TotalRows: count,
	}

	// Save initial job status
	jobData, _ := json.Marshal(jobStatus)
	if appErr := p.API.KVSet(ReindexJobKey, jobData); appErr != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to save job status: %w", appErr))
		return
	}

	// Start the reindexing job in background
	go p.runReindexJob(jobStatus)

	c.JSON(http.StatusOK, jobStatus)
}

// handleGetJobStatus gets the status of the reindex job
func (p *Plugin) handleGetJobStatus(c *gin.Context) {
	data, appErr := p.API.KVGet(ReindexJobKey)
	if appErr != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get job status: %w", appErr))
		return
	}

	if data == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "no_job",
		})
		return
	}

	var jobStatus JobStatus
	if err := json.Unmarshal(data, &jobStatus); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to parse job status: %w", err))
		return
	}

	c.JSON(http.StatusOK, jobStatus)
}

// handleCancelJob cancels a running reindex job
func (p *Plugin) handleCancelJob(c *gin.Context) {
	data, appErr := p.API.KVGet(ReindexJobKey)
	if appErr != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get job status: %w", appErr))
		return
	}

	if data == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "no_job",
		})
		return
	}

	var jobStatus JobStatus
	if err := json.Unmarshal(data, &jobStatus); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to parse job status: %w", err))
		return
	}

	if jobStatus.Status != JobStatusRunning {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "not_running",
		})
		return
	}

	// Update status to canceled
	jobStatus.Status = JobStatusCanceled
	jobStatus.CompletedAt = time.Now()

	// Save updated status
	data, _ = json.Marshal(jobStatus)
	if appErr := p.API.KVSet(ReindexJobKey, data); appErr != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to save job status: %w", appErr))
		return
	}

	c.JSON(http.StatusOK, jobStatus)
}

func (p *Plugin) mattermostAdminAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	if !p.pluginAPI.User.HasPermissionTo(userID, model.PermissionManageSystem) {
		c.AbortWithError(http.StatusForbidden, errors.New("must be a system admin"))
		return
	}
}
