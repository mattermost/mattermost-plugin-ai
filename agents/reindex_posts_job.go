// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
	JobStatusCanceled  = "canceled"

	defaultBatchSize = 100

	// KV store keys
	ReindexJobKey = "reindex_job_status"
)

// PostRecord represents a post record from the database
type PostRecord struct {
	ID       string `db:"id"`
	Message  string `db:"message"`
	UserID   string `db:"userid"`
	CreateAt int64  `db:"createat"`
	TeamID   string `db:"teamid"`

	ChannelID   string `db:"channelid"`
	ChannelName string `db:"channelname"`
	ChannelType string `db:"channeltype"`
}

// JobStatus represents the status of a reindex job
type JobStatus struct {
	Status        string    `json:"status"`
	Error         string    `json:"error,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
	ProcessedRows int64     `json:"processed_rows"`
	TotalRows     int64     `json:"total_rows"`
}

// runReindexJob runs the reindexing process
func (p *AgentsService) runReindexJob(jobStatus *JobStatus) {
	defer func() {
		if r := recover(); r != nil {
			p.pluginAPI.Log.Error("Reindex job panicked", "panic", r)
			jobStatus.Status = JobStatusFailed
			jobStatus.Error = fmt.Sprintf("Job panicked: %v", r)
			jobStatus.CompletedAt = time.Now()
			p.saveJobStatus(jobStatus)
		}
	}()

	ctx := context.Background()

	// Clear the existing index
	if err := p.search.Clear(ctx); err != nil {
		jobStatus.Status = JobStatusFailed
		jobStatus.Error = fmt.Sprintf("Failed to clear search index: %s", err)
		jobStatus.CompletedAt = time.Now()
		p.saveJobStatus(jobStatus)
		return
	}

	var posts []PostRecord
	lastCreateAt := int64(0)
	lastID := ""
	processedCount := int64(0)
	lastSavedCount := int64(0) // Track when we last saved status

	for {
		// Check if the job was canceled
		var data []byte
		_ = p.pluginAPI.KV.Get(ReindexJobKey, &data)
		if data != nil {
			var currentStatus JobStatus
			if err := json.Unmarshal(data, &currentStatus); err == nil {
				if currentStatus.Status == JobStatusCanceled {
					p.pluginAPI.Log.Info("Reindex job was canceled")
					return
				}
			}
		}

		// Run a batch of indexing
		query := `SELECT
			Posts.Id as id,
			Posts.Message as message,
			Posts.UserId as userid,
			Posts.ChannelId as channelid,
			Posts.CreateAt as createat,
			Channels.TeamId as teamid,
			Channels.Name as channelname,
			Channels.Type as channeltype
		FROM Posts
		LEFT JOIN Channels ON Posts.ChannelId = Channels.Id
		WHERE Posts.DeleteAt = 0 AND Posts.Message != '' AND Posts.Type = ''
			AND (Posts.CreateAt, Posts.Id) > ($1, $2)
		ORDER BY Posts.CreateAt ASC, Posts.Id ASC
		LIMIT $3`

		err := p.db.Select(&posts, query, lastCreateAt, lastID, defaultBatchSize)
		if err != nil {
			jobStatus.Status = JobStatusFailed
			jobStatus.Error = fmt.Sprintf("Failed to fetch posts: %s", err)
			jobStatus.CompletedAt = time.Now()
			p.saveJobStatus(jobStatus)
			return
		}

		if len(posts) == 0 {
			break
		}

		// Process batch and index posts
		docs := make([]embeddings.PostDocument, 0, len(posts))
		for _, post := range posts {
			modelPost := &model.Post{
				Id:        post.ID,
				ChannelId: post.ChannelID,
				UserId:    post.UserID,
				Message:   post.Message,
				Type:      model.PostTypeDefault, // We already filter out non-default post types in the SQL query
				DeleteAt:  0,                     // We already filter deleted posts in the SQL query
			}

			// Create a minimal channel object with necessary fields for filtering
			channel := &model.Channel{
				Id:     post.ChannelID,
				TeamId: post.TeamID,
				Name:   post.ChannelName,
				Type:   model.ChannelType(post.ChannelType),
			}

			// Apply same indexing rules as indexPost
			if !p.ShouldIndexPost(modelPost, channel) {
				continue
			}

			docs = append(docs, embeddings.PostDocument{
				PostID:    modelPost.Id,
				CreateAt:  modelPost.CreateAt,
				TeamID:    post.TeamID,
				ChannelID: post.ChannelID,
				UserID:    post.UserID,
				Content:   post.Message,
			})
		}

		// Store the batch
		if len(docs) > 0 {
			if err := p.search.Store(ctx, docs); err != nil {
				jobStatus.Status = JobStatusFailed
				jobStatus.Error = fmt.Sprintf("Failed to store documents: %s", err)
				jobStatus.CompletedAt = time.Now()
				p.saveJobStatus(jobStatus)
				return
			}
		}

		// Update progress
		processedCount += int64(len(posts))
		jobStatus.ProcessedRows = processedCount

		// Update cursors for next batch
		lastPost := posts[len(posts)-1]
		lastCreateAt = lastPost.CreateAt
		lastID = lastPost.ID

		// Save progress every 500 additional processed records
		if processedCount >= lastSavedCount+500 {
			p.saveJobStatus(jobStatus)
			p.pluginAPI.Log.Info("Reindexing progress",
				"processed", processedCount,
				"estimated_total", jobStatus.TotalRows)
			lastSavedCount = processedCount
		}
	}

	// Completed successfully
	jobStatus.Status = JobStatusCompleted
	jobStatus.CompletedAt = time.Now()
	p.saveJobStatus(jobStatus)

	p.pluginAPI.Log.Info("Reindexing completed", "processed_posts", processedCount)
}

// saveJobStatus saves the job status to KV store
func (p *AgentsService) saveJobStatus(status *JobStatus) {
	data, _ := json.Marshal(status)
	if _, err := p.pluginAPI.KV.Set(ReindexJobKey, data); err != nil {
		p.pluginAPI.Log.Error("Failed to save job status", "error", err)
	}
}
