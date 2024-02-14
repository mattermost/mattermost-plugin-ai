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
	"net/url"
	"strings"
	"time"

	"errors"

	"github.com/invopop/jsonschema"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/subtitles"
	openaiClient "github.com/sashabaranov/go-openai"
)

type OpenAI struct {
	client           *openaiClient.Client
	defaultModel     string
	tokenLimit       int
	streamingTimeout time.Duration
}

const StreamingTimeoutDefault = 10 * time.Second

const MaxFunctionCalls = 10

const OpenAIMaxImageSize = 20 * 1024 * 1024 // 20 MB

var ErrStreamingTimeout = errors.New("timeout streaming")

func NewCompatible(llmService ai.ServiceConfig) *OpenAI {
	apiKey := llmService.APIKey
	endpointURL := strings.TrimSuffix(llmService.APIURL, "/")
	defaultModel := llmService.DefaultModel
	config := openaiClient.DefaultConfig(apiKey)
	config.BaseURL = endpointURL

	parsedURL, err := url.Parse(endpointURL)
	if err == nil && strings.HasSuffix(parsedURL.Host, "openai.azure.com") {
		config = openaiClient.DefaultAzureConfig(apiKey, endpointURL)
		config.APIVersion = "2023-07-01-preview"
	}

	streamingTimeout := StreamingTimeoutDefault
	if llmService.StreamingTimeoutSeconds > 0 {
		streamingTimeout = time.Duration(llmService.StreamingTimeoutSeconds) * time.Second
	}
	return &OpenAI{
		client:           openaiClient.NewClientWithConfig(config),
		defaultModel:     defaultModel,
		tokenLimit:       llmService.TokenLimit,
		streamingTimeout: streamingTimeout,
	}
}

func New(llmService ai.ServiceConfig) *OpenAI {
	defaultModel := llmService.DefaultModel
	if defaultModel == "" {
		defaultModel = openaiClient.GPT3Dot5Turbo
	}
	config := openaiClient.DefaultConfig(llmService.APIKey)
	config.OrgID = llmService.OrgID

	streamingTimeout := StreamingTimeoutDefault
	if llmService.StreamingTimeoutSeconds > 0 {
		streamingTimeout = time.Duration(llmService.StreamingTimeoutSeconds) * time.Second
	}

	return &OpenAI{
		client:           openaiClient.NewClientWithConfig(config),
		defaultModel:     defaultModel,
		tokenLimit:       llmService.TokenLimit,
		streamingTimeout: streamingTimeout,
	}
}

func modifyCompletionRequestWithConversation(request openaiClient.ChatCompletionRequest, conversation ai.BotConversation) openaiClient.ChatCompletionRequest {
	request.Messages = postsToChatCompletionMessages(conversation.Posts)
	request.Functions = toolsToFunctionDefinitions(conversation.Tools.GetTools())
	return request
}

func toolsToFunctionDefinitions(tools []ai.Tool) []openaiClient.FunctionDefinition {
	result := make([]openaiClient.FunctionDefinition, 0, len(tools))

	schemaMaker := jsonschema.Reflector{
		Anonymous:      true,
		ExpandedStruct: true,
	}

	for _, tool := range tools {
		schema := schemaMaker.Reflect(tool.Schema)
		result = append(result, openaiClient.FunctionDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  schema,
		})
	}

	return result
}

func postsToChatCompletionMessages(posts []ai.Post) []openaiClient.ChatCompletionMessage {
	result := make([]openaiClient.ChatCompletionMessage, 0, len(posts))

	for _, post := range posts {
		role := openaiClient.ChatMessageRoleUser
		if post.Role == ai.PostRoleBot {
			role = openaiClient.ChatMessageRoleAssistant
		} else if post.Role == ai.PostRoleSystem {
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

// createFunctionArrgmentResolver Creates a resolver for the json arguments of an openai function call. Unmarshaling the json into the supplied struct.
func createFunctionArrgmentResolver(jsonArgs string) ai.ToolArgumentGetter {
	return func(args any) error {
		return json.Unmarshal([]byte(jsonArgs), args)
	}
}

func (s *OpenAI) handleStreamFunctionCall(request openaiClient.ChatCompletionRequest, conversation ai.BotConversation, name, arguments string) (openaiClient.ChatCompletionRequest, error) {
	toolResult, err := conversation.Tools.ResolveTool(name, createFunctionArrgmentResolver(arguments), conversation.Context)
	if err != nil {
		fmt.Println("Error resolving function: ", err)
	}
	request.Messages = append(request.Messages, openaiClient.ChatCompletionMessage{
		Role:    openaiClient.ChatMessageRoleFunction,
		Name:    name,
		Content: toolResult,
	})

	return request, nil
}

func (s *OpenAI) streamResultToChannels(request openaiClient.ChatCompletionRequest, conversation ai.BotConversation, output chan<- string, errChan chan<- error) {
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

	// Buffering in the case of a function call.
	functionName := strings.Builder{}
	functionArguments := strings.Builder{}
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
		case openaiClient.FinishReasonFunctionCall:
			// Verify OpenAI functions are not recursing too deep.
			numFunctionCalls := 0
			for i := len(request.Messages) - 1; i >= 0; i-- {
				if request.Messages[i].Role == openaiClient.ChatMessageRoleFunction {
					numFunctionCalls++
				} else {
					break
				}
			}
			if numFunctionCalls > MaxFunctionCalls {
				errChan <- errors.New("too many function calls")
				return
			}

			// Call ourselves again with the result of the function call
			recursiveRequest, err := s.handleStreamFunctionCall(request, conversation, functionName.String(), functionArguments.String())
			if err != nil {
				errChan <- err
				return
			}
			s.streamResultToChannels(recursiveRequest, conversation, output, errChan)
			return
		default:
			fmt.Printf("Unknown finish reason: %s", response.Choices[0].FinishReason)
			return
		}

		// Keep track of any function call received
		if response.Choices[0].Delta.FunctionCall != nil {
			if response.Choices[0].Delta.FunctionCall.Name != "" {
				functionName.WriteString(response.Choices[0].Delta.FunctionCall.Name)
			}
			if response.Choices[0].Delta.FunctionCall.Arguments != "" {
				functionArguments.WriteString(response.Choices[0].Delta.FunctionCall.Arguments)
			}
		}

		output <- response.Choices[0].Delta.Content
	}
}

func (s *OpenAI) streamResult(request openaiClient.ChatCompletionRequest, conversation ai.BotConversation) (*ai.TextStreamResult, error) {
	output := make(chan string)
	errChan := make(chan error)
	go func() {
		defer close(output)
		defer close(errChan)
		s.streamResultToChannels(request, conversation, output, errChan)
	}()

	return &ai.TextStreamResult{Stream: output, Err: errChan}, nil
}

func (s *OpenAI) GetDefaultConfig() ai.LLMConfig {
	return ai.LLMConfig{
		Model:              s.defaultModel,
		MaxGeneratedTokens: 0,
	}
}

func (s *OpenAI) createConfig(opts []ai.LanguageModelOption) ai.LLMConfig {
	cfg := s.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

func (s *OpenAI) completionRequestFromConfig(cfg ai.LLMConfig) openaiClient.ChatCompletionRequest {
	return openaiClient.ChatCompletionRequest{
		Model:            cfg.Model,
		MaxTokens:        cfg.MaxGeneratedTokens,
		Temperature:      1.0,
		TopP:             1.0,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
	}
}

func (s *OpenAI) ChatCompletion(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (*ai.TextStreamResult, error) {
	request := s.completionRequestFromConfig(s.createConfig(opts))
	request = modifyCompletionRequestWithConversation(request, conversation)
	request.Stream = true
	return s.streamResult(request, conversation)
}

func (s *OpenAI) ChatCompletionNoStream(conversation ai.BotConversation, opts ...ai.LanguageModelOption) (string, error) {
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

func (s *OpenAI) TokenLimit() int {
	if s.tokenLimit > 0 {
		return s.tokenLimit
	}

	switch {
	case strings.HasPrefix(s.defaultModel, "gpt-4-32k"):
		return 32768
	case strings.HasPrefix(s.defaultModel, "gpt-4"):
		return 8192
	case strings.HasPrefix(s.defaultModel, "gpt-3.5-turbo-16k"):
		return 16384
	case strings.HasPrefix(s.defaultModel, "gpt-3.5-turbo"):
		return 4096
	}

	return 4096
}
