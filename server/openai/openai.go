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

type Config struct {
	APIKey              string
	APIURL              string
	OrgID               string
	DefaultModel        string
	InputTokenLimit     int
	OutputTokenLimit    int
	StreamingTimeout    time.Duration
	SendUserID          bool
	EmbeddingModel      string
	EmbeddingDimentions int
}

type OpenAI struct {
	client         *openaiClient.Client
	config         Config
	metricsService metrics.LLMetrics
}

const (
	StreamingTimeoutDefault = 10 * time.Second
	MaxFunctionCalls        = 10
	OpenAIMaxImageSize      = 20 * 1024 * 1024 // 20 MB
)

var ErrStreamingTimeout = errors.New("timeout streaming")

func NewAzure(llmService llm.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *OpenAI {
	config := configFromLLMService(llmService)
	return newOpenAI(config, httpClient, metricsService,
		func(apiKey string) openaiClient.ClientConfig {
			clientConfig := openaiClient.DefaultAzureConfig(apiKey, strings.TrimSuffix(config.APIURL, "/"))
			clientConfig.APIVersion = "2024-06-01"
			return clientConfig
		},
	)
}

func NewCompatible(llmService llm.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *OpenAI {
	config := configFromLLMService(llmService)
	return newOpenAI(config, httpClient, metricsService,
		func(apiKey string) openaiClient.ClientConfig {
			clientConfig := openaiClient.DefaultConfig(apiKey)
			clientConfig.BaseURL = strings.TrimSuffix(config.APIURL, "/")
			return clientConfig
		},
	)
}

func New(llmService llm.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *OpenAI {
	config := configFromLLMService(llmService)
	return newOpenAI(config, httpClient, metricsService,
		func(apiKey string) openaiClient.ClientConfig {
			clientConfig := openaiClient.DefaultConfig(apiKey)
			clientConfig.OrgID = config.OrgID
			return clientConfig
		},
	)
}

// NewCompatibleEmbeddings creates a new OpenAI client configured only for embeddings functionality
func NewCompatibleEmbeddings(config Config, httpClient *http.Client) *OpenAI {
	if config.EmbeddingModel == "" {
		config.EmbeddingModel = string(openaiClient.LargeEmbedding3)
		config.EmbeddingDimentions = 3072
	}

	return newOpenAI(config, httpClient, nil,
		func(apiKey string) openaiClient.ClientConfig {
			clientConfig := openaiClient.DefaultConfig(apiKey)
			clientConfig.BaseURL = strings.TrimSuffix(config.APIURL, "/")
			return clientConfig
		},
	)
}

func configFromLLMService(llmService llm.ServiceConfig) Config {
	defaultModel := llmService.DefaultModel
	if defaultModel == "" {
		defaultModel = openaiClient.GPT3Dot5Turbo
	}

	streamingTimeout := StreamingTimeoutDefault
	if llmService.StreamingTimeoutSeconds > 0 {
		streamingTimeout = time.Duration(llmService.StreamingTimeoutSeconds) * time.Second
	}

	return Config{
		APIKey:           llmService.APIKey,
		APIURL:           llmService.APIURL,
		OrgID:            llmService.OrgID,
		DefaultModel:     defaultModel,
		InputTokenLimit:  llmService.InputTokenLimit,
		OutputTokenLimit: llmService.OutputTokenLimit,
		StreamingTimeout: streamingTimeout,
		SendUserID:       llmService.SendUserID,
	}
}

func newOpenAI(
	config Config,
	httpClient *http.Client,
	metricsService metrics.LLMetrics,
	baseConfigFunc func(apiKey string) openaiClient.ClientConfig,
) *OpenAI {
	clientConfig := baseConfigFunc(config.APIKey)
	clientConfig.HTTPClient = httpClient

	return &OpenAI{
		client:         openaiClient.NewClientWithConfig(clientConfig),
		config:         config,
		metricsService: metricsService,
	}
}

func modifyCompletionRequestWithRequest(openAIRequest openaiClient.ChatCompletionRequest, interalRequest llm.CompletionRequest) openaiClient.ChatCompletionRequest {
	openAIRequest.Messages = postsToChatCompletionMessages(interalRequest.Posts)
	if interalRequest.Context.Tools != nil {
		openAIRequest.Tools = toolsToOpenAITools(interalRequest.Context.Tools.GetTools())
	}
	return openAIRequest
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

func (s *OpenAI) streamResultToChannels(request openaiClient.ChatCompletionRequest, llmContext *llm.Context, output chan<- string, errChan chan<- error) {
	request.Stream = true

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	// watchdog to cancel if the streaming stalls
	watchdog := make(chan struct{})
	go func() {
		timer := time.NewTimer(s.config.StreamingTimeout)
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
				timer.Reset(s.config.StreamingTimeout)
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
				toolResult, err := llmContext.Tools.ResolveTool(name, createFunctionArgumentResolver(arguments), llmContext)
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
			s.streamResultToChannels(request, llmContext, output, errChan)
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

func (s *OpenAI) streamResult(request openaiClient.ChatCompletionRequest, llmContext *llm.Context) (*llm.TextStreamResult, error) {
	output := make(chan string)
	errChan := make(chan error)
	go func() {
		defer close(output)
		defer close(errChan)
		s.streamResultToChannels(request, llmContext, output, errChan)
	}()

	return &llm.TextStreamResult{Stream: output, Err: errChan}, nil
}

func (s *OpenAI) GetDefaultConfig() llm.LanguageModelConfig {
	return llm.LanguageModelConfig{
		Model:              s.config.DefaultModel,
		MaxGeneratedTokens: s.config.OutputTokenLimit,
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

func (s *OpenAI) ChatCompletion(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	s.metricsService.IncrementLLMRequests()

	openAIRequest := s.completionRequestFromConfig(s.createConfig(opts))
	openAIRequest = modifyCompletionRequestWithRequest(openAIRequest, request)
	openAIRequest.Stream = true
	if s.config.SendUserID {
		if request.Context.RequestingUser != nil {
			openAIRequest.User = request.Context.RequestingUser.Id
		}
	}
	return s.streamResult(openAIRequest, request.Context)
}

func (s *OpenAI) ChatCompletionNoStream(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (string, error) {
	// This could perform better if we didn't use the streaming API here, but the complexity is not worth it.
	result, err := s.ChatCompletion(request, opts...)
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
	if s.config.InputTokenLimit > 0 {
		return s.config.InputTokenLimit
	}

	switch {
	case strings.HasPrefix(s.config.DefaultModel, "gpt-4o"),
		strings.HasPrefix(s.config.DefaultModel, "o1-preview"),
		strings.HasPrefix(s.config.DefaultModel, "o1-mini"),
		strings.HasPrefix(s.config.DefaultModel, "gpt-4-turbo"),
		strings.HasPrefix(s.config.DefaultModel, "gpt-4-0125-preview"),
		strings.HasPrefix(s.config.DefaultModel, "gpt-4-1106-preview"):
		return 128000
	case strings.HasPrefix(s.config.DefaultModel, "gpt-4"):
		return 8192
	case strings.HasPrefix(s.config.DefaultModel, "gpt-3.5-turbo"),
		s.config.DefaultModel == "gpt-3.5-turbo-0125",
		s.config.DefaultModel == "gpt-3.5-turbo-1106":
		return 16385
	case s.config.DefaultModel == "gpt-3.5-turbo-instruct":
		return 4096
	}

	return 128000 // Default fallback
}

func (s *OpenAI) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := s.client.CreateEmbeddings(ctx, openaiClient.EmbeddingRequest{
		Input: []string{text},
		Model: openaiClient.EmbeddingModel(s.config.EmbeddingModel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	return resp.Data[0].Embedding, nil
}

// BatchCreateEmbeddings generates embeddings for multiple texts in a single API call
func (s *OpenAI) BatchCreateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := s.client.CreateEmbeddings(ctx, openaiClient.EmbeddingRequest{
		Input: texts,
		Model: openaiClient.EmbeddingModel(s.config.EmbeddingModel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings batch: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// Dimensions returns the dimensionality of the embeddings from the text-embedding-ada-002 model
func (s *OpenAI) Dimensions() int {
	return s.config.EmbeddingDimentions
}
