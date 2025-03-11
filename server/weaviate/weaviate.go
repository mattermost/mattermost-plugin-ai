// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package weaviate

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/embeddings"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

const (
	className = "MattermostPost"
)

// Search implements EmbeddingSearch using Weaviate as both vector store and embedding provider
type Search struct {
	client *weaviate.Client
}

/*func NewWeaviateSearch(config embeddings.ServiceConfig) (*WeaviateSearch, error) {
	cfg := weaviate.Config{
		Host:   config.Endpoint,
		Scheme: "http",
	}
	if authToken, ok := config.Parameters["apiKey"].(string); ok && authToken != "" {
		cfg.Headers = map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", authToken),
		}
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Weaviate client: %w", err)
	}

	return &WeaviateSearch{
		client: client,
	}, nil
}*/

func (w *Search) Initialize(ctx context.Context) (bool, error) {
	classExists, err := w.client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check class existence: %w", err)
	}

	if !classExists {
		class := &models.Class{
			Class: className,
			Properties: []*models.Property{
				{Name: "postID", DataType: []string{"string"}},
				{Name: "teamID", DataType: []string{"string"}},
				{Name: "channelID", DataType: []string{"string"}},
				{Name: "channelName", DataType: []string{"string"}},
				{Name: "userName", DataType: []string{"string"}},
				{Name: "content", DataType: []string{"text"}},
				{Name: "createAt", DataType: []string{"int"}},
			},
			Vectorizer: "text2vec-ollama",
			ModuleConfig: map[string]interface{}{
				"text2vec-ollama": map[string]interface{}{
					"apiEndpoint": "http://172.20.224.1:11434",
					"model":       "nomic-embed-text",
				},
				"generative-ollama": map[string]interface{}{
					"apiEndpoint": "http://172.20.224.1:11434",
					"model":       "llama3.2",
				},
			},
		}

		err = w.client.Schema().ClassCreator().WithClass(class).Do(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to create class: %w", err)
		}
		return true, nil
	}

	return false, nil
}

func (w *Search) Store(ctx context.Context, docs []embeddings.PostDocument) error {
	batcher := w.client.Batch().ObjectsBatcher()

	objects := make([]*models.Object, len(docs))
	for i, doc := range docs {
		properties := map[string]interface{}{
			"postID":      doc.Post.Id,
			"teamID":      doc.TeamID,
			"channelID":   doc.Post.ChannelId,
			"channelName": doc.ChannelID,
			"userName":    doc.UserID,
			"content":     doc.Content,
			"createAt":    doc.Post.CreateAt,
		}

		objects[i] = &models.Object{
			Class:      className,
			Properties: properties,
		}
	}

	batcher = batcher.WithObjects(objects...)

	resp, err := batcher.Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to store documents: %w", err)
	}

	if len(resp) != len(docs) {
		return fmt.Errorf("expected %d responses, got %d", len(docs), len(resp))
	}

	return nil
}

func (w *Search) Search(ctx context.Context, query string, opts embeddings.SearchOptions) ([]embeddings.SearchResult, error) {
	var filterBuilders []*filters.WhereBuilder

	if opts.TeamID != "" {
		teamFilter := filters.Where().
			WithPath([]string{"teamID"}).
			WithOperator(filters.Equal).
			WithValueString(opts.TeamID)
		filterBuilders = append(filterBuilders, teamFilter)
	}

	if opts.ChannelID != "" {
		channelFilter := filters.Where().
			WithPath([]string{"channelID"}).
			WithOperator(filters.Equal).
			WithValueString(opts.ChannelID)
		filterBuilders = append(filterBuilders, channelFilter)
	}

	if opts.CreatedAfter != 0 || opts.CreatedBefore != 0 {
		dateFilter := filters.Where()
		if opts.CreatedAfter != 0 {
			dateFilter = dateFilter.
				WithPath([]string{"createAt"}).
				WithOperator(filters.GreaterThanEqual).
				WithValueInt(opts.CreatedAfter)
		}
		if opts.CreatedBefore != 0 {
			dateFilter = dateFilter.
				WithPath([]string{"createAt"}).
				WithOperator(filters.LessThanEqual).
				WithValueInt(opts.CreatedBefore)
		}
		filterBuilders = append(filterBuilders, dateFilter)
	}

	var whereFilter *filters.WhereBuilder
	if len(filterBuilders) > 0 {
		if len(filterBuilders) == 1 {
			whereFilter = filterBuilders[0]
		} else {
			whereFilter = filters.Where().
				WithOperator(filters.And).
				WithOperands(filterBuilders)
		}
	}

	fields := []graphql.Field{
		{Name: "postID"},
		{Name: "teamID"},
		{Name: "channelID"},
		{Name: "channelName"},
		{Name: "userName"},
		{Name: "content"},
		{Name: "createAt"},
		{Name: "_additional", Fields: []graphql.Field{{Name: "certainty"}}},
	}

	limit := 10
	if opts.Limit > 0 {
		limit = opts.Limit
	}

	result, err := w.client.GraphQL().Get().
		WithClassName(className).
		WithFields(fields...).
		WithNearText(w.client.GraphQL().NearTextArgBuilder().
			WithConcepts([]string{query})).
		WithWhere(whereFilter).
		WithLimit(limit).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to perform search: %w", err)
	}

	if len(result.Errors) > 0 {
		var errMsgs []string
		for _, e := range result.Errors {
			errMsgs = append(errMsgs, e.Message)
		}
		return nil, fmt.Errorf("GraphQL errors: %v", errMsgs)
	}

	var searchResults []embeddings.SearchResult
	getter := result.Data["Get"].(map[string]interface{})
	items := getter[className].([]interface{})
	for _, rawItem := range items {
		item := rawItem.(map[string]interface{})
		props := item
		doc := embeddings.PostDocument{
			Post: &model.Post{
				Id:        props["postID"].(string),
				ChannelId: props["channelID"].(string),
				CreateAt:  int64(props["createAt"].(float64)),
			},
			TeamID:    props["teamID"].(string),
			ChannelID: props["channelName"].(string),
			UserID:    props["userName"].(string),
			Content:   props["content"].(string),
		}

		searchResults = append(searchResults, embeddings.SearchResult{
			Document: doc,
			Score:    float32(item["_additional"].(map[string]interface{})["certainty"].(float64)),
		})
	}

	return searchResults, nil
}

func (w *Search) Delete(ctx context.Context, postIDs []string) error {
	deleter := w.client.Batch().ObjectsBatchDeleter().
		WithClassName(className).
		WithWhere(filters.Where().
			WithPath([]string{"postID"}).
			WithOperator(filters.ContainsAny).
			WithValueString(postIDs...))

	_, err := deleter.Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	return nil
}
