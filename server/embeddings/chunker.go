// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package embeddings

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// generateChunkID creates a deterministic ID for a chunk
func generateChunkID(parentID string, chunkIndex int) string {
	return fmt.Sprintf("%s_chunk_%d", parentID, chunkIndex)
}

// ChunkContent is the default implementation for chunking documents
func ChunkContent(doc PostDocument, opts ChunkingOptions) []PostDocument {
	// If content is empty, return the original document
	if strings.TrimSpace(doc.Content) == "" {
		return []PostDocument{doc}
	}

	// If chunk size is zero or negative, return the original document
	if opts.ChunkSize <= 0 {
		return []PostDocument{doc}
	}

	// Save the original content
	originalContent := doc.Content

	// Extract chunks based on the chosen strategy
	var textChunks []string
	switch opts.ChunkingStrategy {
	case "paragraphs":
		textChunks = splitOnParagraphs(doc.Content, opts.ChunkSize, opts.MinChunkSize)
	case "fixed":
		textChunks = splitFixed(doc.Content, opts.ChunkSize, opts.ChunkOverlap)
	default: // Default to sentences
		textChunks = splitOnSentences(doc.Content, opts.ChunkSize, opts.MinChunkSize)
	}

	// If we only have one chunk and it's the same as the original, return the original
	if len(textChunks) == 1 && textChunks[0] == doc.Content {
		// Mark as not a chunk
		doc.IsChunk = false
		doc.TotalChunks = 1
		doc.ChunkIndex = 0
		return []PostDocument{doc}
	}

	// If the post doesn't have an ID (rare case), generate one
	parentID := doc.Post.Id
	if parentID == "" {
		hasher := sha256.New()
		hasher.Write([]byte(originalContent))
		parentID = hex.EncodeToString(hasher.Sum(nil))
	}

	// Create a document for each chunk
	result := make([]PostDocument, len(textChunks))
	for i, chunk := range textChunks {
		// Create a copy of the original document
		chunkDoc := doc

		// Update the content to the chunk
		chunkDoc.Content = chunk

		// Set chunk information
		chunkDoc.IsChunk = true
		chunkDoc.ChunkIndex = i
		chunkDoc.TotalChunks = len(textChunks)
		chunkDoc.SourceContent = originalContent

		// For chunk documents, use the chunk ID as the Post.Id
		chunkDoc.Post = &model.Post{
			Id:       generateChunkID(parentID, i),
			CreateAt: doc.Post.CreateAt,
		}

		result[i] = chunkDoc
	}

	return result
}

// MergeResults groups search results by parent document and ranks them
func MergeResults(results []SearchResult) []SearchResult {
	if len(results) == 0 {
		return results
	}

	// Group results by parent ID
	parentGroups := make(map[string][]SearchResult)
	for _, result := range results {
		// Extract the parent ID from the chunk ID or use the post ID if it's not a chunk
		parentID := result.Document.Post.Id
		if result.Document.IsChunk {
			// Extract parent ID from chunk ID format "parentID_chunk_N"
			parts := strings.Split(parentID, "_chunk_")
			if len(parts) == 2 {
				parentID = parts[0]
			}
		}
		parentGroups[parentID] = append(parentGroups[parentID], result)
	}

	// For each parent, keep only the highest scoring chunk
	mergedResults := make([]SearchResult, 0, len(parentGroups))
	for _, group := range parentGroups {
		// Find the highest scoring result
		bestResult := group[0]
		for _, result := range group[1:] {
			if result.Score > bestResult.Score {
				bestResult = result
			}
		}

		// If this is a chunk and we have the source content, create a combined result
		if bestResult.Document.IsChunk && bestResult.Document.SourceContent != "" {
			// Create a copy with the full content but the chunk's score
			fullDoc := bestResult.Document
			fullDoc.Content = bestResult.Document.SourceContent
			fullDoc.IsChunk = false

			// Restore original post ID
			parts := strings.Split(fullDoc.Post.Id, "_chunk_")
			if len(parts) == 2 {
				fullDoc.Post.Id = parts[0]
			}

			mergedResults = append(mergedResults, SearchResult{
				Document: fullDoc,
				Score:    bestResult.Score,
			})
		} else {
			mergedResults = append(mergedResults, bestResult)
		}
	}

	return mergedResults
}

// splitOnSentences splits text on sentence boundaries
func splitOnSentences(text string, chunkSize int, minChunkSize float64) []string {
	chunks := make([]string, 0, (len(text)/chunkSize)+1)
	chunkSizeLowerBound := int(float64(chunkSize) * minChunkSize)
	remainingText := text

	for len(remainingText) > chunkSize {
		// Find the last sentence ending before the chunksize
		// If there are none, split on the chunksize
		sentenceEnd := strings.LastIndexAny(remainingText[:chunkSize-1], ".!?")
		if sentenceEnd == -1 || sentenceEnd < chunkSizeLowerBound {
			sentenceEnd = chunkSize - 1
		} else {
			// Include the punctuation
			sentenceEnd++
		}

		chunks = append(chunks, strings.TrimSpace(remainingText[:sentenceEnd]))
		remainingText = strings.TrimSpace(remainingText[sentenceEnd:])
	}

	if len(remainingText) > 0 {
		chunks = append(chunks, remainingText)
	}

	return chunks
}

// splitOnParagraphs splits text on paragraph boundaries
func splitOnParagraphs(text string, chunkSize int, minChunkSize float64) []string {
	chunks := make([]string, 0, (len(text)/chunkSize)+1)
	chunkSizeLowerBound := int(float64(chunkSize) * minChunkSize)
	paragraphs := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")

	currentChunk := ""

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// If adding this paragraph would exceed the chunk size
		if len(currentChunk) > 0 && len(currentChunk)+len(para)+1 > chunkSize {
			// If the current chunk is too small and we can combine
			if len(currentChunk) < chunkSizeLowerBound && len(currentChunk)+len(para) < chunkSize*2 {
				currentChunk += "\n\n" + para
				chunks = append(chunks, currentChunk)
				currentChunk = ""
			} else {
				// Add the current chunk and start a new one
				chunks = append(chunks, currentChunk)
				currentChunk = para
			}
		} else {
			// Add paragraph to current chunk
			if len(currentChunk) > 0 {
				currentChunk += "\n\n" + para
			} else {
				currentChunk = para
			}
		}
	}

	// Add the final chunk if not empty
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// splitFixed splits text into fixed-size chunks with optional overlap
func splitFixed(text string, chunkSize int, overlap int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}

	chunks := make([]string, 0, (len(text)/(chunkSize-overlap))+1)

	for i := 0; i < len(text); i += (chunkSize - overlap) {
		end := i + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[i:end])

		// If we're near the end and the next chunk would be small, just stop
		if end >= len(text)-overlap {
			break
		}
	}

	return chunks
}
