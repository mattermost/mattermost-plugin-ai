// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package postgres

import (
	"context"
	"fmt"
	"strconv"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/server/embeddings"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pgvector/pgvector-go"
)

type PGVector struct {
	db *sqlx.DB
}

type PGVectorConfig struct {
	Dimensions int `json:"dimensions"`
}

func NewPGVector(db *sqlx.DB, config PGVectorConfig) (*PGVector, error) {
	// Enable pgvector extension if not already enabled
	if _, err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		return nil, fmt.Errorf("failed to create vector extension: %w", err)
	}

	// Create the llm_posts_embeddings table if it doesn't exist
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS llm_posts_embeddings (
			post_id TEXT PRIMARY KEY REFERENCES Posts(Id) ON DELETE CASCADE,
			team_id TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			content TEXT NOT NULL,
			embedding vector(` + strconv.Itoa(config.Dimensions) + `),
			created_at BIGINT NOT NULL
		)`,
	); err != nil {
		return nil, fmt.Errorf("failed to create llm_posts_embeddings table: %w", err)
	}

	// Create index for similarity search using HNSW
	/*if _, err := db.Exec("CREATE INDEX IF NOT EXISTS llm_posts_embeddings_embedding_idx ON llm_posts_embeddings USING hnsw (embedding vector_l2_ops)"); err != nil {
		return nil, fmt.Errorf("failed to create vector index: %w", err)
	}*/

	return &PGVector{db: db}, nil
}

func (pv *PGVector) Store(ctx context.Context, docs []embeddings.PostDocument, embeddings [][]float32) error {
	for i, doc := range docs {
		_, err := pv.db.NamedExecContext(ctx, `
			INSERT INTO llm_posts_embeddings (post_id, team_id, channel_id, user_id, content, embedding, created_at)
			VALUES (:post_id, :team_id, :channel_id, :user_id, :content, :embedding, :created_at)
			ON CONFLICT (post_id) DO UPDATE SET
				content = EXCLUDED.content,
				embedding = EXCLUDED.embedding`,
			map[string]interface{}{
				"post_id":    doc.Post.Id,
				"team_id":    doc.TeamID,
				"channel_id": doc.ChannelID,
				"user_id":    doc.UserID,
				"content":    doc.Content,
				"embedding":  pgvector.NewVector(embeddings[i]),
				"created_at": doc.Post.CreateAt,
			},
		)
		if err != nil {
			return fmt.Errorf("failed to insert vector: %w", err)
		}
	}

	return nil
}

func (pv *PGVector) Search(ctx context.Context, embedding []float32, opts embeddings.SearchOptions) ([]embeddings.SearchResult, error) {
	queryBuilder := sq.Select("post_id", "team_id", "channel_id", "user_id", "content",
		"(embedding <-> ?) as similarity").
		From("llm_posts_embeddings").
		PlaceholderFormat(sq.Dollar)

	if opts.TeamID != "" {
		queryBuilder = queryBuilder.Where(sq.Eq{"team_id": opts.TeamID})
	}

	if opts.ChannelID != "" {
		queryBuilder = queryBuilder.Where(sq.Eq{"channel_id": opts.ChannelID})
	}

	if opts.CreatedAfter != 0 {
		queryBuilder = queryBuilder.Where(sq.Gt{"created_at": opts.CreatedAfter})
	}

	if opts.CreatedBefore != 0 {
		queryBuilder = queryBuilder.Where(sq.Lt{"created_at": opts.CreatedBefore})
	}

	queryBuilder = queryBuilder.OrderBy("similarity ASC")

	if opts.Limit > 0 && opts.Limit < 100000 {
		queryBuilder = queryBuilder.Limit(uint64(opts.Limit)) //nolint:gosec
	}

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build SQL: %w", err)
	}

	// Need to append the embedding to the args slice from the select
	args = append([]interface{}{pgvector.NewVector(embedding)}, args...)

	rows, err := pv.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %w", err)
	}
	defer rows.Close()

	var results []embeddings.SearchResult
	for rows.Next() {
		var postID, teamID, channelID, userID, content string
		var similarity float32

		if err := rows.Scan(&postID, &teamID, &channelID, &userID, &content, &similarity); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		score := 1 - similarity
		if score < 0 {
			score = 0
		}

		if score < opts.MinScore {
			continue
		}

		results = append(results, embeddings.SearchResult{
			Document: embeddings.PostDocument{
				Post: &model.Post{
					Id: postID,
				},
				TeamID:    teamID,
				ChannelID: channelID,
				UserID:    userID,
				Content:   content,
			},
			Score: score,
		})
	}

	return results, nil
}

func (pv *PGVector) Delete(ctx context.Context, postIDs []string) error {
	query, args, err := sq.
		Delete("llm_posts_embeddings").
		Where(sq.Eq{"post_id": postIDs}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to create query: %w", err)
	}
	_, err = pv.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete vectors: %w", err)
	}
	return nil
}

func (pv *PGVector) Clear(ctx context.Context) error {
	_, err := pv.db.ExecContext(ctx, "TRUNCATE TABLE llm_posts_embeddings")
	if err != nil {
		return fmt.Errorf("failed to clear vectors: %w", err)
	}
	return nil
}
