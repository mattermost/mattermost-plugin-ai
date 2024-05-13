package n8n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
)

type N8N struct {
	n8nURL     string
	authToken  string
	HTTPClient *http.Client
}

type PerformResponse struct {
	Status              string `json:"status"`
	AssistantHint       string `json:"assistant_hint"`
	Result              any    `json:"result"`
	CopilotSpecificData any    `json:"copilot_specific_data"`
}

func New(url, authToken string) *N8N {
	return &N8N{
		n8nURL:     url,
		authToken:  authToken,
		HTTPClient: &http.Client{},
	}
}

func (n *N8N) Resolver(httpVerb string, functionName string, context ai.ConversationContext, argsGetter ai.ToolArgumentGetter) (string, error) {
	var args any
	performResolverErr := argsGetter(&args)
	if performResolverErr != nil {
		return "", performResolverErr
	}
	toolPerformResult, performResolverErr := n.Perform(httpVerb, context.RequestingUser.Id, functionName, args)
	if performResolverErr != nil {
		return "", performResolverErr
	}
	return toolPerformResult.ToString()
}

func (n *N8N) ListTools(userID string) ([]ai.Tool, error) {
	result := N8NListResponse{}

	resp, err := n.do(http.MethodGet, "/api/v1/workflows?active=true", userID, nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	if len(result.Tools) == 0 {
		return nil, nil
	}

	n8nTools := []ai.Tool{}
	for _, tool := range result.Tools {
		if !tool.Active {
			continue
		}
		n8nTool := tool.ToMattermostAITool()

		n8nTool.Resolver = func(functionName string, context ai.ConversationContext, argsGetter ai.ToolArgumentGetter) (string, error) {
			return n.Resolver(n8nTool.HTTPMethod, functionName, context, argsGetter)
		}
		n8nTools = append(n8nTools, n8nTool)
	}

	return n8nTools, nil
}

func (n *N8N) Perform(httpVerb, userID, functionName string, arguments any) (*PerformResponse, error) {
	var result PerformResponse

	fmt.Println("PERFORMING N8N ACTION", functionName)
	resp, err := n.do(httpVerb, fmt.Sprintf("/webhook/%s", functionName), userID, arguments)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	fmt.Println("N8N RESULT OBJECT:", result)
	result.CopilotSpecificData = nil

	return &result, nil
}

func (s *N8N) do(method, path, userID string, body interface{}) (*http.Response, error) {
	var req *http.Request
	fullPath := s.n8nURL + path
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		fmt.Println("SENDING BODY:", string(jsonBody))
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
	req.Header.Set("X-N8N-API-KEY", s.authToken)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
