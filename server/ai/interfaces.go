package ai

import (
	"image"
	"io"
)

type LLMConfig struct {
	Model     string
	MaxTokens int
}

type LanguageModelOption func(*LLMConfig)

func WithModel(model string) LanguageModelOption {
	return func(cfg *LLMConfig) {
		cfg.Model = model
	}
}

func WithmaxTokens(maxTokens int) LanguageModelOption {
	return func(cfg *LLMConfig) {
		cfg.MaxTokens = maxTokens
	}
}

type LanguageModel interface {
	ChatCompletion(conversation BotConversation, opts ...LanguageModelOption) (*TextStreamResult, error)
	ChatCompletionNoStream(conversation BotConversation, opts ...LanguageModelOption) (string, error)
}

type Transcriber interface {
	Transcribe(file io.Reader) (string, error)
}

type ImageGenerator interface {
	GenerateImage(prompt string) (image.Image, error)
}
