package main

import (
	"fmt"
	"net/http"

	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) mattermostAdminAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	if !p.pluginAPI.User.HasPermissionTo(userID, model.PermissionManageSystem) {
		c.AbortWithError(http.StatusForbidden, errors.New("must be a system admin"))
		return
	}
}

type PostEmbedding struct {
	PostId    string
	Embedding []float32
}

func (p *Plugin) handleReindex(c *gin.Context) {
	// Not production ready
	var everyPost []struct {
		Id      string
		Message string
	}

	if err := p.doQuery(&everyPost, p.builder.
		Select("id, message").
		From("Posts").
		Where(sq.Eq{"Type": ""}),
	); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to query posts: %w", err))
		return
	}

	embeddings := make([]PostEmbedding, 0, len(everyPost))
	embeddingMaker := p.getEmbeddingsModel()
	for _, post := range everyPost {
		embedding, err := embeddingMaker.Embed(post.Message)
		if err != nil {
			p.API.LogDebug("Unable to embed on reindex", "message", post.Message, "postid", post.Id, "error", err)
			continue
		}
		if len(embedding) == 0 {
			p.API.LogDebug("Zero length embedding on reindex", "message", post.Message, "postid", post.Id)
			continue
		}
		embeddings = append(embeddings, PostEmbedding{
			PostId:    post.Id,
			Embedding: embedding,
		})
	}

	for _, embedding := range embeddings {
		p.saveEmbedding(embedding.PostId, embedding.Embedding)
	}

	c.Status(http.StatusOK)
}
