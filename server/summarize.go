package main

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAISummarizer struct {
	openaiClient *openai.Client
}

const (
	SummarizeThreadSystemMessage = `You are a helpful assistant that summarizes threads. Given a thread, return a summary of the thread using less than 30 words. Do not refer to the thread, just give the summary. Include who was speaking.

Then answer any questions the user has about the thread. Keep your responses short.
`

	AnswerThreadQuestionSystemMessage = `You are a helpful assistant that answers questions about threads. Give a short answer that correctly answers questions asked.
`
)

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
