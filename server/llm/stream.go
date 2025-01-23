// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llm

type TextStreamResult struct {
	Stream <-chan string
	Err    <-chan error
}

func NewStreamFromString(text string) *TextStreamResult {
	output := make(chan string)
	err := make(chan error)

	go func() {
		output <- text
		close(output)
		close(err)
	}()

	return &TextStreamResult{
		Stream: output,
		Err:    err,
	}
}

func (t *TextStreamResult) ReadAll() string {
	result := ""
	for next := range t.Stream {
		result += next
	}

	return result
}
