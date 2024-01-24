package anthropic

import (
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/pkg/errors"
)

const (
	HumanPrompt     = "\n\nHuman: "
	AssistantPrompt = "\n\nAssistant: "
)

type Anthropic struct {
	client       *Client
	defaultModel string
	maxTokens    int
}

func New(llmService ai.ServiceConfig) *Anthropic {
	client := NewClient(llmService.APIKey)

	return &Anthropic{
		client:       client,
		defaultModel: llmService.DefaultModel,
		maxTokens:    llmService.TokenLimit,
	}
}

func conversationToPrompt(conversation ai.BotConversation) string {
	prompt := strings.Builder{}
	for _, post := range conversation.Posts {
		if post.Role == ai.PostRoleBot {
			prompt.WriteString(AssistantPrompt + post.Message)
		} else if post.Role == ai.PostRoleUser || post.Role == ai.PostRoleSystem {
			prompt.WriteString(HumanPrompt + post.Message)
		}
	}

	prompt.WriteString(AssistantPrompt)

	return prompt.String()
}

func (a *Anthropic) GetDefaultConfig() ai.LLMConfig {
	return ai.LLMConfig{
		Model:     a.defaultModel,
		MaxTokens: 0,
	}
}

func (a *Anthropic) createConfig(opts []ai.LanguageModelOption) ai.LLMConfig { //nolint:unused
	cfg := a.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (a *Anthropic) ChatCompletion(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (*ai.TextStreamResult, error) {
	prompt := conversationToPrompt(conversation)
	result, err := a.client.Completion(prompt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send query to anthropic")
	}

	return result, nil
}

func (a *Anthropic) ChatCompletionNoStream(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (string, error) {
	prompt := conversationToPrompt(conversation)
	result, err := a.client.CompletionNoStream(prompt)
	if err != nil {
		return "", errors.Wrap(err, "failed to send query to anthropic")
	}

	return result, nil
}

func (a *Anthropic) CountTokens(text string) int {
	return 0
}

func (a *Anthropic) TokenLimit() int {
	if a.maxTokens > 0 {
		return a.maxTokens
	}
	return 100000
}
