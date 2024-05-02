package superface

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
)

type Superface struct {
	superfaceURL string
	authToken    string
	HTTPClient   *http.Client
}

func New(superfaceURL, authToken string) *Superface {
	return &Superface{
		superfaceURL: superfaceURL,
		authToken:    authToken,
		HTTPClient:   &http.Client{},
	}
}

func (s *Superface) ListTools(userID string) ([]ai.Tool, error) {
	result := []FunctionResponse{}
	err := s.do(http.MethodGet, "/api/hub/fd", userID, nil, result)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	superfaceTools := []ai.Tool{}
	for _, tool := range result {
		superfaceTool := tool.Function.ToMattermostAITool()
		superfaceTool.Resolver = func(context ai.ConversationContext, argsGetter ai.ToolArgumentGetter) (string, error) {
			var args any
			performResolverErr := argsGetter(&args)
			if performResolverErr != nil {
				return "", performResolverErr
			}
			toolPerformResult, performResolverErr := s.Perform(userID, superfaceTool.Name, args)
			if performResolverErr != nil {
				return "", performResolverErr
			}

			return toolPerformResult.ToString()
		}
		superfaceTools = append(superfaceTools, tool.Function.ToMattermostAITool())
	}

	return superfaceTools, nil
}

func (s *Superface) Perform(userID, functionName string, arguments any) (*PerformResponse, error) {
	var result *PerformResponse
	err := s.do(http.MethodPost, fmt.Sprintf("/api/hub/perform/%s", functionName), userID, arguments, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Superface) do(method, path, userID string, body interface{}, result interface{}) error {
	var req *http.Request
	fullPath := s.superfaceURL + path
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyBuffer := bytes.NewBuffer(jsonBody)

		req, err = http.NewRequest(method, fullPath, bodyBuffer)
		if err != nil {
			return err
		}
	} else {
		var err error
		req, err = http.NewRequest(method, fullPath, nil)
		if err != nil {
			return err
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-tokens", s.authToken)
	req.Header.Set("x-superface-user-id", userID)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("unable to read response body on status %v. Error: %w", resp.Status, err)
		}

		return errors.New("non 200 response from asksage: " + resp.Status + "\nBody:\n" + string(body))
	}

	// Decode response body into specified struct
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return err
	}

	return nil
}
