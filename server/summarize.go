package main

import (
	"context"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAISummarizer struct {
	openaiClient *openai.Client
}

func NewOpenAISummarizer(apiKey string) *OpenAISummarizer {
	return &OpenAISummarizer{
		openaiClient: openai.NewClient(apiKey),
	}
}

func (s *OpenAISummarizer) SummarizeThread(thread string) (string, error) {
	resp, err := s.openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: SummarizeThreadSystemMessage,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: thread,
				},
			},
		},
	)
	if err != nil {
		return "", err
	}
	summary := resp.Choices[0].Message.Content

	return summary, nil
}

func (s *OpenAISummarizer) AnswerQuestionOnThread(thread string, question string) (string, error) {
	resp, err := s.openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: AnswerThreadQuestionSystemMessage,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: thread,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: question,
				},
			},
		},
	)
	if err != nil {
		return "", err
	}
	summary := resp.Choices[0].Message.Content

	return summary, nil
}

func (s *OpenAISummarizer) ThreadConversation(originalThread string, posts []string) (string, error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: AnswerThreadQuestionSystemMessage,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: originalThread,
		},
	}
	for i, post := range posts {
		role := openai.ChatMessageRoleUser
		if i%2 == 0 {
			role = openai.ChatMessageRoleAssistant
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: post,
		})
	}

	resp, err := s.openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
		},
	)
	if err != nil {
		return "", err
	}
	newMessage := resp.Choices[0].Message.Content

	return newMessage, nil

}

func (s *OpenAISummarizer) SelectEmoji(message string) (string, error) {
	resp, err := s.openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:     openai.GPT3Dot5Turbo,
			MaxTokens: 25,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: EmojiSystemMessage,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: message,
				},
			},
		},
	)
	if err != nil {
		return "", err
	}
	result := strings.Trim(strings.TrimSpace(resp.Choices[0].Message.Content), ":")

	return result, nil
}
