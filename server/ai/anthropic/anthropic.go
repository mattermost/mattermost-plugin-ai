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
}

func New(apiKey, defaultModel string) *Anthropic {
	client := NewClient(apiKey)

	return &Anthropic{
		client:       client,
		defaultModel: defaultModel,
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

func (s *Anthropic) GetDefaultConfig() ai.LLMConfig {
	return ai.LLMConfig{
		Model:     s.defaultModel,
		MaxTokens: 0,
	}
}

func (s *Anthropic) createConfig(opts []ai.LanguageModelOption) ai.LLMConfig {
	cfg := s.GetDefaultConfig()
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
	return 100000
}
