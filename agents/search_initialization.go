// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/embeddings"
	"github.com/mattermost/mattermost-plugin-ai/openai"
	"github.com/mattermost/mattermost-plugin-ai/postgres"
)

// NewVectorStore creates a new vector store based on the provided configuration
func (p *AgentsService) newVectorStore(config embeddings.UpstreamConfig, dimensions int) (embeddings.VectorStore, error) {
	switch config.Type { //nolint:gocritic
	case embeddings.VectorStoreTypePGVector:
		pgVectorConfig := postgres.PGVectorConfig{
			Dimensions: dimensions,
		}
		if err := json.Unmarshal(config.Parameters, &pgVectorConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pgvector config: %w", err)
		}
		return postgres.NewPGVector(p.db, pgVectorConfig)
	}

	return nil, fmt.Errorf("unsupported vector store type: %s", config.Type)
}

// NewEmbeddingProvider creates a new embedding provider based on the provided configuration
func (p *AgentsService) newEmbeddingProvider(config embeddings.UpstreamConfig) (embeddings.EmbeddingProvider, error) {
	switch config.Type {
	case embeddings.ProviderTypeOpenAICompatible:
		compatibleConfig := openai.Config{}
		if err := json.Unmarshal(config.Parameters, &compatibleConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal OpenAI-compatible config: %w", err)
		}
		return openai.NewCompatibleEmbeddings(compatibleConfig, p.llmUpstreamHTTPClient), nil
	case embeddings.ProviderTypeOpenAI:
		var openaiConfig openai.Config
		if err := json.Unmarshal(config.Parameters, &openaiConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal OpenAI config: %w", err)
		}
		return openai.NewCompatibleEmbeddings(openaiConfig, p.llmUpstreamHTTPClient), nil
	}

	return nil, fmt.Errorf("unsupported embedding provider type: %s", config.Type)
}

func (p *AgentsService) initSearch() (embeddings.EmbeddingSearch, error) {
	cfg := p.getConfiguration()

	if cfg.EmbeddingSearchConfig.Type == "" {
		return nil, fmt.Errorf("search is disabled")
	}

	if !p.licenseChecker.IsBasicsLicensed() {
		return nil, fmt.Errorf("search is unavailable without a valid license")
	}

	switch cfg.EmbeddingSearchConfig.Type { //nolint:gocritic
	case embeddings.SearchTypeComposite:
		vector, err := p.newVectorStore(cfg.EmbeddingSearchConfig.VectorStore, cfg.EmbeddingSearchConfig.Dimensions)
		if err != nil {
			return nil, err
		}
		embeddor, err := p.newEmbeddingProvider(cfg.EmbeddingSearchConfig.EmbeddingProvider)
		if err != nil {
			return nil, err
		}

		// Check if we have specific chunking options configured
		chunkingOpts := cfg.EmbeddingSearchConfig.ChunkingOptions
		if chunkingOpts.ChunkSize == 0 {
			chunkingOpts = embeddings.DefaultChunkingOptions()
		}

		return embeddings.NewCompositeSearch(vector, embeddor, chunkingOpts), nil
	}

	return nil, fmt.Errorf("unsupported search type: %s", cfg.EmbeddingSearchConfig.Type)
}
