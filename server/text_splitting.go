package main

import (
	"strings"
)

// splitPlaintextOnSentences splits a string into chunks of the given size.
// It intelligently splits on sentence boundaries assuming that ., !, and ? are sentence endings.
// It guarentees that each chunk is no larger than the given size, therefore it may split in the middle of a sentence.
// It limits the amount a chunk can be smaller than the given size to 3/4 of the given size. There for the number
// of chunks may be 25% greater than expected in worst case.
func splitPlaintextOnSentences(text string, chunksize int) []string {
	chunks := make([]string, 0, (len(text)/chunksize)+1)
	chunkSizeLowerBound := int(float64(chunksize) * 0.75)
	remainingText := text

	for len(remainingText) > chunksize {
		// Find the last sentence ending before the chunksize
		// If there are none, split on the chunksize
		sentenceEnd := strings.LastIndexAny(remainingText[:chunksize-1], ".!?")
		if sentenceEnd == -1 || sentenceEnd < chunkSizeLowerBound {
			sentenceEnd = chunksize - 1
		}

		chunks = append(chunks, strings.TrimSpace(remainingText[:sentenceEnd+1]))
		remainingText = strings.TrimSpace(remainingText[sentenceEnd+1:])
	}

	chunks = append(chunks, remainingText)

	return chunks
}
