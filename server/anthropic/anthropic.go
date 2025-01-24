// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/invopop/jsonschema"
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
	output   chan<- string
	errChan  chan<- error
	depth    int
	config   llm.LanguageModelConfig
	tools    []llm.Tool
	resolver func(name string, argsGetter llm.ToolArgumentGetter, context llm.ConversationContext) (string, error)
	context  llm.ConversationContext
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
	var toolResults []anthropicSDK.ContentBlockParamUnion

	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return fmt.Errorf("error accumulating message: %w", err)
		}

		// Stream text content immediately
		switch delta := event.Delta.(type) { // nolint: gocritic
		case anthropicSDK.ContentBlockDeltaEventDelta:
			if delta.Text != "" {
				state.output <- delta.Text
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("error from anthropic stream: %w", err)
	}

	// Check for tool usage after message is complete
	for _, block := range message.Content {
		if block.Type == anthropicSDK.ContentBlockTypeToolUse {
			// Resolve the tool
			result, err := state.resolver(block.Name, func(args any) error {
				return json.Unmarshal(block.Input, args)
			}, state.context)

			if err != nil {
				return fmt.Errorf("tool resolution error: %w", err)
			}

			toolResults = append(toolResults, anthropicSDK.NewToolResultBlock(block.ID, result, false))
		}
	}

	// If tools were used, continue the conversation with the results
	if len(toolResults) > 0 {
		// Add tool results as a new user message
		state.messages = append(state.messages,
			message.ToParam(),
			anthropicSDK.MessageParam{
				Role:    anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
				Content: anthropicSDK.F(toolResults),
			},
		)

		newState := messageState{
			messages: state.messages,
			system:   state.system,
			output:   state.output,
			errChan:  state.errChan,
			depth:    state.depth + 1,
			config:   state.config,
			tools:    state.tools,
			resolver: state.resolver,
			context:  state.context,
		}

		// Recursively handle the continued conversation
		if err := a.streamChatWithTools(newState); err != nil {
			return err
		}
	}

	return nil
}

func (a *Anthropic) ChatCompletion(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	a.metricsService.IncrementLLMRequests()

	output := make(chan string)
	errChan := make(chan error)

	cfg := a.createConfig(opts)

	system, messages := conversationToMessages(conversation.Posts)

	initialState := messageState{
		messages: messages,
		system:   system,
		output:   output,
		errChan:  errChan,
		depth:    0,
		config:   cfg,
		tools:    conversation.Tools.GetTools(),
		resolver: conversation.Tools.ResolveTool,
		context:  conversation.Context,
	}

	go func() {
		defer close(output)
		defer close(errChan)

		if err := a.streamChatWithTools(initialState); err != nil {
			errChan <- err
		}
	}()

	return &llm.TextStreamResult{Stream: output, Err: errChan}, nil
}

func (a *Anthropic) ChatCompletionNoStream(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (string, error) {
	// This could perform better if we didn't use the streaming API here, but the complexity is not worth it.
	result, err := a.ChatCompletion(conversation, opts...)
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
		reflector := jsonschema.Reflector{
			AllowAdditionalProperties: false,
			DoNotReference:            true,
		}
		schema := any(reflector.Reflect(tool.Schema))
		converted[i] = anthropicSDK.ToolParam{
			Name:        anthropicSDK.F(tool.Name),
			Description: anthropicSDK.F(tool.Description),
			InputSchema: anthropicSDK.F(schema),
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
