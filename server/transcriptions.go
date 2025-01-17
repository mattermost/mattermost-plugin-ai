package main

import (
	"io"

	"github.com/mattermost/mattermost-plugin-ai/server/llm/subtitles"
)

type Transcriber interface {
	Transcribe(file io.Reader) (*subtitles.Subtitles, error)
}
