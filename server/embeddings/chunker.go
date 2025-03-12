// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package embeddings

import (
	"strings"

	"github.com/tmc/langchaingo/textsplitter"
)

// ChunkPostDocument splits a PostDocument into multiple smaller PostDocuments
func ChunkPostDocument(doc PostDocument, opts ChunkingOptions) []PostDocument {
	// If content is empty, return the original document
	if strings.TrimSpace(doc.Content) == "" {
		return []PostDocument{doc}
	}

	// If chunk size is zero or negative, return the original document
	if opts.ChunkSize <= 0 {
		return []PostDocument{doc}
	}

	// Extract chunks based on the chosen strategy
	var textChunks []string
	var err error

	switch opts.ChunkingStrategy {
	case "paragraphs":
		// For paragraphs, use RecursiveCharacter with "\n\n" as first separator
		splitter := textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(opts.ChunkSize),
			textsplitter.WithChunkOverlap(opts.ChunkOverlap),
			textsplitter.WithSeparators([]string{"\n\n", "\n", " ", ""}),
		)
		textChunks, err = splitter.SplitText(doc.Content)
	case "fixed":
		// For fixed chunks, use RecursiveCharacter with just space and empty string as separators
		splitter := textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(opts.ChunkSize),
			textsplitter.WithChunkOverlap(opts.ChunkOverlap),
			textsplitter.WithSeparators([]string{" ", ""}),
		)
		textChunks, err = splitter.SplitText(doc.Content)
	default: // Default to sentences
		// For sentences, use RecursiveCharacter with sentence ending punctuation as separators
		splitter := textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(opts.ChunkSize),
			textsplitter.WithChunkOverlap(opts.ChunkOverlap),
			textsplitter.WithSeparators([]string{". ", "! ", "? ", "\n", " ", ""}),
		)
		textChunks, err = splitter.SplitText(doc.Content)
	}

	if err != nil {
		// In case of error, return the original document
		return []PostDocument{doc}
	}

	// If we only have one chunk and it's the same as the original, return the original
	if len(textChunks) == 1 && textChunks[0] == doc.Content {
		// Mark as not a chunk
		doc.IsChunk = false
		doc.TotalChunks = 1
		doc.ChunkIndex = 0
		return []PostDocument{doc}
	}

	// Create a document for each chunk
	result := make([]PostDocument, len(textChunks))
	for i, chunk := range textChunks {
		// Create a copy of the original document
		chunkDoc := doc

		chunkDoc.Content = chunk
		chunkDoc.IsChunk = true
		chunkDoc.ChunkIndex = i
		chunkDoc.TotalChunks = len(textChunks)
		chunkDoc.PostID = doc.PostID
		chunkDoc.CreateAt = doc.CreateAt

		result[i] = chunkDoc
	}

	return result
}
