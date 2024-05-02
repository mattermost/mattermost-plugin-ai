package superface

import (
	"encoding/json"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
)

// FunctionPayload represents a single function entry in the array
type FunctionResponse struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function describes the details of a function
type Function struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Parameters  FunctionParameters `json:"parameters"`
}

// FunctionParameters holds function parameter details
type FunctionParameters struct {
	Type       string               `json:"type"`
	Required   []string             `json:"required"`
	Properties map[string]Parameter `json:"properties"`
	Nullable   bool                 `json:"nullable"`
}

// Parameter describes a single function parameter
type Parameter struct {
	Type        string   `json:"type"`
	Nullable    bool     `json:"nullable"`
	Title       string   `json:"title"`
	Description string   `json:"description"`    // Added for the 'weather' example
	Enum        []string `json:"enum,omitempty"` // For potential enum values
}

type PerformResponse struct {
	Status        string `json:"status"`
	AssistantHint string `json:"assistant_hint"`
	Result        any    `json:"result"`
}

func (f *Function) ToMattermostAITool() ai.Tool {
	return ai.Tool{
		Name:        f.Name,
		Description: f.Description,
		Schema:      f.Parameters,
	}
}

func (pr *PerformResponse) ToString() (string, error) {
	jsonData, err := json.Marshal(pr)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}
