// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"errors"

	"github.com/invopop/jsonschema"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost-plugin-ai/server/llm/subtitles"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
	openaiClient "github.com/sashabaranov/go-openai"
)

type OpenAI struct {
	client           *openaiClient.Client
	defaultModel     string
	inputTokenLimit  int
	streamingTimeout time.Duration
	metricsService   metrics.LLMetrics
	sendUserID       bool
	outputTokenLimit int
}

const StreamingTimeoutDefault = 10 * time.Second

const MaxFunctionCalls = 10

const OpenAIMaxImageSize = 20 * 1024 * 1024 // 20 MB

var ErrStreamingTimeout = errors.New("timeout streaming")

func NewAzure(llmService llm.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *OpenAI {
	return newOpenAI(llmService, httpClient, metricsService,
		func(apiKey string) openaiClient.ClientConfig {
			config := openaiClient.DefaultAzureConfig(apiKey, strings.TrimSuffix(llmService.APIURL, "/"))
			config.APIVersion = "2024-06-01"
			return config
		},
	)
}

func NewCompatible(llmService llm.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *OpenAI {
	return newOpenAI(llmService, httpClient, metricsService,
		func(apiKey string) openaiClient.ClientConfig {
			config := openaiClient.DefaultConfig(apiKey)
			config.BaseURL = strings.TrimSuffix(llmService.APIURL, "/")
			return config
		},
	)
}

func New(llmService llm.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *OpenAI {
	return newOpenAI(llmService, httpClient, metricsService,
		func(apiKey string) openaiClient.ClientConfig {
			config := openaiClient.DefaultConfig(apiKey)
			config.OrgID = llmService.OrgID
			return config
		},
	)
}

func newOpenAI(
	llmService llm.ServiceConfig,
	httpClient *http.Client,
	metricsService metrics.LLMetrics,
	baseConfigFunc func(apiKey string) openaiClient.ClientConfig,
) *OpenAI {
	apiKey := llmService.APIKey
	defaultModel := llmService.DefaultModel
	if defaultModel == "" {
		defaultModel = openaiClient.GPT3Dot5Turbo
	}

	config := baseConfigFunc(apiKey)
	config.HTTPClient = httpClient

	streamingTimeout := StreamingTimeoutDefault
	if llmService.StreamingTimeoutSeconds > 0 {
		streamingTimeout = time.Duration(llmService.StreamingTimeoutSeconds) * time.Second
	}

	return &OpenAI{
		client:           openaiClient.NewClientWithConfig(config),
		defaultModel:     defaultModel,
		inputTokenLimit:  llmService.InputTokenLimit,
		streamingTimeout: streamingTimeout,
		metricsService:   metricsService,
		sendUserID:       llmService.SendUserID,
		outputTokenLimit: llmService.OutputTokenLimit,
	}
}

func modifyCompletionRequestWithConversation(request openaiClient.ChatCompletionRequest, conversation llm.BotConversation) openaiClient.ChatCompletionRequest {
	request.Messages = postsToChatCompletionMessages(conversation.Posts)
	request.Tools = toolsToOpenAITools(conversation.Tools.GetTools())
	return request
}

func toolsToOpenAITools(tools []llm.Tool) []openaiClient.Tool {
	result := make([]openaiClient.Tool, 0, len(tools))

	schemaMaker := jsonschema.Reflector{
		Anonymous:      true,
		ExpandedStruct: true,
	}

	for _, tool := range tools {
		schema := schemaMaker.Reflect(tool.Schema)
		result = append(result, openaiClient.Tool{
			Type: openaiClient.ToolTypeFunction,
			Function: &openaiClient.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  schema,
			},
		})
	}

	return result
}

func postsToChatCompletionMessages(posts []llm.Post) []openaiClient.ChatCompletionMessage {
	result := make([]openaiClient.ChatCompletionMessage, 0, len(posts))

	for _, post := range posts {
		role := openaiClient.ChatMessageRoleUser
		if post.Role == llm.PostRoleBot {
			role = openaiClient.ChatMessageRoleAssistant
		} else if post.Role == llm.PostRoleSystem {
			role = openaiClient.ChatMessageRoleSystem
		}
		completionMessage := openaiClient.ChatCompletionMessage{
			Role: role,
		}

		if len(post.Files) > 0 {
			completionMessage.MultiContent = make([]openaiClient.ChatMessagePart, 0, len(post.Files)+1)
			if post.Message != "" {
				completionMessage.MultiContent = append(completionMessage.MultiContent, openaiClient.ChatMessagePart{
					Type: openaiClient.ChatMessagePartTypeText,
					Text: post.Message,
				})
			}
			for _, file := range post.Files {
				if file.MimeType != "image/png" &&
					file.MimeType != "image/jpeg" &&
					file.MimeType != "image/gif" &&
					file.MimeType != "image/webp" {
					completionMessage.MultiContent = append(completionMessage.MultiContent, openaiClient.ChatMessagePart{
						Type: openaiClient.ChatMessagePartTypeText,
						Text: "User submitted image was not a supported format. Tell the user this.",
					})
					continue
				}
				if file.Size > OpenAIMaxImageSize {
					completionMessage.MultiContent = append(completionMessage.MultiContent, openaiClient.ChatMessagePart{
						Type: openaiClient.ChatMessagePartTypeText,
						Text: "User submitted a image larger than 20MB. Tell the user this.",
					})
					continue
				}
				fileBytes, err := io.ReadAll(file.Reader)
				if err != nil {
					continue
				}
				imageEncoded := base64.StdEncoding.EncodeToString(fileBytes)
				encodedString := fmt.Sprintf("data:"+file.MimeType+";base64,%s", imageEncoded)
				completionMessage.MultiContent = append(completionMessage.MultiContent, openaiClient.ChatMessagePart{
					Type: openaiClient.ChatMessagePartTypeImageURL,
					ImageURL: &openaiClient.ChatMessageImageURL{
						URL:    encodedString,
						Detail: openaiClient.ImageURLDetailAuto,
					},
				})
			}
		} else {
			completionMessage.Content = post.Message
		}

		result = append(result, completionMessage)
	}

	return result
}

// createFunctionArgumentResolver Creates a resolver for the json arguments of an openai function call. Unmarshalling the json into the supplied struct.
func createFunctionArgumentResolver(jsonArgs string) llm.ToolArgumentGetter {
	return func(args any) error {
		return json.Unmarshal([]byte(jsonArgs), args)
	}
}

type ToolBufferElement struct {
	id   strings.Builder
	name strings.Builder
	args strings.Builder
}

func (s *OpenAI) streamResultToChannels(request openaiClient.ChatCompletionRequest, conversation llm.BotConversation, output chan<- string, errChan chan<- error) {
	request.Stream = true

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	// watchdog to cancel if the streaming stalls
	watchdog := make(chan struct{})
	go func() {
		timer := time.NewTimer(s.streamingTimeout)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				cancel(ErrStreamingTimeout)
				return
			case <-ctx.Done():
				return
			case <-watchdog:
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(s.streamingTimeout)
			}
		}
	}()

	stream, err := s.client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		if ctxErr := context.Cause(ctx); ctxErr != nil {
			errChan <- ctxErr
		} else {
			errChan <- err
		}
		return
	}

	defer stream.Close()

	// Buffering in the case of tool use
	var toolsBuffer map[int]*ToolBufferElement
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			if ctxErr := context.Cause(ctx); ctxErr != nil {
				errChan <- ctxErr
			} else {
				errChan <- err
			}
			return
		}

		// Ping the watchdog when we receive a response
		watchdog <- struct{}{}

		if len(response.Choices) == 0 {
			continue
		}

		// Check finishing conditions
		switch response.Choices[0].FinishReason {
		case "":
			// Not done yet, keep going
		case openaiClient.FinishReasonStop:
			return
		case openaiClient.FinishReasonToolCalls:
			// Verify OpenAI functions are not recursing too deep.
			numFunctionCalls := 0
			for i := len(request.Messages) - 1; i >= 0; i-- {
				if request.Messages[i].Role == openaiClient.ChatMessageRoleTool {
					numFunctionCalls++
				} else {
					break
				}
			}
			if numFunctionCalls > MaxFunctionCalls {
				errChan <- errors.New("too many function calls")
				return
			}

			// Transfer the buffered tools into tool calls
			tools := []openaiClient.ToolCall{}
			for i, tool := range toolsBuffer {
				name := tool.name.String()
				arguments := tool.args.String()
				toolID := tool.id.String()
				num := i
				tools = append(tools, openaiClient.ToolCall{
					Function: openaiClient.FunctionCall{
						Name:      name,
						Arguments: arguments,
					},
					ID:    toolID,
					Index: &num,
					Type:  openaiClient.ToolTypeFunction,
				})
			}

			// Add the tool calls to the request
			request.Messages = append(request.Messages, openaiClient.ChatCompletionMessage{
				Role:      openaiClient.ChatMessageRoleAssistant,
				ToolCalls: tools,
			})

			// Resolve the tools and create messages for each
			for _, tool := range tools {
				name := tool.Function.Name
				arguments := tool.Function.Arguments
				toolID := tool.ID
				toolResult, err := conversation.Tools.ResolveTool(name, createFunctionArgumentResolver(arguments), conversation.Context)
				if err != nil {
					fmt.Printf("Error resolving function %s: %s", name, err)
				}
				request.Messages = append(request.Messages, openaiClient.ChatCompletionMessage{
					Role:       openaiClient.ChatMessageRoleTool,
					Name:       name,
					Content:    toolResult,
					ToolCallID: toolID,
				})
			}

			// Call ourselves again with the result of the function call
			s.streamResultToChannels(request, conversation, output, errChan)
			return
		default:
			fmt.Printf("Unknown finish reason: %s", response.Choices[0].FinishReason)
			return
		}

		delta := response.Choices[0].Delta
		numTools := len(delta.ToolCalls)
		if numTools != 0 {
			if toolsBuffer == nil {
				toolsBuffer = make(map[int]*ToolBufferElement)
			}
			for _, toolCall := range delta.ToolCalls {
				if toolCall.Index == nil {
					continue
				}
				toolIndex := *toolCall.Index
				if toolsBuffer[toolIndex] == nil {
					toolsBuffer[toolIndex] = &ToolBufferElement{}
				}
				toolsBuffer[toolIndex].name.WriteString(toolCall.Function.Name)
				toolsBuffer[toolIndex].args.WriteString(toolCall.Function.Arguments)
				toolsBuffer[toolIndex].id.WriteString(toolCall.ID)
			}
		}

		output <- response.Choices[0].Delta.Content
	}
}

func (s *OpenAI) streamResult(request openaiClient.ChatCompletionRequest, conversation llm.BotConversation) (*llm.TextStreamResult, error) {
	output := make(chan string)
	errChan := make(chan error)
	go func() {
		defer close(output)
		defer close(errChan)
		s.streamResultToChannels(request, conversation, output, errChan)
	}()

	return &llm.TextStreamResult{Stream: output, Err: errChan}, nil
}

func (s *OpenAI) GetDefaultConfig() llm.LanguageModelConfig {
	return llm.LanguageModelConfig{
		Model:              s.defaultModel,
		MaxGeneratedTokens: s.outputTokenLimit,
	}
}

func (s *OpenAI) createConfig(opts []llm.LanguageModelOption) llm.LanguageModelConfig {
	cfg := s.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

func (s *OpenAI) completionRequestFromConfig(cfg llm.LanguageModelConfig) openaiClient.ChatCompletionRequest {
	request := openaiClient.ChatCompletionRequest{
		Model: cfg.Model,
	}

	if _, ok := openaiClient.O1SeriesModels[cfg.Model]; ok {
		request.MaxCompletionTokens = cfg.MaxGeneratedTokens
	} else {
		request.MaxTokens = cfg.MaxGeneratedTokens
	}

	return request
}

func (s *OpenAI) ChatCompletion(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	s.metricsService.IncrementLLMRequests()

	request := s.completionRequestFromConfig(s.createConfig(opts))
	request = modifyCompletionRequestWithConversation(request, conversation)
	request.Stream = true
	if s.sendUserID {
		request.User = conversation.Context.RequestingUser.Id
	}
	return s.streamResult(request, conversation)
}

func (s *OpenAI) ChatCompletionNoStream(conversation llm.BotConversation, opts ...llm.LanguageModelOption) (string, error) {
	// This could perform better if we didn't use the streaming API here, but the complexity is not worth it.
	result, err := s.ChatCompletion(conversation, opts...)
	if err != nil {
		return "", err
	}
	return result.ReadAll(), nil
}

func (s *OpenAI) Transcribe(file io.Reader) (*subtitles.Subtitles, error) {
	resp, err := s.client.CreateTranscription(context.Background(), openaiClient.AudioRequest{
		Model:    openaiClient.Whisper1,
		Reader:   file,
		FilePath: "input.mp3",
		Format:   openaiClient.AudioResponseFormatVTT,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create whisper transcription: %w", err)
	}

	timedTranscript, err := subtitles.NewSubtitlesFromVTT(strings.NewReader(resp.Text))
	if err != nil {
		return nil, fmt.Errorf("unable to parse whisper transcription: %w", err)
	}

	return timedTranscript, nil
}

func (s *OpenAI) GenerateImage(prompt string) (image.Image, error) {
	req := openaiClient.ImageRequest{
		Prompt:         prompt,
		Size:           openaiClient.CreateImageSize256x256,
		ResponseFormat: openaiClient.CreateImageResponseFormatB64JSON,
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

func (s *OpenAI) CountTokens(text string) int {
	// Counting tokens is really annoying, so we approximate for now.
	charCount := float64(len(text)) / 4.0
	wordCount := float64(len(strings.Fields(text))) / 0.75

	// Average the two
	return int((charCount + wordCount) / 2.0)
}

func (s *OpenAI) InputTokenLimit() int {
	if s.inputTokenLimit > 0 {
		return s.inputTokenLimit
	}

	switch {
	case strings.HasPrefix(s.defaultModel, "gpt-4o"),
		strings.HasPrefix(s.defaultModel, "o1-preview"),
		strings.HasPrefix(s.defaultModel, "o1-mini"),
		strings.HasPrefix(s.defaultModel, "gpt-4-turbo"),
		strings.HasPrefix(s.defaultModel, "gpt-4-0125-preview"),
		strings.HasPrefix(s.defaultModel, "gpt-4-1106-preview"):
		return 128000
	case strings.HasPrefix(s.defaultModel, "gpt-4"):
		return 8192
	case strings.HasPrefix(s.defaultModel, "gpt-3.5-turbo"),
		s.defaultModel == "gpt-3.5-turbo-0125",
		s.defaultModel == "gpt-3.5-turbo-1106":
		return 16385
	case s.defaultModel == "gpt-3.5-turbo-instruct":
		return 4096
	}

	return 128000 // Default fallback
}
