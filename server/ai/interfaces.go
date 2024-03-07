package ai

import (
	"image"
	"io"

	"github.com/mattermost/mattermost-plugin-ai/server/ai/subtitles"
)

type LLMConfig struct {
	Model              string
	MaxGeneratedTokens int
}

type LanguageModelOption func(*LLMConfig)

func WithModel(model string) LanguageModelOption {
	return func(cfg *LLMConfig) {
		cfg.Model = model
	}
}

func WithMaxGeneratedTokens(maxGeneratedTokens int) LanguageModelOption {
	return func(cfg *LLMConfig) {
		cfg.MaxGeneratedTokens = maxGeneratedTokens
	}
}

type LanguageModel interface {
	ChatCompletion(conversation BotConversation, opts ...LanguageModelOption) (*TextStreamResult, error)
	ChatCompletionNoStream(conversation BotConversation, opts ...LanguageModelOption) (string, error)

	CountTokens(text string) int
	TokenLimit() int
}

type Transcriber interface {
	Transcribe(file io.Reader) (*subtitles.Subtitles, error)
}

type ImageGenerator interface {
	GenerateImage(prompt string) (image.Image, error)
}
