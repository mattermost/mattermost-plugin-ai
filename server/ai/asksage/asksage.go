package asksage

import (
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
)

type AskSage struct {
	client       *Client
	defaultModel string
	maxTokens    int
	metric       metrics.LLMetrics
}

func New(llmService ai.ServiceConfig, httpClient *http.Client, metric metrics.LLMetrics) *AskSage {
	client := NewClient("", httpClient)
	client.Login(GetTokenParams{
		Email:    llmService.Username,
		Password: llmService.Password,
	})
	return &AskSage{
		client:       client,
		defaultModel: llmService.DefaultModel,
		maxTokens:    llmService.TokenLimit,
		metric:       metric,
	}
}

func conversationToMessagesList(conversation ai.BotConversation) []Message {
	result := make([]Message, 0, len(conversation.Posts))

	for _, post := range conversation.Posts {
		role := RoleUser
		if post.Role == ai.PostRoleBot {
			role = RoleGPT
		} else if post.Role == ai.PostRoleSystem {
			continue // Ask Sage doesn't support this
		}
		result = append(result, Message{
			User:    role,
			Message: post.Message,
		})
	}

	return result
}

func (s *AskSage) GetDefaultConfig() ai.LLMConfig {
	return ai.LLMConfig{
		Model:              s.defaultModel,
		MaxGeneratedTokens: 0,
	}
}

func (s *AskSage) createConfig(opts []ai.LanguageModelOption) ai.LLMConfig {
	cfg := s.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (s *AskSage) queryParamsFromConfig(cfg ai.LLMConfig) QueryParams {
	return QueryParams{
		Model: cfg.Model,
	}
}

func (s *AskSage) ChatCompletion(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (*ai.TextStreamResult, error) {
	// Ask Sage does not support streaming.
	result, err := s.ChatCompletionNoStream(conversation, opts...)
	if err != nil {
		return nil, err
	}
	return ai.NewStreamFromString(result), nil
}

func (s *AskSage) ChatCompletionNoStream(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (string, error) {
	s.metric.IncrementLLMRequests()

	params := s.queryParamsFromConfig(s.createConfig(opts))
	params.Message = conversationToMessagesList(conversation)
	params.SystemPrompt = conversation.ExtractSystemMessage()
	params.Persona = "default"

	response, err := s.client.Query(params)
	if err != nil {
		return "", err
	}
	return response.Message, nil
}

// TODO: Implement actual token counting. For now just estimated based off OpenAI estimations
func (s *AskSage) CountTokens(text string) int {
	charCount := float64(len(text)) / 4.0
	wordCount := float64(len(strings.Fields(text))) / 0.75

	// Average the two and add a buffer
	return int((charCount+wordCount)/2.0) + 100
}

// TODO: Figure out what the actual token limit is. For now just be conservative.
func (s *AskSage) TokenLimit() int {
	return s.maxTokens
}
