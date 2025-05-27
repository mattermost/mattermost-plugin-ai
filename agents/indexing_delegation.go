// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/indexing"
	"github.com/mattermost/mattermost/server/public/model"
)

// HandleReindexPosts delegates to the indexing service
func (p *AgentsService) HandleReindexPosts() (indexing.JobStatus, error) {
	if p.indexingService == nil {
		return indexing.JobStatus{}, fmt.Errorf("indexing functionality is not configured")
	}
	return p.indexingService.StartReindexJob()
}

// GetJobStatus delegates to the indexing service
func (p *AgentsService) GetJobStatus() (indexing.JobStatus, error) {
	if p.indexingService == nil {
		return indexing.JobStatus{}, fmt.Errorf("indexing functionality is not configured")
	}
	return p.indexingService.GetJobStatus()
}

// CancelJob delegates to the indexing service
func (p *AgentsService) CancelJob() (indexing.JobStatus, error) {
	if p.indexingService == nil {
		return indexing.JobStatus{}, fmt.Errorf("indexing functionality is not configured")
	}
	return p.indexingService.CancelJob()
}

// ShouldIndexPost returns whether a post should be indexed based on consistent criteria
func (p *AgentsService) ShouldIndexPost(post *model.Post, channel *model.Channel) bool {
	// Skip posts that don't have content
	if post.Message == "" {
		return false
	}

	// Skip posts from bots
	if p.bots.IsAnyBot(post.UserId) {
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
	if channel != nil && p.bots.GetBotForDMChannel(channel) != nil {
		return false
	}

	return true
}
