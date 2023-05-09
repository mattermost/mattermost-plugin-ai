package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"
	openaiClient "github.com/sashabaranov/go-openai"
)

type OpenAI struct {
	client *openaiClient.Client
}

func New(apiKey string) *OpenAI {
	return &OpenAI{
		client: openaiClient.NewClient(apiKey),
	}
}

func (s *OpenAI) QuestionAnswerStream(question string) (chan string, error) {
	request := openaiClient.ChatCompletionRequest{
		Model: openaiClient.GPT3Dot5Turbo,
		Messages: []openaiClient.ChatCompletionMessage{
			{
				Role:    openaiClient.ChatMessageRoleSystem,
				Content: GenericQuestionSystemMessage,
			},
			{
				Role:    openaiClient.ChatMessageRoleUser,
				Content: question,
			},
		},
		Stream: true,
	}

	stream, err := s.client.CreateChatCompletionStream(context.Background(), request)
	if err != nil {
		return nil, err
	}

	output := make(chan string)

	go func() {
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}

			if err != nil {
				fmt.Println("Stream error: " + err.Error())
				return
			}

			output <- response.Choices[0].Delta.Content
		}
	}()

	return output, nil
}

func (s *OpenAI) SummarizeThread(thread string) (string, error) {
	resp, err := s.client.CreateChatCompletion(
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
	resp, err := s.client.CreateChatCompletion(
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

	respBase64, err := s.client.CreateImage(context.Background(), req)
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

func (s *OpenAI) ThreadConversation(originalThread string, posts []string) (string, error) {
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

	resp, err := s.client.CreateChatCompletion(
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

func (s *OpenAI) SelectEmoji(message string) (string, error) {
	resp, err := s.client.CreateChatCompletion(
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
