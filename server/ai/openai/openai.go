package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"io"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/sashabaranov/go-openai"
	openaiClient "github.com/sashabaranov/go-openai"
)

type OpenAI struct {
	client       *openaiClient.Client
	defaultModel string
}

func NewCompatible(apiKey string, url string, model string) *OpenAI {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = url
	return &OpenAI{
		client:       openaiClient.NewClientWithConfig(config),
		defaultModel: model,
	}
}

func New(apiKey string, defaultModel string) *OpenAI {
	if defaultModel == "" {
		defaultModel = openaiClient.GPT4
	}
	return &OpenAI{
		client:       openaiClient.NewClient(apiKey),
		defaultModel: defaultModel,
	}
}

func conversationToCompletion(conversation ai.BotConversation) []openaiClient.ChatCompletionMessage {
	result := make([]openaiClient.ChatCompletionMessage, 0, len(conversation.Posts))

	for _, post := range conversation.Posts {
		role := openaiClient.ChatMessageRoleUser
		if post.Role == ai.PostRoleBot {
			role = openaiClient.ChatMessageRoleAssistant
		} else if post.Role == ai.PostRoleSystem {
			role = openaiClient.ChatMessageRoleSystem
		}
		result = append(result, openai.ChatCompletionMessage{
			Role:    role,
			Content: post.Message,
		})
	}

	return result
}

func (s *OpenAI) streamResult(request openaiClient.ChatCompletionRequest) (*ai.TextStreamResult, error) {
	output := make(chan string)
	errChan := make(chan error)
	go func() {
		defer close(output)
		defer close(errChan)
		request.Stream = true
		stream, err := s.client.CreateChatCompletionStream(context.Background(), request)
		if err != nil {
			errChan <- err
			return
		}

		defer stream.Close()

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}

			if err != nil {
				errChan <- err
				return
			}

			output <- response.Choices[0].Delta.Content
		}
	}()

	return &ai.TextStreamResult{Stream: output, Err: errChan}, nil
}

func (s *OpenAI) GetDefaultConfig() ai.LLMConfig {
	return ai.LLMConfig{
		Model:     s.defaultModel,
		MaxTokens: 0,
	}
}

func (s *OpenAI) createConfig(opts []ai.LanguageModelOption) ai.LLMConfig {
	cfg := s.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (s *OpenAI) completionReqeustFromConfig(cfg ai.LLMConfig) openaiClient.ChatCompletionRequest {
	return openaiClient.ChatCompletionRequest{
		Model:     cfg.Model,
		MaxTokens: cfg.MaxTokens,
	}
}

func (s *OpenAI) ChatCompletion(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (*ai.TextStreamResult, error) {
	request := s.completionReqeustFromConfig(s.createConfig(opts))
	request.Messages = conversationToCompletion(conversation)
	request.Stream = true
	return s.streamResult(request)
}

func (s *OpenAI) ChatCompletionNoStream(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (string, error) {
	request := s.completionReqeustFromConfig(s.createConfig(opts))
	request.Messages = conversationToCompletion(conversation)
	response, err := s.client.CreateChatCompletion(context.Background(), request)
	if err != nil {
		return "", err
	}
	return response.Choices[0].Message.Content, nil
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
