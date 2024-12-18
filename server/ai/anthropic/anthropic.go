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
}

func New(llmService ai.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *Anthropic {
	client := NewClient(llmService.APIKey, httpClient)

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
func conversationToMessages(conversation ai.BotConversation) (string, []InputMessage) {
	systemMessage := ""
	messages := make([]InputMessage, 0, len(conversation.Posts))

	var currentBlocks []ContentBlock
	var currentRole string

	flushCurrentMessage := func() {
		if len(currentBlocks) > 0 {
			var content interface{}
			if len(currentBlocks) == 1 && currentBlocks[0].Type == "text" {
				content = currentBlocks[0].Text
			} else {
				content = currentBlocks
			}
			messages = append(messages, InputMessage{
				Role:    currentRole,
				Content: content,
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
			if currentRole != RoleAssistant {
				flushCurrentMessage()
				currentRole = RoleAssistant
			}
		case ai.PostRoleUser:
			if currentRole != RoleUser {
				flushCurrentMessage()
				currentRole = RoleUser
			}
		default:
			continue
		}

		// Handle text message
		if post.Message != "" {
			currentBlocks = append(currentBlocks, ContentBlock{
				Type: "text",
				Text: post.Message,
			})
		}

		// Handle files/images
		for _, file := range post.Files {
			if !isValidImageType(file.MimeType) {
				currentBlocks = append(currentBlocks, ContentBlock{
					Type: "text",
					Text: fmt.Sprintf("[Unsupported image type: %s]", file.MimeType),
				})
				continue
			}

			// Read image data
			data, err := io.ReadAll(file.Reader)
			if err != nil {
				currentBlocks = append(currentBlocks, ContentBlock{
					Type: "text",
					Text: "[Error reading image data]",
				})
				continue
			}

			currentBlocks = append(currentBlocks, ContentBlock{
				Type: "image",
				Source: &ImageSource{
					Type:      "base64",
					MediaType: file.MimeType,
					Data:      base64.StdEncoding.EncodeToString(data),
				},
			})
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
