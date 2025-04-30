// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package anthropic

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
)

const (
	DefaultMaxTokens       = 8192
	MaxToolResolutionDepth = 10
)

type messageState struct {
	messages []anthropicSDK.MessageParam
	system   string
	output   chan<- llm.TextStreamEvent
	depth    int
	config   llm.LanguageModelConfig
	tools    []llm.Tool
	resolver func(name string, argsGetter llm.ToolArgumentGetter, context *llm.Context) (string, error)
	context  *llm.Context
}

type Anthropic struct {
	client           *anthropicSDK.Client
	defaultModel     string
	inputTokenLimit  int
	metricsService   metrics.LLMetrics
	outputTokenLimit int
}

func New(llmService llm.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *Anthropic {
	client := anthropicSDK.NewClient(
		option.WithAPIKey(llmService.APIKey),
		option.WithHTTPClient(httpClient),
	)

	return &Anthropic{
		client:           client,
		defaultModel:     llmService.DefaultModel,
		inputTokenLimit:  llmService.InputTokenLimit,
		metricsService:   metricsService,
		outputTokenLimit: llmService.OutputTokenLimit,
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

// conversationToMessages creates a system prompt and a slice of input messages from conversation posts.
func conversationToMessages(posts []llm.Post) (string, []anthropicSDK.MessageParam) {
	systemMessage := ""
	messages := make([]anthropicSDK.MessageParam, 0, len(posts))

	var currentBlocks []anthropicSDK.ContentBlockParamUnion
	var currentRole anthropicSDK.MessageParamRole

	flushCurrentMessage := func() {
		if len(currentBlocks) > 0 {
			messages = append(messages, anthropicSDK.MessageParam{
				Role:    anthropicSDK.F(currentRole),
				Content: anthropicSDK.F(currentBlocks),
			})
			currentBlocks = nil
		}
	}

	for _, post := range posts {
		switch post.Role {
		case llm.PostRoleSystem:
			systemMessage += post.Message
			continue
		case llm.PostRoleBot:
			if currentRole != "assistant" {
				flushCurrentMessage()
				currentRole = "assistant"
			}
		case llm.PostRoleUser:
			if currentRole != "user" {
				flushCurrentMessage()
				currentRole = "user"
			}
		default:
			continue
		}

		if post.Message != "" {
			textBlock := anthropicSDK.TextBlockParam{
				Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
				Text: anthropicSDK.F(post.Message),
			}
			currentBlocks = append(currentBlocks, textBlock)
		}

		for _, file := range post.Files {
			if !isValidImageType(file.MimeType) {
				textBlock := anthropicSDK.TextBlockParam{
					Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
					Text: anthropicSDK.F(fmt.Sprintf("[Unsupported image type: %s]", file.MimeType)),
				}
				currentBlocks = append(currentBlocks, textBlock)
				continue
			}

			data, err := io.ReadAll(file.Reader)
			if err != nil {
				textBlock := anthropicSDK.TextBlockParam{
					Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
					Text: anthropicSDK.F("[Error reading image data]"),
				}
				currentBlocks = append(currentBlocks, textBlock)
				continue
			}

			imageBlock := anthropicSDK.ImageBlockParam{
				Type: anthropicSDK.F(anthropicSDK.ImageBlockParamTypeImage),
				Source: anthropicSDK.F(anthropicSDK.ImageBlockParamSource{
					Type:      anthropicSDK.F(anthropicSDK.ImageBlockParamSourceTypeBase64),
					MediaType: anthropicSDK.F(anthropicSDK.ImageBlockParamSourceMediaType(file.MimeType)),
					Data:      anthropicSDK.F(base64.StdEncoding.EncodeToString(data)),
				}),
			}
			currentBlocks = append(currentBlocks, imageBlock)
		}

		if len(post.ToolUse) > 0 {
			for _, tool := range post.ToolUse {
				toolBlock := anthropicSDK.ToolUseBlockParam{
					ID:    anthropicSDK.F(tool.ID),
					Type:  anthropicSDK.F(anthropicSDK.ToolUseBlockParamTypeToolUse),
					Name:  anthropicSDK.F(tool.Name),
					Input: anthropicSDK.Raw[any](tool.Arguments),
				}
				currentBlocks = append(currentBlocks, toolBlock)
			}

			resultBlocks := make([]anthropicSDK.ContentBlockParamUnion, 0, len(post.ToolUse))
			for _, tool := range post.ToolUse {
				if tool.Result != "" {
					toolResultBlock := anthropicSDK.ToolResultBlockParam{
						Type:      anthropicSDK.F(anthropicSDK.ToolResultBlockParamTypeToolResult),
						ToolUseID: anthropicSDK.F(tool.ID),
						Content: anthropicSDK.F([]anthropicSDK.ToolResultBlockParamContentUnion{
							anthropicSDK.TextBlockParam{
								Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
								Text: anthropicSDK.F(tool.Result),
							},
						}),
					}
					resultBlocks = append(resultBlocks, toolResultBlock)
				}
			}

			if len(resultBlocks) > 0 {
				flushCurrentMessage()
				currentRole = anthropicSDK.MessageParamRoleUser
				currentBlocks = resultBlocks
				flushCurrentMessage()
			}
		}
	}

	flushCurrentMessage()
	return systemMessage, messages
}

func (a *Anthropic) GetDefaultConfig() llm.LanguageModelConfig {
	config := llm.LanguageModelConfig{
		Model: a.defaultModel,
	}
	if a.outputTokenLimit == 0 {
		config.MaxGeneratedTokens = DefaultMaxTokens
	} else {
		config.MaxGeneratedTokens = a.outputTokenLimit
	}
	return config
}

func (a *Anthropic) createConfig(opts []llm.LanguageModelOption) llm.LanguageModelConfig {
	cfg := a.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (a *Anthropic) streamChatWithTools(state messageState) error {
	if state.depth >= MaxToolResolutionDepth {
		state.output <- llm.TextStreamEvent{
			Type:  llm.EventTypeError,
			Value: fmt.Errorf("max tool resolution depth (%d) exceeded", MaxToolResolutionDepth),
		}
		return fmt.Errorf("max tool resolution depth (%d) exceeded", MaxToolResolutionDepth)
	}

	stream := a.client.Messages.NewStreaming(context.Background(), anthropicSDK.MessageNewParams{
		Model:     anthropicSDK.F(state.config.Model),
		MaxTokens: anthropicSDK.F(int64(state.config.MaxGeneratedTokens)),
		Messages:  anthropicSDK.F(state.messages),
		System: anthropicSDK.F([]anthropicSDK.TextBlockParam{{
			Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
			Text: anthropicSDK.F(state.system),
		}}),
		Tools: anthropicSDK.F(convertTools(state.tools)),
	})

	message := anthropicSDK.Message{}
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return fmt.Errorf("error accumulating message: %w", err)
		}

		// Stream text content immediately
		switch delta := event.Delta.(type) { // nolint: gocritic
		case anthropicSDK.ContentBlockDeltaEventDelta:
			if delta.Text != "" {
				state.output <- llm.TextStreamEvent{
					Type:  llm.EventTypeText,
					Value: delta.Text,
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		state.output <- llm.TextStreamEvent{
			Type:  llm.EventTypeError,
			Value: fmt.Errorf("error from anthropic stream: %w", err),
		}
		return fmt.Errorf("error from anthropic stream: %w", err)
	}

	// Check for tool usage after message is complete
	pendingToolCalls := make([]llm.ToolCall, 0, len(message.Content))
	for _, block := range message.Content {
		if block.Type == anthropicSDK.ContentBlockTypeToolUse {
			// Convert to pending tool calls
			for _, block := range message.Content {
				if block.Type == anthropicSDK.ContentBlockTypeToolUse {
					pendingToolCalls = append(pendingToolCalls, llm.ToolCall{
						ID:          block.ID,
						Name:        block.Name,
						Description: "",
						Arguments:   block.Input,
					})
				}
			}
		}
	}

	// If tools were used, send tool calls event and end the stream
	if len(pendingToolCalls) > 0 {
		// Send the tool calls event
		state.output <- llm.TextStreamEvent{
			Type:  llm.EventTypeToolCalls,
			Value: pendingToolCalls,
		}
	}

	// Send end event if no tools were used
	state.output <- llm.TextStreamEvent{
		Type:  llm.EventTypeEnd,
		Value: nil,
	}

	return nil
}

func (a *Anthropic) ChatCompletion(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	a.metricsService.IncrementLLMRequests()

	eventStream := make(chan llm.TextStreamEvent)

	cfg := a.createConfig(opts)

	system, messages := conversationToMessages(request.Posts)

	initialState := messageState{
		messages: messages,
		system:   system,
		output:   eventStream,
		depth:    0,
		config:   cfg,
		context:  request.Context,
	}

	if request.Context.Tools != nil {
		initialState.tools = request.Context.Tools.GetTools()
		initialState.resolver = request.Context.Tools.ResolveTool
	}

	go func() {
		defer close(eventStream)

		_ = a.streamChatWithTools(initialState)
	}()

	return &llm.TextStreamResult{Stream: eventStream}, nil
}

func (a *Anthropic) ChatCompletionNoStream(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (string, error) {
	// This could perform better if we didn't use the streaming API here, but the complexity is not worth it.
	result, err := a.ChatCompletion(request, opts...)
	if err != nil {
		return "", err
	}
	return result.ReadAll(), nil
}

func (a *Anthropic) CountTokens(text string) int {
	return 0
}

// convertTools converts from llm.Tool to anthropicSDK.Tool format
func convertTools(tools []llm.Tool) []anthropicSDK.ToolParam {
	converted := make([]anthropicSDK.ToolParam, len(tools))
	for i, tool := range tools {
		converted[i] = anthropicSDK.ToolParam{
			Name:        anthropicSDK.F(tool.Name),
			Description: anthropicSDK.F(tool.Description),
			InputSchema: anthropicSDK.Raw[any](tool.Schema),
		}
	}
	return converted
}

func (a *Anthropic) InputTokenLimit() int {
	if a.inputTokenLimit > 0 {
		return a.inputTokenLimit
	}
	return 100000
}
