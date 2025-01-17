package llm

type LanguageModel interface {
	ChatCompletion(conversation BotConversation, opts ...LanguageModelOption) (*TextStreamResult, error)
	ChatCompletionNoStream(conversation BotConversation, opts ...LanguageModelOption) (string, error)

	CountTokens(text string) int
	TokenLimit() int
}

type LanguageModelConfig struct {
	Model              string
	MaxGeneratedTokens int
	EnableVision       bool
}

type LanguageModelOption func(*LanguageModelConfig)

func WithModel(model string) LanguageModelOption {
	return func(cfg *LanguageModelConfig) {
		cfg.Model = model
	}
}
func WithMaxGeneratedTokens(maxGeneratedTokens int) LanguageModelOption {
	return func(cfg *LanguageModelConfig) {
		cfg.MaxGeneratedTokens = maxGeneratedTokens
	}
}
