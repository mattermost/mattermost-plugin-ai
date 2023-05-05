package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"

	"github.com/sashabaranov/go-openai"
	openaiClient "github.com/sashabaranov/go-openai"
)

type OpenAI struct {
	openaiClient *openaiClient.Client
}

const (
	SummarizeThreadSystemMessage = `You are a helpful assistant that summarizes threads. Given a thread, return a summary of the thread using less than 30 words. Do not refer to the thread, just give the summary. Include who was speaking.

Then answer any questions the user has about the thread. Keep your responses short.
`

	AnswerThreadQuestionSystemMessage = `You are a helpful assistant that answers questions about threads. Give a short answer that correctly answers questions asked.
`
)

func New(apiKey string) *OpenAI {
	return &OpenAI{
		openaiClient: openaiClient.NewClient(apiKey),
	}
}

func (s *OpenAI) SummarizeThread(thread string) (string, error) {
	resp, err := s.openaiClient.CreateChatCompletion(
		context.Background(),
		openaiClient.ChatCompletionRequest{
			Model: openaiClient.GPT3Dot5Turbo,
			Messages: []openaiClient.ChatCompletionMessage{
				{
					Role:    openaiClient.ChatMessageRoleSystem,
					Content: SummarizeThreadSystemMessage,
				},
				{
					Role:    openaiClient.ChatMessageRoleUser,
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

func (s *OpenAI) AnswerQuestionOnThread(thread string, question string) (string, error) {
	resp, err := s.openaiClient.CreateChatCompletion(
		context.Background(),
		openaiClient.ChatCompletionRequest{
			Model: openaiClient.GPT3Dot5Turbo,
			Messages: []openaiClient.ChatCompletionMessage{
				{
					Role:    openaiClient.ChatMessageRoleSystem,
					Content: AnswerThreadQuestionSystemMessage,
				},
				{
					Role:    openaiClient.ChatMessageRoleUser,
					Content: thread,
				},
				{
					Role:    openaiClient.ChatMessageRoleUser,
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

func (s *OpenAI) GenerateImage(prompt string) (image.Image, error) {
	req := openaiClient.ImageRequest{
		Prompt:         prompt,
		Size:           openai.CreateImageSize256x256,
		ResponseFormat: openai.CreateImageResponseFormatB64JSON,
		N:              1,
	}

	respBase64, err := s.openaiClient.CreateImage(context.Background(), req)
	if err != nil {
		return nil, err
	}

	imgBytes, err := base64.StdEncoding.DecodeString(respBase64.Data[0].B64JSON)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(imgBytes)
	imgData, err := png.Decode(r)
	if err != nil {
		return nil, err
	}

	return imgData, nil
}
