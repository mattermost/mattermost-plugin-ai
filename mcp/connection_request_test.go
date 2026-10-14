// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

const (
	jiraTestOrigin   = "https://jira.example.com/mcp"
	githubTestOrigin = "https://github.example.com/mcp"
)

func TestRequestConnectionToolValidateArguments(t *testing.T) {
	tool := NewRequestConnectionTool([]llm.UnconnectedMCPServer{
		{Name: "Jira", ServerOrigin: jiraTestOrigin},
		{Name: "GitHub", ServerOrigin: githubTestOrigin},
	})
	require.Equal(t, llm.UserInteractionConnect, tool.UserInteraction)

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "listed server", input: `{"server":"Jira","reason":"Look up your issues"}`},
		{name: "listed server without a reason", input: `{"server":"GitHub"}`},
		{name: "server that is not offered", input: `{"server":"Salesforce","reason":"x"}`, wantErr: `"Salesforce" is not an integration the user can connect`},
		{name: "empty server", input: `{"server":"  ","reason":"x"}`, wantErr: "server must not be empty"},
		{name: "malformed arguments", input: `{"server":`, wantErr: "failed to parse connection request arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.ValidateArguments(json.RawMessage(tt.input))
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestRequestConnectionToolSchemaListsOnlyOfferedServers(t *testing.T) {
	tool := NewRequestConnectionTool([]llm.UnconnectedMCPServer{{Name: "Jira"}, {Name: "GitHub"}})

	raw, err := json.Marshal(tool.Schema)
	require.NoError(t, err)
	var schema struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	assert.Equal(t, []string{"Jira", "GitHub"}, schema.Properties["server"].Enum)

	// A second tool must not inherit the first tool's servers.
	other := NewRequestConnectionTool([]llm.UnconnectedMCPServer{{Name: "Linear"}})
	raw, err = json.Marshal(other.Schema)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &schema))
	assert.Equal(t, []string{"Linear"}, schema.Properties["server"].Enum)
}

func TestResolveConnectionRequest(t *testing.T) {
	origins := map[string]string{"Jira": jiraTestOrigin, "GitHub": githubTestOrigin}
	jiraTools := []llm.Tool{
		{Name: "jira__search", ServerOrigin: jiraTestOrigin},
		{Name: "jira__create_issue", ServerOrigin: jiraTestOrigin},
	}
	githubTools := []llm.Tool{{Name: "github__list_prs", ServerOrigin: githubTestOrigin}}

	tests := []struct {
		name          string
		loaded        []llm.Tool
		unloaded      []llm.Tool
		authErrors    []llm.ToolAuthError
		dynamic       bool
		wantConnected bool
		wantTools     []string
		wantInMessage string
	}{
		{
			name:          "connected server reports its tools",
			loaded:        append(append([]llm.Tool{}, jiraTools...), githubTools...),
			wantConnected: true,
			wantTools:     []string{"jira__create_issue", "jira__search"},
			wantInMessage: "The user connected Jira.",
		},
		{
			name:          "dynamic loading counts unloaded tools and points at load_tool",
			unloaded:      jiraTools,
			dynamic:       true,
			wantConnected: true,
			wantTools:     []string{"jira__create_issue", "jira__search"},
			wantInMessage: "Load a tool with load_tool before calling it.",
		},
		{
			name:          "server still needing OAuth is not connected",
			loaded:        githubTools,
			authErrors:    []llm.ToolAuthError{{ServerName: "Jira", ServerOrigin: jiraTestOrigin}},
			wantInMessage: "Jira is still not connected",
		},
		{
			name:          "server with no tools in the catalog is not connected",
			loaded:        githubTools,
			wantInMessage: "Jira is still not connected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := llm.NewToolStore()
			store.AddTools(tt.loaded)
			store.SetUnloadedMCPTools(tt.unloaded)
			for _, authErr := range tt.authErrors {
				store.AddAuthError(authErr)
			}
			llmContext := &llm.Context{
				Tools: store,
				ToolCatalog: llm.ToolCatalogContext{
					MCPDynamicToolLoading: tt.dynamic,
					MCPServerOrigins:      origins,
				},
			}

			raw, err := ResolveConnectionRequest(json.RawMessage(`{"server":"Jira","reason":"Find issues"}`), llmContext)
			require.NoError(t, err)

			var result RequestConnectionResult
			require.NoError(t, json.Unmarshal([]byte(raw), &result))
			assert.Equal(t, "Jira", result.Server)
			assert.Equal(t, tt.wantConnected, result.Connected)
			assert.Equal(t, tt.wantTools, result.Tools)
			assert.Contains(t, result.Message, tt.wantInMessage)
		})
	}
}

func TestConnectionRequestDeclinedResult(t *testing.T) {
	assert.Contains(t, ConnectionRequestDeclinedResult(json.RawMessage(`{"server":"Jira"}`)), "chose not to connect Jira")
	assert.Contains(t, ConnectionRequestDeclinedResult(json.RawMessage(`not json`)), "chose not to connect the integration")
}
