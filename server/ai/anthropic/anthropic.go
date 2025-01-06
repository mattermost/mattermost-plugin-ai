package anthropic

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
)

const DefaultMaxTokens = 4096

type Anthropic struct {
	client         *anthropicSDK.Client
	defaultModel   string
	tokenLimit     int
	metricsService metrics.LLMetrics
}

func New(llmService ai.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *Anthropic {
	client := anthropicSDK.NewClient(
		option.WithAPIKey(llmService.APIKey),
		option.WithHTTPClient(httpClient),
	)

	return &Anthropic{
		client:         client,
		defaultModel:   llmService.DefaultModel,
		tokenLimit:     llmService.TokenLimit,
		metricsService: metricsService,
	}
}

// isValidImageType checks if the MIME type is supported by the Anthropic API
func isValidImageType(mimeType string) bool {
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	return validTypes[mimeType]
}

// conversationToMessages creates a system prompt and a slice of input messages from a bot conversation.
func conversationToMessages(conversation ai.BotConversation) (string, []anthropicSDK.MessageParam) {
	systemMessage := ""
	messages := make([]anthropicSDK.MessageParam, 0, len(conversation.Posts))

	var currentBlocks []anthropicSDK.ContentBlockParam
	var currentRole string

	flushCurrentMessage := func() {
		if len(currentBlocks) > 0 {
			messages = append(messages, anthropicSDK.MessageParam{
				Role:    currentRole,
				Content: currentBlocks,
			})
			currentBlocks = nil
		}
	}

	for _, post := range conversation.Posts {
		switch post.Role {
		case ai.PostRoleSystem:
			systemMessage += post.Message
			continue
		case ai.PostRoleBot:
			if currentRole != "assistant" {
				flushCurrentMessage()
				currentRole = "assistant"
			}
		case ai.PostRoleUser:
			if currentRole != "user" {
				flushCurrentMessage()
				currentRole = "user"
			}
		default:
			continue
		}

		if post.Message != "" {
			currentBlocks = append(currentBlocks, anthropicSDK.NewTextBlock(post.Message))
		}

		for _, file := range post.Files {
			if !isValidImageType(file.MimeType) {
				currentBlocks = append(currentBlocks, anthropicSDK.NewTextBlock(
					fmt.Sprintf("[Unsupported image type: %s]", file.MimeType),
				))
				continue
			}

			data, err := io.ReadAll(file.Reader)
			if err != nil {
				currentBlocks = append(currentBlocks, anthropicSDK.NewTextBlock("[Error reading image data]"))
				continue
			}

			currentBlocks = append(currentBlocks, anthropicSDK.NewImageBlock(
				anthropicSDK.ImageBlockSource{
					Type:      "base64",
					MediaType: file.MimeType,
					Data:      base64.StdEncoding.EncodeToString(data),
				},
			))
		}
	}

	flushCurrentMessage()
	return systemMessage, messages
}

func (a *Anthropic) GetDefaultConfig() ai.LLMConfig {
	return ai.LLMConfig{
		Model:              a.defaultModel,
		MaxGeneratedTokens: DefaultMaxTokens,
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
		MaxTokens: cfg.MaxGeneratedTokens,
	}
}

func (a *Anthropic) ChatCompletion(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (*ai.TextStreamResult, error) {
	a.metricsService.IncrementLLMRequests()

	system, messages := conversationToMessages(conversation)
	cfg := a.createConfig(opts)

	stream := a.client.Messages.NewStreaming(context.Background(), anthropicSDK.MessageNewParams{
		Model:     anthropicSDK.F(cfg.Model),
		MaxTokens: anthropicSDK.F(int64(cfg.MaxGeneratedTokens)),
		Messages:  anthropicSDK.F(messages),
		System:    anthropicSDK.F([]anthropicSDK.TextBlockParam{anthropicSDK.NewTextBlock(system)}),
	})

	output := make(chan string)
	errChan := make(chan error)

	go func() {
		defer close(output)
		defer close(errChan)

		for stream.Next() {
			event := stream.Current()
			switch delta := event.Delta.(type) {
			case anthropicSDK.ContentBlockDeltaEventDelta:
				if delta.Text != "" {
					output <- delta.Text
				}
			}
		}

		if err := stream.Err(); err != nil {
			errChan <- err
		}
	}()

	return &ai.TextStreamResult{Stream: output, Err: errChan}, nil
}

func (a *Anthropic) ChatCompletionNoStream(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (string, error) {
	a.metricsService.IncrementLLMRequests()

	system, messages := conversationToMessages(conversation)
	cfg := a.createConfig(opts)

	message, err := a.client.Messages.New(context.Background(), anthropicSDK.MessageNewParams{
		Model:     anthropicSDK.F(cfg.Model),
		MaxTokens: anthropicSDK.F(int64(cfg.MaxGeneratedTokens)),
		Messages:  anthropicSDK.F(messages),
		System:    anthropicSDK.F([]anthropicSDK.TextBlockParam{anthropicSDK.NewTextBlock(system)}),
	})
	if err != nil {
		return "", fmt.Errorf("failed to send query to anthropic: %w", err)
	}

	return message.Content[0].Text, nil
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
