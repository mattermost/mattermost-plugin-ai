package superface

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func (s *Superface) Resolver(functionName string, context ai.ConversationContext, argsGetter ai.ToolArgumentGetter) (string, error) {
	var args any
	performResolverErr := argsGetter(&args)
	if performResolverErr != nil {
		return "", performResolverErr
	}
	toolPerformResult, performResolverErr := s.Perform(context.RequestingUser.Id, functionName, args)
	if performResolverErr != nil {
		return "", performResolverErr
	}
	return toolPerformResult.ToString()
}

func (s *Superface) ListTools(userID string) ([]ai.Tool, error) {
	result := []FunctionResponse{}
	resp, err := s.do(http.MethodGet, "/api/hub/fd", userID, nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	superfaceTools := []ai.Tool{}
	for _, tool := range result {
		superfaceTool := tool.Function.ToMattermostAITool()
		superfaceTool.Resolver = s.Resolver
		superfaceTools = append(superfaceTools, superfaceTool)
	}

	return superfaceTools, nil
}

func (s *Superface) Perform(userID, functionName string, arguments any) (*PerformResponse, error) {
	var result PerformResponse
	resp, err := s.do(http.MethodPost, fmt.Sprintf("/api/hub/perform/%s", functionName), userID, arguments)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Superface) do(method, path, userID string, body interface{}) (*http.Response, error) {
	var req *http.Request
	fullPath := s.superfaceURL + path
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyBuffer := bytes.NewBuffer(jsonBody)

		req, err = http.NewRequest(method, fullPath, bodyBuffer)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		req, err = http.NewRequest(method, fullPath, nil)
		if err != nil {
			return nil, err
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))
	req.Header.Set("x-superface-user-id", "abc123abc123nmisasixyz")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
