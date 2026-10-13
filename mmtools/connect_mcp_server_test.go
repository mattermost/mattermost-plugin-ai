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
	const jiraOrigin = "https://jira.example.com"
	jiraInput := json.RawMessage(`{"server":"Jira Cloud"}`)
	resolver := func(context.Context, *llm.Context, llm.ToolArgumentGetter) (string, error) { return "", nil }
	origins := func(name string) (string, bool) {
		if name == "Jira Cloud" {
			return jiraOrigin + "/", true
		}
		return "", false
	}
	storeWith := func(tools []llm.Tool, unloaded []llm.Tool) *llm.ToolStore {
		store := llm.NewToolStore()
		store.AddTools(tools)
		store.SetUnloadedMCPTools(unloaded)
		return store
	}
	jiraTool := llm.Tool{Name: "jira__create_issue", ServerOrigin: jiraOrigin, Resolver: resolver}
	otherTool := llm.Tool{Name: "github__create_pr", ServerOrigin: "https://github.example.com", Resolver: resolver}
	searchTools := llm.Tool{Name: "search_tools", Resolver: resolver}

	const (
		connected     = "User connected Jira Cloud. Its tools are now available."
		stillNot      = "Jira Cloud is still not connected: the user did not finish connecting it. Do not perform actions that need Jira Cloud. Tell the user they can connect Jira Cloud and ask again."
		notAvailable  = "Jira Cloud is not available: none of its tools can be used in this conversation. Do not perform actions that need Jira Cloud. Tell the user Jira Cloud is unavailable here."
		dynamicSuffix = " Use search_tools to find them and load_tool to load one before calling it."
	)

	tests := []struct {
		name         string
		input        json.RawMessage
		llmContext   *llm.Context
		serverOrigin MCPServerOriginLookup
		want         UserInteractionResult
		wantErr      bool
	}{
		{
			name:         "connected when the server's tools are visible",
			input:        jiraInput,
			llmContext:   &llm.Context{Tools: storeWith([]llm.Tool{jiraTool}, nil)},
			serverOrigin: origins,
			want:         UserInteractionResult{Content: connected},
		},
		{
			name:  "connected in dynamic mode when the server's tools are in the unloaded catalog",
			input: jiraInput,
			llmContext: &llm.Context{
				Tools:       storeWith([]llm.Tool{searchTools}, []llm.Tool{jiraTool}),
				ToolCatalog: llm.ToolCatalogContext{MCPDynamicToolLoading: true},
			},
			serverOrigin: origins,
			want:         UserInteractionResult{Content: connected + dynamicSuffix},
		},
		{
			name:  "still not connected when the rebuilt context lists it",
			input: jiraInput,
			llmContext: &llm.Context{
				Tools:       storeWith([]llm.Tool{jiraTool}, nil),
				ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: testUnconnectedServers()},
			},
			serverOrigin: origins,
			want:         UserInteractionResult{Content: stillNot, IsError: true},
		},
		{
			name:         "not listed but no tools from its origin is unavailable",
			input:        jiraInput,
			llmContext:   &llm.Context{Tools: storeWith([]llm.Tool{otherTool}, nil)},
			serverOrigin: origins,
			want:         UserInteractionResult{Content: notAvailable, IsError: true},
		},
		{
			name:         "server missing from config is unavailable",
			input:        jiraInput,
			llmContext:   &llm.Context{Tools: storeWith([]llm.Tool{jiraTool}, nil)},
			serverOrigin: func(string) (string, bool) { return "", false },
			want:         UserInteractionResult{Content: notAvailable, IsError: true},
		},
		{
			name:       "no origin lookup is unavailable",
			input:      jiraInput,
			llmContext: &llm.Context{Tools: storeWith([]llm.Tool{jiraTool}, nil)},
			want:       UserInteractionResult{Content: notAvailable, IsError: true},
		},
		{
			name:         "no tool store is unavailable",
			input:        jiraInput,
			llmContext:   &llm.Context{},
			serverOrigin: origins,
			want:         UserInteractionResult{Content: notAvailable, IsError: true},
		},
		{name: "requires rebuilt context", input: jiraInput, serverOrigin: origins, wantErr: true},
		{name: "invalid arguments", input: json.RawMessage(`{}`), llmContext: &llm.Context{}, serverOrigin: origins, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveUserInteractionAnswer(llm.UserInteractionConnect, tt.input, UserInteractionAnswer{}, tt.llmContext, tt.serverOrigin)
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
