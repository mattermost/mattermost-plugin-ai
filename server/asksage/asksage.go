package asksage

import (
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
)

type AskSage struct {
	client           *Client
	defaultModel     string
	inputTokenLimit  int
	metric           metrics.LLMetrics
	outputTokenLimit int
}

func New(llmService llm.ServiceConfig, httpClient *http.Client, metric metrics.LLMetrics) *AskSage {
	client := NewClient("", httpClient)
	if err := client.Login(GetTokenParams{
		Email:    llmService.Username,
		Password: llmService.Password,
	}); err != nil {
		return nil
	}

	return &AskSage{
		client:           client,
		defaultModel:     llmService.DefaultModel,
		inputTokenLimit:  llmService.InputTokenLimit,
		metric:           metric,
		outputTokenLimit: llmService.OutputTokenLimit,
	}
}

func conversationToMessagesList(conversation llm.BotConversation) []Message {
	result := make([]Message, 0, len(conversation.Posts))

	for _, post := range conversation.Posts {
		role := RoleUser
		if post.Role == llm.PostRoleBot {
			role = RoleGPT
		} else if post.Role == llm.PostRoleSystem {
			continue // Ask Sage doesn't support this
		}
		result = append(result, Message{
			User:    role,
			Message: post.Message,
		})
	}

	return result
}

func (s *AskSage) GetDefaultConfig() llm.LanguageModelConfig {
	return llm.LanguageModelConfig{
		Model:              s.defaultModel,
		MaxGeneratedTokens: s.outputTokenLimit,
	}
}

func (s *AskSage) createConfig(opts []llm.LanguageModelOption) llm.LanguageModelConfig {
	cfg := s.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (s *AskSage) queryParamsFromConfig(cfg llm.LanguageModelConfig) QueryParams {
	return QueryParams{
		Model: cfg.Model,
	}
}

func (s *AskSage) ChatCompletion(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	// Ask Sage does not support streaming.
	result, err := s.ChatCompletionNoStream(conversation, opts...)
	if err != nil {
		return nil, err
	}
	return llm.NewStreamFromString(result), nil
}

func (s *AskSage) ChatCompletionNoStream(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (string, error) {
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
func (s *AskSage) InputTokenLimit() int {
	return s.inputTokenLimit
}
