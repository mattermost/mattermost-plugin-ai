// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"io"

	"github.com/mattermost/mattermost-plugin-ai/server/llm/subtitles"
)

type Transcriber interface {
	Transcribe(file io.Reader) (*subtitles.Subtitles, error)
}
