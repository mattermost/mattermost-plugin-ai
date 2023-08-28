package subtitles

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/asticode/go-astisub"
)

type Subtitles struct {
	storage *astisub.Subtitles
}

func NewSubtitlesFromVTT(webvtt io.Reader) (*Subtitles, error) {
	storage, err := astisub.ReadFromWebVTT(webvtt)
	if err != nil {
		return nil, err
	}
	return &Subtitles{storage: storage}, nil
}

func (s *Subtitles) WebVTT() io.Reader {
	reader, writer := io.Pipe()
	go func() {
		s.storage.WriteToWebVTT(writer)
		writer.Close()
	}()
	return reader
}

func (s *Subtitles) FormatForLLM() string {
	var result strings.Builder
	for _, item := range s.storage.Items {
		// Timestamps
		result.WriteString(formatDurationForLLM(item.StartAt))
		result.WriteString(" to ")
		result.WriteString(formatDurationForLLM(item.EndAt))
		result.WriteString(" - ")

		// Words
		result.WriteString(item.String())
		result.WriteString("\n")
	}

	return strings.TrimSpace(result.String())
}

func (s *Subtitles) FormatTextOnly() string {
	var result strings.Builder
	for _, item := range s.storage.Items {
		result.WriteString(item.String())
		result.WriteString(" ")
	}

	return strings.TrimSpace(result.String())
}

func formatDurationForLLM(dur time.Duration) string {
	dur = dur.Round(time.Second)
	hours := dur / time.Hour
	minutes := (dur - hours*time.Hour) / time.Minute
	seconds := (dur - hours*time.Hour - minutes*time.Minute) / time.Second

	if hours == 0 {
		return fmt.Sprintf("%02d:%02d", int(minutes), int(seconds))
	}

	return fmt.Sprintf("%02d:%02d:%02d", int(hours), int(minutes), int(seconds))
}
