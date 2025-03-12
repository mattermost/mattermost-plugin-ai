// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"net/http"

	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-ai/server/embeddings"
	"github.com/mattermost/mattermost/server/public/model"
)

const defaultBatchSize = 100

func (p *Plugin) handleReindexPosts(c *gin.Context) {
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

	// Check if search is initialized
	if p.search == nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("search functionality is not configured"))
		return
	}

	var posts []PostRecord
	lastCreateAt := int64(0)
	lastID := ""

	if err := p.search.Clear(c.Request.Context()); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to clear search index: %w", err))
		return
	}
	newSearch, err := p.initSearch()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to re-initialize search: %w", err))
		return
	}
	p.search = newSearch

	for {
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
		LEFT JOIN
			Channels
		ON
			Posts.ChannelId = Channels.Id
		WHERE
			Posts.DeleteAt = 0 AND
			Posts.Message != '' AND
			Posts.Type = '' AND
			(Posts.CreateAt, Posts.Id) > ($1, $2)
		ORDER BY
			Posts.CreateAt ASC, Posts.Id ASC
		LIMIT
			$3`

		err := p.db.Select(&posts, query, lastCreateAt, lastID, defaultBatchSize)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to fetch posts: %w", err))
			return
		}

		if len(posts) == 0 {
			break
		}

		// Convert to PostDocuments, applying the same filtering logic as indexPost
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
			if err := p.search.Store(c.Request.Context(), docs); err != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to store documents: %w", err))
				return
			}
		}

		// Update cursors for next batch
		lastPost := posts[len(posts)-1]
		lastCreateAt = lastPost.CreateAt
		lastID = lastPost.ID
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "complete",
	})
}

func (p *Plugin) mattermostAdminAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	if !p.pluginAPI.User.HasPermissionTo(userID, model.PermissionManageSystem) {
		c.AbortWithError(http.StatusForbidden, errors.New("must be a system admin"))
		return
	}
}
