package anthropic

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/pkg/errors"
	"github.com/r3labs/sse/v2"
)

const (
	MessageEndpoint = "https://api.anthropic.com/v1/messages"
	APIKeyHeader    = "X-API-Key" //nolint:gosec

	StopReasonStopSequence = "stop_sequence"
	StopReasonMaxTokens    = "max_tokens"
)

const RoleUser = "user"
const RoleAssistant = "assistant"

type InputMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestMetadata struct {
	UserID string `json:"user_id"`
}

type MessageRequest struct {
	Model     string          `json:"model"`
	Messages  []InputMessage  `json:"messages"`
	System    string          `json:"system"`
	MaxTokens int             `json:"max_tokens"`
	Metadata  RequestMetadata `json:"metadata"`
	Stream    bool            `json:"stream"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type OutputMessage struct {
	ID         string    `json:"id"`
	Content    []Content `json:"content"`
	StopReason string    `json:"stop_reason"`
	Usage      Usage     `json:"usage"`
}

type StreamDelta struct {
	Type       string `json:"type"`
	Text       string `json:"text"`
	StopReason string `json:"stop_reason"`
	Usage      Usage  `json:"usage"`
}

type MessageStreamEvent struct {
	Type         string `json:"type"`
	Message      OutputMessage
	Index        int         `json:"index"`
	ContentBlock StreamDelta `json:"content_block"`
	Delta        StreamDelta `json:"delta"`
}

type Client struct {
	apiKey     string
	httpClient http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: http.Client{},
	}
}

func (c *Client) MessageCompletionNoStream(completionRequest MessageRequest) (string, error) {
	reqBodyBytes, err := json.Marshal(completionRequest)
	if err != nil {
		return "", errors.Wrap(err, "could not marshal completion request")
	}

	req, err := http.NewRequest("POST", MessageEndpoint, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return "", errors.Wrap(err, "could not create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "could not send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", errors.Wrap(err, "unable to read response body on error: "+resp.Status)
		}

		return "", errors.New("non 200 response from anthropic: " + resp.Status + "\nBody:\n" + string(body))
	}

	outputMessage := OutputMessage{}
	if err := json.NewDecoder(resp.Body).Decode(&outputMessage); err != nil {
		return "", errors.Wrap(err, "couldn't unmarshal response body")
	}

	return outputMessage.Content[0].Text, nil
}

func (c *Client) MessageCompletion(completionRequest MessageRequest) (*ai.TextStreamResult, error) {
	reqBodyBytes, err := json.Marshal(completionRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", MessageEndpoint, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("anthropic-version", "2023-06-01")

	output := make(chan string)
	errChan := make(chan error)
	go func() {
		defer close(output)
		defer close(errChan)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				errChan <- errors.Wrap(err, "unable to read response body on error: "+resp.Status)
				return
			}

			errChan <- errors.New("non 200 response from anthropic: " + resp.Status + "\nBody:\n" + string(body))
			return
		}

		reader := sse.NewEventStreamReader(resp.Body, 65536)
		for {
			nextEvent, err := reader.ReadEvent()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				errChan <- errors.Wrap(err, "error while reading event")
				return
			}

			var nextData []byte
			for _, line := range bytes.FieldsFunc(nextEvent, func(r rune) bool { return r == '\n' || r == '\r' }) {
				if result, isData := bytes.CutPrefix(line, []byte("data: ")); isData {
					nextData = result
				}
			}

			messageStreamEvent := MessageStreamEvent{}
			if err := json.Unmarshal(nextData, &messageStreamEvent); err != nil {
				errChan <- errors.Wrap(err, "couldn't unmarshal data block")
				return
			}

			if messageStreamEvent.Type == "content_block_delta" {
				// Handle future anthropic changes
				if messageStreamEvent.Index != 0 {
					continue
				}
				output <- messageStreamEvent.Delta.Text
			} else if messageStreamEvent.Type == "message_stop" {
				return
			}
		}
	}()

	return &ai.TextStreamResult{Stream: output, Err: errChan}, nil
}
