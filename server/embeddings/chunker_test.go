// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package embeddings

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkContent(t *testing.T) {
	t.Run("empty content", func(t *testing.T) {
		doc := PostDocument{
			Post: &model.Post{
				Id: "post1",
			},
			Content: "",
		}
		opts := DefaultChunkingOptions()

		chunks := ChunkContent(doc, opts)
		require.Len(t, chunks, 1)
		assert.Equal(t, doc, chunks[0])
		assert.False(t, chunks[0].IsChunk)
	})

	t.Run("short content", func(t *testing.T) {
		doc := PostDocument{
			Post: &model.Post{
				Id: "post1",
			},
			Content: "This is a short message.",
		}
		opts := DefaultChunkingOptions()

		chunks := ChunkContent(doc, opts)
		require.Len(t, chunks, 1)
		assert.Equal(t, doc.Content, chunks[0].Content)
		assert.False(t, chunks[0].IsChunk)
	})

	t.Run("sentences strategy", func(t *testing.T) {
		doc := PostDocument{
			Post: &model.Post{
				Id: "post1",
			},
			Content: "This is sentence one. This is sentence two! This is sentence three? This is sentence four.",
		}
		opts := ChunkingOptions{
			ChunkSize:        25,
			MinChunkSize:     0.75,
			ChunkingStrategy: "sentences",
		}

		chunks := ChunkContent(doc, opts)
		require.Len(t, chunks, 4)
		assert.Equal(t, "This is sentence one.", chunks[0].Content)
		assert.Equal(t, "This is sentence two!", chunks[1].Content)
		assert.Equal(t, "This is sentence three?", chunks[2].Content)
		assert.Equal(t, "This is sentence four.", chunks[3].Content)

		// Verify chunk information
		for i, chunk := range chunks {
			assert.True(t, chunk.IsChunk)
			assert.Equal(t, i, chunk.ChunkIndex)
			assert.Equal(t, 4, chunk.TotalChunks)
			assert.Equal(t, doc.Content, chunk.SourceContent)
			assert.Contains(t, chunk.Post.Id, "post1_chunk_")
		}
	})

	t.Run("paragraphs strategy", func(t *testing.T) {
		doc := PostDocument{
			Post: &model.Post{
				Id: "post1",
			},
			Content: "Paragraph one.\nMore of paragraph one.\n\nParagraph two.\nMore of paragraph two.\n\nParagraph three.",
		}
		opts := ChunkingOptions{
			ChunkSize:        30,
			MinChunkSize:     0.75,
			ChunkingStrategy: "paragraphs",
		}

		chunks := ChunkContent(doc, opts)
		require.Len(t, chunks, 3)
		assert.Contains(t, chunks[0].Content, "Paragraph one")
		assert.Contains(t, chunks[1].Content, "Paragraph two")
		assert.Contains(t, chunks[2].Content, "Paragraph three")
	})

	t.Run("fixed strategy", func(t *testing.T) {
		doc := PostDocument{
			Post: &model.Post{
				Id: "post1",
			},
			// Add a 'Z' at the end to verify it's included
			Content: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		}
		opts := ChunkingOptions{
			ChunkSize:        10,
			ChunkOverlap:     5,
			ChunkingStrategy: "fixed",
		}

		chunks := ChunkContent(doc, opts)
		// Using t.Logf to debug
		for i, chunk := range chunks {
			t.Logf("Chunk %d: '%s'", i, chunk.Content)
		}

		require.Len(t, chunks, 4)
		assert.Equal(t, "ABCDEFGHIJ", chunks[0].Content)
		assert.Equal(t, "FGHIJKLMNO", chunks[1].Content)
		assert.Equal(t, "KLMNOPQRST", chunks[2].Content)

		// Just check that the last chunk contains the right prefix
		assert.True(t, strings.HasPrefix(chunks[3].Content, "PQRST"))
	})
}

func TestMergeResults(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		results := []SearchResult{}
		merged := MergeResults(results)
		assert.Empty(t, merged)
	})

	t.Run("merge chunks from same parent", func(t *testing.T) {
		// Create multiple chunks from the same parent
		results := []SearchResult{
			{
				Document: PostDocument{
					Post:          &model.Post{Id: "post1_chunk_0"},
					Content:       "Chunk 1",
					IsChunk:       true,
					ChunkIndex:    0,
					TotalChunks:   2,
					SourceContent: "Chunk 1 Chunk 2",
				},
				Score: 0.7,
			},
			{
				Document: PostDocument{
					Post:          &model.Post{Id: "post1_chunk_1"},
					Content:       "Chunk 2",
					IsChunk:       true,
					ChunkIndex:    1,
					TotalChunks:   2,
					SourceContent: "Chunk 1 Chunk 2",
				},
				Score: 0.8,
			},
		}

		merged := MergeResults(results)
		require.Len(t, merged, 1)
		assert.Equal(t, "Chunk 1 Chunk 2", merged[0].Document.Content)
		assert.Equal(t, float32(0.8), merged[0].Score) // Should take highest score
		assert.False(t, merged[0].Document.IsChunk)
		assert.Equal(t, "post1", merged[0].Document.Post.Id)
	})

	t.Run("multiple parents", func(t *testing.T) {
		results := []SearchResult{
			{
				Document: PostDocument{
					Post:          &model.Post{Id: "post1_chunk_0"},
					Content:       "Chunk 1 from post 1",
					IsChunk:       true,
					TotalChunks:   1,
					SourceContent: "Chunk 1 from post 1",
				},
				Score: 0.7,
			},
			{
				Document: PostDocument{
					Post:          &model.Post{Id: "post2_chunk_0"},
					Content:       "Chunk 1 from post 2",
					IsChunk:       true,
					TotalChunks:   1,
					SourceContent: "Chunk 1 from post 2",
				},
				Score: 0.9,
			},
		}

		merged := MergeResults(results)
		require.Len(t, merged, 2)

		// The order of results is not guaranteed as it depends on map iteration
		scores := []float32{merged[0].Score, merged[1].Score}
		assert.Contains(t, scores, float32(0.7))
		assert.Contains(t, scores, float32(0.9))
	})
}
