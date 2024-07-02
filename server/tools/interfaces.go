package tools

import "github.com/mattermost/mattermost-plugin-ai/server/ai"

type ToolGetterConfig struct {
	Provider  string
	URL       string
	AuthToken string
}

type ToolGetter interface {
	ListTools(userID string) ([]ai.Tool, error)
	Perform(userID string, functionName string, arguments any) (any, error)
}
