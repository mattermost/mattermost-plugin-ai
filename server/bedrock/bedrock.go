// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bedrock

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
)

const (
	StreamingTimeoutDefault = 10 * time.Second
	MaxToolResolutionDepth  = 10
)

// Config represents AWS Bedrock specific configuration
type Config struct {
	APIKey           string        `json:"apiKey"`
	APISecret        string        `json:"apiSecret"`
	Region           string        `json:"region"`
	DefaultModel     string        `json:"defaultModel"`
	InputTokenLimit  int           `json:"inputTokenLimit"`
	OutputTokenLimit int           `json:"outputTokenLimit"`
	StreamingTimeout time.Duration `json:"streamingTimeout"`
	SendUserID       bool          `json:"sendUserID"`
}

// Bedrock is the implementation of the AWS Bedrock LLM provider
type Bedrock struct {
	client         *bedrockruntime.Client
	config         Config
	metricsService metrics.LLMetrics
}

// New creates a new Bedrock LLM provider
func New(llmService llm.ServiceConfig, httpClient *http.Client, metricsService metrics.LLMetrics) *Bedrock {
	config := configFromLLMService(llmService)

	// Configure AWS SDK
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(config.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.APIKey,
			config.APISecret,
			"",
		)),
		awsconfig.WithHTTPClient(httpClient),
	)

	if err != nil {
		// Log the error but continue with empty config
		fmt.Printf("Error loading AWS config: %v", err)
		awsCfg = aws.Config{}
	}

	// Create Bedrock Runtime client
	client := bedrockruntime.NewFromConfig(awsCfg)

	return &Bedrock{
		client:         client,
		config:         config,
		metricsService: metricsService,
	}
}

// configFromLLMService converts generic LLM service config to Bedrock specific config
func configFromLLMService(llmService llm.ServiceConfig) Config {
	defaultModel := llmService.DefaultModel
	if defaultModel == "" {
		defaultModel = "anthropic.claude-3-sonnet-20240229-v1:0" // Default to Claude 3 Sonnet
	}

	streamingTimeout := StreamingTimeoutDefault
	if llmService.StreamingTimeoutSeconds > 0 {
		streamingTimeout = time.Duration(llmService.StreamingTimeoutSeconds) * time.Second
	}

	// API Secret is stored in OrgID field
	return Config{
		APIKey:           llmService.APIKey,
		APISecret:        llmService.OrgID,
		Region:           llmService.APIURL, // Using APIURL field to store region
		DefaultModel:     defaultModel,
		InputTokenLimit:  llmService.InputTokenLimit,
		OutputTokenLimit: llmService.OutputTokenLimit,
		StreamingTimeout: streamingTimeout,
		SendUserID:       llmService.SendUserID,
	}
}

// GetDefaultConfig returns the default LLM config for Bedrock
func (b *Bedrock) GetDefaultConfig() llm.LanguageModelConfig {
	config := llm.LanguageModelConfig{
		Model: b.config.DefaultModel,
	}

	if b.config.OutputTokenLimit == 0 {
		config.MaxGeneratedTokens = 4096 // Default token limit
	} else {
		config.MaxGeneratedTokens = b.config.OutputTokenLimit
	}

	return config
}

// createConfig applies LLM options to the default config
func (b *Bedrock) createConfig(opts []llm.LanguageModelOption) llm.LanguageModelConfig {
	cfg := b.GetDefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// convertToBedrockMessages converts conversation posts to Bedrock Converse API format
func convertToBedrockMessages(posts []llm.Post) ([]types.Message, string) {
	var systemContent string
	messages := make([]types.Message, 0, len(posts))

	for _, post := range posts {
		// Skip empty messages
		if post.Message == "" && len(post.Files) == 0 {
			continue
		}
		
		switch post.Role {
		case llm.PostRoleSystem:
			systemContent += post.Message
			continue
		case llm.PostRoleBot:
			// Convert bot/assistant messages
			// Ensure message is not empty to avoid API validation error
			if post.Message != "" {
				messages = append(messages, types.Message{
					Role: types.ConversationRoleAssistant,
					Content: []types.ContentBlock{
						&types.ContentBlockMemberText{
							Value: post.Message,
						},
					},
				})
			}
		case llm.PostRoleUser:
			// Skip if message is empty and has no files
			if post.Message == "" && len(post.Files) == 0 {
				continue
			}
			
			// Ensure we have a non-empty message for the content block
			messageText := post.Message
			if messageText == "" {
				messageText = "Please analyze the attached content."
			}
			
			contentBlocks := []types.ContentBlock{
				&types.ContentBlockMemberText{
					Value: messageText,
				},
			}

			// TODO: Add image and document handling if needed
			// if len(post.Files) > 0 {
			//     for _, file := range post.Files {
			//         // Process files (images, documents, etc.)
			//     }
			// }

			messages = append(messages, types.Message{
				Role:    types.ConversationRoleUser,
				Content: contentBlocks,
			})
		}
	}

	return messages, systemContent
}

// streamResultToChannels handles streaming responses from Bedrock using the Converse API
func (b *Bedrock) streamResultToChannels(model string, bedrockMessages []types.Message, systemContent string, maxTokens int, llmContext *llm.Context, output chan<- string, errChan chan<- error) {
	// Prepare SystemContentBlock if system content is provided
	var systemBlocks []types.SystemContentBlock
	if systemContent != "" {
		systemBlocks = []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{
				Value: systemContent,
			},
		}
	}

	// Create inference config
	inferenceConfig := &types.InferenceConfiguration{
		MaxTokens:   aws.Int32(int32(maxTokens)),
		Temperature: aws.Float32(0.7), // Default temperature
		TopP:        aws.Float32(0.9), // Default top_p
	}

	// Create streaming request
	streamingInputParams := &bedrockruntime.ConverseStreamInput{
		ModelId:         aws.String(model),
		Messages:        bedrockMessages,
		System:          systemBlocks,
		InferenceConfig: inferenceConfig,
	}

	// Invoke model streaming with Converse API
	resp, err := b.client.ConverseStream(context.Background(), streamingInputParams)
	if err != nil {
		errChan <- fmt.Errorf("failed to invoke model: %w", err)
		return
	}

	// Process streaming response
	eventStream := resp.GetStream()
	eventsChan := eventStream.Events()

	for event := range eventsChan {
		switch v := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			// Extract text from the delta
			switch delta := v.Value.Delta.(type) {
			case *types.ContentBlockDeltaMemberText:
				output <- delta.Value
			}
		case *types.ConverseStreamOutputMemberContentBlockStart:
			// Content block started, nothing to output
		case *types.ConverseStreamOutputMemberContentBlockStop:
			// Content block completed, nothing special to do
		case *types.ConverseStreamOutputMemberMessageStart:
			// Message started, nothing to output
		case *types.ConverseStreamOutputMemberMessageStop:
			// Message completed
			// For tool usage, we would check for specific stop reasons
			// Currently, there's no standard constant for tool usage in the types package
			stopReason := string(v.Value.StopReason)
			if stopReason == "tool_use" || strings.Contains(stopReason, "tool") {
				errChan <- errors.New("tool use not yet implemented for Bedrock")
				return
			}
			// Message complete, nothing else to do
			return
		case *types.ConverseStreamOutputMemberMetadata:
			// Metadata event, nothing to output
		default:
			// Unknown event type, log but continue
			fmt.Printf("Unhandled event type: %T\n", v)
		}
	}

	// Check if there was an error in the stream
	if err := eventStream.Err(); err != nil {
		errChan <- fmt.Errorf("stream error: %w", err)
	}
}

// streamResult sets up a streaming response for Bedrock
func (b *Bedrock) streamResult(model string, bedrockMessages []types.Message, systemContent string, maxTokens int, llmContext *llm.Context) (*llm.TextStreamResult, error) {
	output := make(chan string)
	errChan := make(chan error)

	go func() {
		defer close(output)
		defer close(errChan)
		b.streamResultToChannels(model, bedrockMessages, systemContent, maxTokens, llmContext, output, errChan)
	}()

	return &llm.TextStreamResult{Stream: output, Err: errChan}, nil
}

// ChatCompletion implements the LanguageModel interface for streaming chat completion
func (b *Bedrock) ChatCompletion(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	b.metricsService.IncrementLLMRequests()

	cfg := b.createConfig(opts)
	bedrockMessages, systemContent := convertToBedrockMessages(request.Posts)

	return b.streamResult(cfg.Model, bedrockMessages, systemContent, cfg.MaxGeneratedTokens, request.Context)
}

// ChatCompletionNoStream implements the LanguageModel interface for non-streaming chat completion
func (b *Bedrock) ChatCompletionNoStream(request llm.CompletionRequest, opts ...llm.LanguageModelOption) (string, error) {
	// Use streaming implementation and collect the results
	result, err := b.ChatCompletion(request, opts...)
	if err != nil {
		return "", err
	}

	return result.ReadAll(), nil
}

// CountTokens returns an approximated token count
func (b *Bedrock) CountTokens(text string) int {
	// Simple approximation, actual token counting depends on the model
	charCount := float64(len(text)) / 4.0
	wordCount := float64(len(strings.Fields(text))) / 0.75

	// Average the two
	return int((charCount + wordCount) / 2.0)
}

// InputTokenLimit returns the input token limit for the model
func (b *Bedrock) InputTokenLimit() int {
	if b.config.InputTokenLimit > 0 {
		return b.config.InputTokenLimit
	}

	// Default limits based on model
	if strings.Contains(b.config.DefaultModel, "claude-3-opus") {
		return 200000
	} else if strings.Contains(b.config.DefaultModel, "claude-3-sonnet") {
		return 180000
	} else if strings.Contains(b.config.DefaultModel, "claude-3-haiku") {
		return 150000
	} else if strings.Contains(b.config.DefaultModel, "claude-2") {
		return 100000
	} else if strings.Contains(b.config.DefaultModel, "claude-instant") {
		return 100000
	} else if strings.Contains(b.config.DefaultModel, "titan") {
		return 32000
	}

	return 100000 // Default fallback
}

