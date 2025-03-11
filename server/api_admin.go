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
		ID        string `db:"id"`
		Message   string `db:"message"`
		UserID    string `db:"userid"`
		ChannelID string `db:"channelid"`
		CreateAt  int64  `db:"createat"`
		TeamID    string `db:"teamid"`
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
			Channels.TeamId as teamid
		FROM Posts
		LEFT JOIN
			Channels
		ON
			Posts.ChannelId = Channels.Id
		WHERE
			Posts.DeleteAt = 0 AND
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

		// Convert to PostDocuments
		docs := make([]embeddings.PostDocument, 0, len(posts))
		for _, post := range posts {
			docs = append(docs, embeddings.PostDocument{
				Post: &model.Post{
					Id:        post.ID,
					ChannelId: post.ChannelID,
					UserId:    post.UserID,
					Message:   post.Message,
				},
				TeamID:    post.TeamID,
				ChannelID: post.ChannelID,
				UserID:    post.UserID,
				Content:   post.Message,
			})
		}

		// Store the batch
		if err := p.search.Store(c.Request.Context(), docs); err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to store documents: %w", err))
			return
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

func (p *Plugin) handleSearchPosts(c *gin.Context) {
	type SearchRequest struct {
		Query         string                 `json:"query"`
		Limit         int                    `json:"limit"`
		MinScore      float32                `json:"minScore"`
		TeamID        string                 `json:"teamId"`
		ChannelID     string                 `json:"channelId"`
		CreatedAfter  int64                  `json:"createdAfter"`
		CreatedBefore int64                  `json:"createdBefore"`
		Filter        map[string]interface{} `json:"filter"`
	}

	var req SearchRequest
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	opts := embeddings.SearchOptions{
		Limit:         req.Limit,
		MinScore:      req.MinScore,
		TeamID:        req.TeamID,
		ChannelID:     req.ChannelID,
		CreatedAfter:  req.CreatedAfter,
		CreatedBefore: req.CreatedBefore,
	}

	// Default limit if not specified
	if opts.Limit == 0 {
		opts.Limit = 10
	}

	results, err := p.search.Search(c.Request.Context(), req.Query, opts)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("search failed: %w", err))
		return
	}

	c.JSON(http.StatusOK, results)
}

func (p *Plugin) mattermostAdminAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	if !p.pluginAPI.User.HasPermissionTo(userID, model.PermissionManageSystem) {
		c.AbortWithError(http.StatusForbidden, errors.New("must be a system admin"))
		return
	}
}
