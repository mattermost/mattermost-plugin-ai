package anthropic

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/pkg/errors"
	"github.com/r3labs/sse/v2"
)

const (
	CompletionEndpoint = "https://api.anthropic.com/v1/complete"
	APIKeyHeader       = "X-API-Key"

	StopReasonStopSequence = "stop_sequence"
	StopReasonMaxTokens    = "max_tokens"
)

type CompletionRequest struct {
	Prompt            string `json:"prompt"`
	Model             string `json:"model"`
	MaxTokensToSample int    `json:"max_tokens_to_sample"`
	Stream            bool   `json:"stream"`
}

type CompletionResponse struct {
	Completion string `json:"completion"`
	StopReason string `json:"stop_reason"`
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

func (c *Client) CompletionNoStream(prompt string) (string, error) {
	reqBody := CompletionRequest{
		Prompt:            prompt,
		Model:             "claude-v1",
		MaxTokensToSample: 1000,
		Stream:            false,
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", CompletionEndpoint, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", errors.Wrap(err, "unable to read response body on error: "+resp.Status)
		}

		return "", errors.New("non 200 response from anthropic: " + resp.Status + "\nBody:\n" + string(body))
	}

	completionResponse := CompletionResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&completionResponse); err != nil {
		return "", err
	}

	return completionResponse.Completion, nil
}

func (c *Client) Completion(prompt string) (*ai.TextStreamResult, error) {
	reqBody := CompletionRequest{
		Prompt:            prompt,
		Model:             "claude-v1",
		MaxTokensToSample: 1000,
		Stream:            true,
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", CompletionEndpoint, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

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
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				errChan <- errors.Wrap(err, "unable to read response body on error: "+resp.Status)
				return
			}

			errChan <- errors.New("non 200 response from anthropic: " + resp.Status + "\nBody:\n" + string(body))
			return
		}

		reader := sse.NewEventStreamReader(resp.Body, 65536)

		seen := strings.Builder{}
		for {
			nextEvent, err := reader.ReadEvent()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				errChan <- errors.Wrap(err, "error while reading event")
				return
			}

			nextEvent = bytes.TrimPrefix(nextEvent, []byte("data: "))

			// There is a bunch of other garbage that can be sent. Just skip it.
			if !json.Valid(nextEvent) {
				continue
			}

			completionResponse := CompletionResponse{}
			if err := json.Unmarshal(nextEvent, &completionResponse); err != nil {
				errChan <- errors.Wrap(err, "couldn't unmarshal data block")
				return
			}

			nextString := strings.TrimPrefix(completionResponse.Completion, seen.String())
			output <- nextString
			seen.WriteString(nextString)
		}
	}()

	return &ai.TextStreamResult{Stream: output, Err: errChan}, nil
}
