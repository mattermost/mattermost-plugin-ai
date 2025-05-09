// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package embeddings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkContent(t *testing.T) {
	t.Run("empty content", func(t *testing.T) {
		doc := PostDocument{
			PostID:  "post1",
			Content: "",
		}
		opts := DefaultChunkingOptions()

		chunks := ChunkPostDocument(doc, opts)
		require.Len(t, chunks, 1)
		assert.Equal(t, doc, chunks[0])
		assert.False(t, chunks[0].IsChunk)
	})

	t.Run("short content", func(t *testing.T) {
		doc := PostDocument{
			PostID:  "post1",
			Content: "This is a short message.",
		}
		opts := DefaultChunkingOptions()

		chunks := ChunkPostDocument(doc, opts)
		require.Len(t, chunks, 1)
		assert.Equal(t, doc.Content, chunks[0].Content)
		assert.False(t, chunks[0].IsChunk)
	})

	t.Run("sentences strategy", func(t *testing.T) {
		doc := PostDocument{
			PostID:  "post1",
			Content: "This is sentence one. This is sentence two! This is sentence three? This is sentence four.",
		}
		opts := ChunkingOptions{
			ChunkSize:        25,
			MinChunkSize:     0.75,
			ChunkingStrategy: "sentences",
		}

		chunks := ChunkPostDocument(doc, opts)
		require.Greater(t, len(chunks), 1, "Expected multiple chunks")

		for i, chunk := range chunks {
			t.Logf("Chunk %d: '%s'", i, chunk.Content)
			assert.True(t, chunk.IsChunk)
			assert.Equal(t, i, chunk.ChunkIndex)
			assert.Equal(t, len(chunks), chunk.TotalChunks)
			assert.Equal(t, chunk.PostID, "post1")
		}
	})

	t.Run("paragraphs strategy", func(t *testing.T) {
		doc := PostDocument{
			PostID:  "post1",
			Content: "Paragraph one.\nMore of paragraph one.\n\nParagraph two.\nMore of paragraph two.\n\nParagraph three.",
		}
		opts := ChunkingOptions{
			ChunkSize:        30,
			MinChunkSize:     0.75,
			ChunkingStrategy: "paragraphs",
		}

		chunks := ChunkPostDocument(doc, opts)
		require.Greater(t, len(chunks), 1, "Expected multiple chunks")

		for i, chunk := range chunks {
			t.Logf("Chunk %d: '%s'", i, chunk.Content)
			assert.True(t, chunk.IsChunk)
			assert.Equal(t, i, chunk.ChunkIndex)
			assert.Equal(t, len(chunks), chunk.TotalChunks)
		}
	})

	t.Run("fixed strategy", func(t *testing.T) {
		doc := PostDocument{
			PostID:  "post1",
			Content: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		}
		opts := ChunkingOptions{
			ChunkSize:        10,
			ChunkOverlap:     5,
			ChunkingStrategy: "fixed",
		}

		chunks := ChunkPostDocument(doc, opts)
		require.Greater(t, len(chunks), 1, "Expected multiple chunks")

		// Output for debugging
		for i, chunk := range chunks {
			t.Logf("Chunk %d: '%s'", i, chunk.Content)
		}

		// Verify that chunks cover the entire content
		combined := ""
		for _, chunk := range chunks {
			if len(combined) == 0 {
				combined = chunk.Content
			} else {
				// Only add characters not already present due to overlap
				overlap := min(len(combined), opts.ChunkOverlap)
				if overlap > 0 && len(chunk.Content) > overlap {
					combined += chunk.Content[overlap:]
				} else {
					combined += chunk.Content
				}
			}
		}

		// The combined chunks should contain all original characters
		for _, ch := range doc.Content {
			assert.Contains(t, combined, string(ch))
		}
	})
}
