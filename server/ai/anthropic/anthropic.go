package anthropic

import (
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
)

const DefaultMaxTokens = 4096

type Anthropic struct {
	client         *Client
	defaultModel   string
	tokenLimit     int
	metricsService metrics.LLMetrics
	maxTokens      int
}

func New(llmService ai.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *Anthropic {
	client := NewClient(llmService.APIKey, httpClient)

	return &Anthropic{
		client:         client,
		defaultModel:   llmService.DefaultModel,
		tokenLimit:     llmService.TokenLimit,
		metricsService: metricsService,
		maxTokens:      llmService.MaxTokens,
	}
}

// conversationToMessages creates a system prompt and a slice of input messages from a bot conversation.
func conversationToMessages(conversation ai.BotConversation) (string, []InputMessage) {
	systemMessage := ""
	messages := make([]InputMessage, 0, len(conversation.Posts))
	for _, post := range conversation.Posts {
		previousRole := ""
		previousContent := ""
		if len(messages) > 0 {
			previous := messages[len(messages)-1]
			previousRole = previous.Role
			previousContent = previous.Content
		}
		switch post.Role {
		case ai.PostRoleSystem:
			systemMessage += post.Message
		case ai.PostRoleBot:
			if previousRole == RoleAssistant {
				previousContent += post.Message
				continue
			}
			messages = append(messages,
				InputMessage{
					Role:    RoleAssistant,
					Content: post.Message,
				},
			)
		case ai.PostRoleUser:
			if previousRole == RoleUser {
				previousContent += post.Message
				continue
			}
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
	config := ai.LLMConfig{
		Model: a.defaultModel,
	}
	if a.maxTokens == 0 {
		config.MaxGeneratedTokens = DefaultMaxTokens
	} else {
		config.MaxGeneratedTokens = a.maxTokens
	}
	return config
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
		MaxTokens: cfg.MaxGeneratedTokens,
	}
}

func (a *Anthropic) ChatCompletion(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (*ai.TextStreamResult, error) {
	a.metricsService.IncrementLLMRequests()

	request := a.createCompletionRequest(conversation, opts)
	request.Stream = true
	result, err := a.client.MessageCompletion(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send query to anthropic: %w", err)
	}

	return result, nil
}

func (a *Anthropic) ChatCompletionNoStream(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (string, error) {
	a.metricsService.IncrementLLMRequests()

	request := a.createCompletionRequest(conversation, opts)
	request.Stream = false
	result, err := a.client.MessageCompletionNoStream(request)
	if err != nil {
		return "", fmt.Errorf("failed to send query to anthropic: %w", err)
	}

	return result, nil
}

func (a *Anthropic) CountTokens(text string) int {
	return 0
}

func (a *Anthropic) TokenLimit() int {
	if a.tokenLimit > 0 {
		return a.tokenLimit
	}
	return 100000
}
