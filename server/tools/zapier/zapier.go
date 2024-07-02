package zapier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	// "github.com/sashabaranov/go-openai/jsonschema"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
)

type Zapier struct {
	zapierURL  string
	authToken  string
	HTTPClient *http.Client
}

func New(url, authToken string) *Zapier {
	return &Zapier{
		zapierURL:  url,
		authToken:  authToken,
		HTTPClient: &http.Client{},
	}
}

func (z *Zapier) Resolver(functionName string, context ai.ConversationContext, argsGetter ai.ToolArgumentGetter) (string, error) {
	var args any
	performResolverErr := argsGetter(&args)
	if performResolverErr != nil {
		return "", performResolverErr
	}
	if performResolverErr != nil {
		return "", performResolverErr
	}
	toolPerformResult, performResolverErr := z.Perform(context.RequestingUser.Id, functionName, args)
	if performResolverErr != nil {
		return "", performResolverErr
	}
	return toolPerformResult.ToString()
}

func (z *Zapier) ListTools(userID string) ([]ai.Tool, error) {
	result := ExposedFunctionsResponse{}
	resp, err := z.do(http.MethodGet, "/api/v1/exposed", userID, nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	zapierTools := []ai.Tool{}
	for _, tool := range result.Results {
		zapierTool := tool.ToMattermostAITool()
		zapierTool.Resolver = z.Resolver
		zapierTool.Schema = tool.Params.ToExposedFunctionParams()
		zapierTools = append(zapierTools, zapierTool)
	}
	return zapierTools, nil
}

func (z *Zapier) Perform(userID, functionName string, arguments any) (*ExecuteResponse, error) {
	var result ExecuteResponse
	resp, err := z.do(http.MethodPost, fmt.Sprintf("/api/v1/exposed/%s/execute", functionName), userID, arguments)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (z *Zapier) do(method, path, userID string, body interface{}) (*http.Response, error) {
	var req *http.Request
	fullPath := z.zapierURL + path
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
	req.Header.Set("X-API-Key", z.authToken)

	resp, err := z.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
