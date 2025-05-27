// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

type Indexer struct {
	search    embeddings.EmbeddingSearch
	pluginAPI mmapi.Client
	bots      *bots.MMBots
	db        *sqlx.DB
}

func New(
	search embeddings.EmbeddingSearch,
	pluginAPI mmapi.Client,
	bots *bots.MMBots,
	db *sqlx.DB,
) *Indexer {
	return &Indexer{
		search:    search,
		pluginAPI: pluginAPI,
		bots:      bots,
		db:        db,
	}
}

// IndexPost indexes a post if it meets the criteria
func (s *Indexer) IndexPost(ctx context.Context, post *model.Post, channel *model.Channel) error {
	if !s.shouldIndexPost(post, channel) {
		return nil
	}

	if s.search == nil {
		return nil // Search not configured
	}

	// Create document
	doc := embeddings.PostDocument{
		PostID:    post.Id,
		CreateAt:  post.CreateAt,
		TeamID:    channel.TeamId,
		ChannelID: post.ChannelId,
		UserID:    post.UserId,
		Content:   post.Message,
	}

	// Store the document
	return s.search.Store(ctx, []embeddings.PostDocument{doc})
}

// DeletePost deletes a post from the index
func (s *Indexer) DeletePost(ctx context.Context, postID string) error {
	if s.search == nil {
		return nil // Search not configured
	}

	return s.search.Delete(ctx, []string{postID})
}

// StartReindexJob starts a post reindexing job
func (s *Indexer) StartReindexJob() (JobStatus, error) {
	// Check if search is initialized
	if s.search == nil {
		return JobStatus{}, fmt.Errorf("search functionality is not configured")
	}

	// Check if a job is already running
	var jobStatus JobStatus
	err := s.pluginAPI.KVGet(ReindexJobKey, &jobStatus)
	if err != nil && err.Error() != "not found" {
		return JobStatus{}, fmt.Errorf("failed to check job status: %w", err)
	}

	// If we have a valid job status and it's running, return conflict
	if jobStatus.Status == JobStatusRunning {
		return jobStatus, fmt.Errorf("job already running")
	}

	// Get an estimate of total posts for progress tracking
	var count int64
	dbErr := s.db.Get(&count, `SELECT COUNT(*) FROM Posts WHERE DeleteAt = 0 AND Message != '' AND Type = ''`)
	if dbErr != nil {
		s.pluginAPI.LogWarn("Failed to get post count for progress tracking", "error", dbErr)
		count = 0 // Continue with zero estimate
	}

	// Create initial job status
	newJobStatus := JobStatus{
		Status:    JobStatusRunning,
		StartedAt: time.Now(),
		TotalRows: count,
	}

	// Save initial job status
	err = s.pluginAPI.KVSet(ReindexJobKey, newJobStatus)
	if err != nil {
		return JobStatus{}, fmt.Errorf("failed to save job status: %w", err)
	}

	// Start the reindexing job in background
	go s.runReindexJob(&newJobStatus)

	return newJobStatus, nil
}

// GetJobStatus gets the status of the reindex job
func (s *Indexer) GetJobStatus() (JobStatus, error) {
	var jobStatus JobStatus
	err := s.pluginAPI.KVGet(ReindexJobKey, &jobStatus)
	if err != nil {
		return JobStatus{}, err
	}
	return jobStatus, nil
}

// CancelJob cancels a running reindex job
func (s *Indexer) CancelJob() (JobStatus, error) {
	var jobStatus JobStatus
	err := s.pluginAPI.KVGet(ReindexJobKey, &jobStatus)
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
	err = s.pluginAPI.KVSet(ReindexJobKey, jobStatus)
	if err != nil {
		return JobStatus{}, fmt.Errorf("failed to save job status: %w", err)
	}

	return jobStatus, nil
}

// shouldIndexPost returns whether a post should be indexed based on consistent criteria
func (s *Indexer) shouldIndexPost(post *model.Post, channel *model.Channel) bool {
	// Skip posts that don't have content
	if post.Message == "" {
		return false
	}

	// Skip posts from bots
	if s.bots.IsAnyBot(post.UserId) {
		return false
	}

	// Skip non-regular posts
	if post.Type != model.PostTypeDefault {
		return false
	}

	// Skip deleted posts
	if post.DeleteAt != 0 {
		return false
	}

	// Skip posts in DM channels with the bots
	if channel != nil && s.bots.GetBotForDMChannel(channel) != nil {
		return false
	}

	return true
}
