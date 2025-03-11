// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package embeddings

import (
	"context"
)

// CompositeSearch implements EmbeddingSearch using separate vector store and embedding provider
type CompositeSearch struct {
	store    VectorStore
	provider EmbeddingProvider
}

func NewCompositeSearch(store VectorStore, provider EmbeddingProvider) *CompositeSearch {
	return &CompositeSearch{
		store:    store,
		provider: provider,
	}
}

func (c *CompositeSearch) Store(ctx context.Context, docs []PostDocument) error {
	texts := make([]string, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Content
	}

	embeddings, err := c.provider.BatchCreateEmbeddings(ctx, texts)
	if err != nil {
		return err
	}
	return c.store.Store(ctx, docs, embeddings)
}

func (c *CompositeSearch) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	embedding, err := c.provider.CreateEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}
	return c.store.Search(ctx, embedding, opts)
}

func (c *CompositeSearch) Delete(ctx context.Context, postIDs []string) error {
	return c.store.Delete(ctx, postIDs)
}

func (c *CompositeSearch) Clear(ctx context.Context) error {
	return c.store.Clear(ctx)
}
