// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mmtools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

func testUnconnectedServers() []llm.UnconnectedMCPServer {
	return []llm.UnconnectedMCPServer{
		{Name: "Jira Cloud", Origin: "jira", Description: "Issues", AuthURL: "https://example.com/jira/start", ToolNames: []string{"create_issue"}},
		{Name: "Notes", Origin: "notes", AuthURL: "https://example.com/notes/start"},
	}
}

func TestConnectMCPServerToolValidation(t *testing.T) {
	tool := NewConnectMCPServerTool(testUnconnectedServers())

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "listed server", input: `{"server":"Jira Cloud"}`},
		{name: "second listed server", input: `{"server":"Notes"}`},
		{name: "unlisted server", input: `{"server":"GitHub"}`, wantErr: `server "GitHub" is not an unconnected MCP server; use one of: Jira Cloud, Notes`},
		{name: "name must match exactly", input: `{"server":"jira cloud"}`, wantErr: "is not an unconnected MCP server"},
		{name: "empty server", input: `{"server":" "}`, wantErr: "server must not be empty"},
		{name: "missing server", input: `{}`, wantErr: "server must not be empty"},
		{name: "malformed", input: `{`, wantErr: "failed to parse connect arguments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.ValidateArguments(json.RawMessage(tt.input))
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestConnectMCPServerToolShape(t *testing.T) {
	tool := NewConnectMCPServerTool(testUnconnectedServers())

	assert.Equal(t, "ConnectMCPServer", tool.Name)
	assert.Equal(t, llm.UserInteractionConnect, tool.UserInteraction)
	assert.Equal(t, "connect", tool.UserInteraction)
	assert.False(t, tool.AutoExecute)

	schema, ok := tool.Schema.(*jsonschema.Schema)
	require.True(t, ok)
	server := schema.Properties["server"]
	require.NotNil(t, server)
	assert.Equal(t, []any{"Jira Cloud", "Notes"}, server.Enum)

	assert.Contains(t, tool.Description, "- Jira Cloud: Issues (tools include: create_issue)")
	assert.Contains(t, tool.Description, "- Notes")

	_, err := tool.Resolver(context.Background(), &llm.Context{}, nil)
	require.Error(t, err)
}

func TestResolveConnectMCPServerAnswer(t *testing.T) {
	jiraInput := json.RawMessage(`{"server":"Jira Cloud"}`)
	metaTools := func() *llm.ToolStore {
		store := llm.NewToolStore()
		store.AddTools([]llm.Tool{{
			Name: "search_tools",
			Resolver: func(context.Context, *llm.Context, llm.ToolArgumentGetter) (string, error) {
				return "", nil
			},
		}})
		return store
	}

	tests := []struct {
		name       string
		input      json.RawMessage
		llmContext *llm.Context
		want       string
		wantErr    bool
	}{
		{
			name:       "connected when no longer listed",
			input:      jiraInput,
			llmContext: &llm.Context{Tools: llm.NewToolStore()},
			want:       "User connected Jira Cloud. Its tools are now available.",
		},
		{
			name:  "connected in dynamic mode points at search_tools",
			input: jiraInput,
			llmContext: &llm.Context{
				Tools:       metaTools(),
				ToolCatalog: llm.ToolCatalogContext{MCPDynamicToolLoading: true},
			},
			want: "User connected Jira Cloud. Its tools are now available. Use search_tools to find them and load_tool to load one before calling it.",
		},
		{
			name:  "still not connected when rebuilt context lists it",
			input: jiraInput,
			llmContext: &llm.Context{
				Tools:       llm.NewToolStore(),
				ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: testUnconnectedServers()},
			},
			want: "Jira Cloud is still not connected: the user did not finish connecting it. Do not perform actions that need Jira Cloud. Tell the user they can connect Jira Cloud and ask again.",
		},
		{
			name:  "other servers still unconnected do not matter",
			input: jiraInput,
			llmContext: &llm.Context{
				Tools:       llm.NewToolStore(),
				ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: testUnconnectedServers()[1:]},
			},
			want: "User connected Jira Cloud. Its tools are now available.",
		},
		{name: "requires rebuilt context", input: jiraInput, wantErr: true},
		{name: "invalid arguments", input: json.RawMessage(`{}`), llmContext: &llm.Context{}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveUserInteractionAnswer(llm.UserInteractionConnect, tt.input, UserInteractionAnswer{}, tt.llmContext)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserInteractionSkippedResult(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		input json.RawMessage
		want  string
	}{
		{name: "connect", kind: llm.UserInteractionConnect, input: json.RawMessage(`{"server":"Jira Cloud"}`), want: "User declined to connect Jira Cloud"},
		{name: "connect with bad arguments", kind: llm.UserInteractionConnect, input: json.RawMessage(`{`), want: "User declined to connect the MCP server"},
		{name: "question", kind: llm.UserInteractionSelect, input: json.RawMessage(`{}`), want: "User skipped the question"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UserInteractionSkippedResult(tt.kind, tt.input))
		})
	}
}
