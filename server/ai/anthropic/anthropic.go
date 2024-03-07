package anthropic

import (
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/pkg/errors"
)

const DefaultMaxTokens = 4096

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

// conversationToMessages creates a system prompt and a slice of input messages from a bot conversation.
func conversationToMessages(conversation ai.BotConversation) (string, []InputMessage) {
	systemMessage := ""
	messages := make([]InputMessage, 0, len(conversation.Posts))
	for _, post := range conversation.Posts {
		switch post.Role {
		case ai.PostRoleSystem:
			systemMessage += post.Message
		case ai.PostRoleBot:
			messages = append(messages,
				InputMessage{
					Role:    RoleAssistant,
					Content: post.Message,
				},
			)
		case ai.PostRoleUser:
			messages = append(messages,
				InputMessage{
					Role:    RoleUser,
					Content: post.Message,
				},
			)
		}
	}

	return systemMessage, messages
}

func (a *Anthropic) GetDefaultConfig() ai.LLMConfig {
	return ai.LLMConfig{
		Model:     a.defaultModel,
		MaxTokens: DefaultMaxTokens,
	}
}

func (a *Anthropic) createConfig(opts []ai.LanguageModelOption) ai.LLMConfig {
	cfg := a.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (a *Anthropic) createCompletionRequest(conversation ai.BotConversation, opts []ai.LanguageModelOption) MessageRequest {
	system, messages := conversationToMessages(conversation)
	cfg := a.createConfig(opts)
	return MessageRequest{
		Model:     cfg.Model,
		Messages:  messages,
		System:    system,
		MaxTokens: cfg.MaxTokens,
	}
}

func (a *Anthropic) ChatCompletion(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (*ai.TextStreamResult, error) {
	request := a.createCompletionRequest(conversation, opts)
	request.Stream = true
	result, err := a.client.MessageCompletion(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send query to anthropic")
	}

	return result, nil
}

func (a *Anthropic) ChatCompletionNoStream(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (string, error) {
	request := a.createCompletionRequest(conversation, opts)
	request.Stream = false
	result, err := a.client.MessageCompletionNoStream(request)
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
