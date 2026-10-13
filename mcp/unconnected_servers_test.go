// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

func connectCallableStore() *llm.ToolStore {
	store := llm.NewToolStore()
	store.AddTools([]llm.Tool{{
		Name: llm.ConnectMCPServerToolName,
		Resolver: func(context.Context, *llm.Context, llm.ToolArgumentGetter) (string, error) {
			return "", nil
		},
	}})
	return store
}

func TestSearchToolsReportsUnavailableServers(t *testing.T) {
	unconnected := []llm.UnconnectedMCPServer{
		{Name: "Jira", Origin: "https://jira.example.com", Description: "Atlassian issue tracker", AuthURL: "https://mm/jira/start", ToolNames: []string{"create_issue"}},
		{Name: "Calendar", Origin: "https://cal.example.com", Description: "Meetings and events", AuthURL: "https://mm/cal/start"},
	}
	registry := NewToolRegistry([]llm.Tool{testRegistryTool("github__create_pr", "Open a pull request", "https://github.example.com")})

	tests := []struct {
		name       string
		query      string
		llmContext *llm.Context
		want       []SearchToolsUnavailableServer
	}{
		{
			name:  "matches by admin tool name",
			query: "create issue",
			llmContext: &llm.Context{
				Tools:       connectCallableStore(),
				ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: unconnected},
			},
			want: []SearchToolsUnavailableServer{{
				Server: "Jira",
				Status: "needs_authentication",
				Hint:   `The user has not connected Jira. Call ConnectMCPServer with {"server":"Jira"} to ask them to connect it.`,
			}},
		},
		{
			name:  "matches by description",
			query: "meetings",
			llmContext: &llm.Context{
				Tools:       connectCallableStore(),
				ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: unconnected},
			},
			want: []SearchToolsUnavailableServer{{
				Server: "Calendar",
				Status: "needs_authentication",
				Hint:   `The user has not connected Calendar. Call ConnectMCPServer with {"server":"Calendar"} to ask them to connect it.`,
			}},
		},
		{
			name:  "no match",
			query: "weather forecast",
			llmContext: &llm.Context{
				Tools:       connectCallableStore(),
				ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: unconnected},
			},
		},
		{
			name:  "omitted when ConnectMCPServer is not callable",
			query: "create issue",
			llmContext: &llm.Context{
				Tools:       llm.NewToolStore(),
				ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: unconnected},
			},
		},
		{
			name:  "omitted when ConnectMCPServer is disabled in this context",
			query: "create issue",
			llmContext: &llm.Context{
				Tools:             connectCallableStore(),
				DisabledToolsInfo: []llm.ToolInfo{{Name: llm.ConnectMCPServerToolName}},
				ToolCatalog:       llm.ToolCatalogContext{UnconnectedMCPServers: unconnected},
			},
		},
		{
			name:  "omitted for empty query",
			query: " ",
			llmContext: &llm.Context{
				Tools:       connectCallableStore(),
				ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: unconnected},
			},
		},
		{name: "nil context", query: "create issue"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchTool := metaToolByName(t, NewMetaTools(registry), SearchToolsName)
			args, err := json.Marshal(SearchToolsArgs{Query: tt.query})
			require.NoError(t, err)

			raw, err := searchTool.Resolver(context.Background(), tt.llmContext, metaToolArgs(string(args)))
			require.NoError(t, err)

			var result SearchToolsResult
			require.NoError(t, json.Unmarshal([]byte(raw), &result))
			assert.Equal(t, tt.want, result.UnavailableServers)
			if tt.want == nil {
				assert.NotContains(t, raw, "unavailable_servers")
			}
		})
	}
}

// An unconnected server is advertised with admin-authored metadata only. The
// shared tools cache holds whichever catalog the last authenticated user saw,
// so it must not leak into another user's advertisement.
func TestOAuthNeededConnectFailureCarriesAdminMetadataOnly(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(httpServer.Close)

	serverConfig := ServerConfig{
		Name:        "Jira",
		BaseURL:     httpServer.URL,
		Enabled:     true,
		Description: "Jira issues for Engineering",
		ToolConfigs: []ToolConfig{
			{Name: "create_issue", Enabled: true},
			{Name: "delete_project", Enabled: false},
			{Name: "", Enabled: true},
			{Name: "search_issues", Enabled: true},
		},
	}

	toolsCache := newTestToolsCache()
	require.NoError(t, toolsCache.SetTools(serverConfig.Name, serverConfig.Name, serverConfig.BaseURL, map[string]*gomcp.Tool{
		"other_users_private_tool": {Name: "other_users_private_tool", Description: "cached from another user"},
	}, time.Now()))
	require.NotEmpty(t, toolsCache.GetTools(serverConfig.Name))

	uc := NewUserClients("alice", newTestLogService(), newTestOAuthManager(), httpServer.Client(), toolsCache)
	t.Cleanup(uc.Close)

	mcpErrors := uc.ConnectToRemoteServers(context.Background(), []ServerConfig{serverConfig}, false)
	require.NotNil(t, mcpErrors)
	require.Len(t, mcpErrors.ToolAuthErrors, 1)

	authErr := mcpErrors.ToolAuthErrors[0]
	assert.Equal(t, "Jira", authErr.ServerName)
	assert.Equal(t, "Jira issues for Engineering", authErr.ServerDescription)
	assert.Equal(t, []string{"create_issue", "search_issues"}, authErr.ToolNames)
	assert.NotContains(t, authErr.ToolNames, "other_users_private_tool")
	assert.NotEmpty(t, authErr.AuthURL)
}

func TestRememberOAuthNeededForToolCallDetachesSession(t *testing.T) {
	const origin = "https://jira.example.com"
	const authURL = "https://mattermost.example.com/plugins/mattermost-ai/mcp/oauth/Jira/start"

	tests := []struct {
		name         string
		err          error
		withOAuth    bool
		wantAuthURL  string
		wantDetached bool
	}{
		{
			name:         "oauth needed detaches and returns start URL",
			err:          &OAuthNeededError{authURL: authURL},
			withOAuth:    true,
			wantAuthURL:  authURL,
			wantDetached: true,
		},
		{
			name:         "wrapped oauth needed error",
			err:          errors.Join(errors.New("call failed"), &OAuthNeededError{authURL: authURL}),
			withOAuth:    true,
			wantAuthURL:  authURL,
			wantDetached: true,
		},
		{
			name:         "oauth needed without URL falls back to start route",
			err:          &OAuthNeededError{},
			withOAuth:    true,
			wantAuthURL:  "https://mattermost.example.com/plugins/mattermost-ai/mcp/oauth/Jira/start",
			wantDetached: true,
		},
		{
			name:      "ordinary failure keeps the session",
			err:       errors.New("tool exploded"),
			withOAuth: true,
		},
		{
			name: "service account bags have no oauth manager",
			err:  &OAuthNeededError{authURL: authURL},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var oauthManager *OAuthManager
			if tt.withOAuth {
				oauthManager, _ = newStatefulKVManager(t, nil, http.DefaultClient)
				oauthManager.callbackURL = "https://mattermost.example.com/plugins/mattermost-ai/oauth/callback"
			}
			uc := NewUserClients("alice", newTestLogService(), oauthManager, nil, nil)
			client := testClientWithTools("Jira", origin, "create_issue")
			uc.clients["Jira"] = client

			got := uc.rememberOAuthNeededForToolCall(client, tt.err)
			assert.Equal(t, tt.wantAuthURL, got)

			_, stillCached := uc.clients["Jira"]
			assert.Equal(t, !tt.wantDetached, stillCached)

			if tt.wantAuthURL != "" {
				state, err := oauthManager.LoadAuthNeededState("alice", "Jira")
				require.NoError(t, err)
				require.NotNil(t, state)
				assert.Equal(t, tt.wantAuthURL, state.AuthURL)
			}
		})
	}
}

func TestReconnectNeededToolError(t *testing.T) {
	const authURL = "https://mm/jira/start"
	client := testClientWithTools("Jira", "https://jira.example.com", "create_issue")
	client.config.Description = "Jira issues"
	client.config.ToolConfigs = []ToolConfig{{Name: "create_issue", Enabled: true}}
	cause := &OAuthNeededError{authURL: authURL}

	tests := []struct {
		name         string
		llmContext   func() *llm.Context
		wantContains []string
		wantReported bool
	}{
		{
			name: "interactive turn points the model at ConnectMCPServer",
			llmContext: func() *llm.Context {
				c := &llm.Context{Tools: llm.NewToolStore()}
				c.ToolRuntime.MCPServerNeedsReconnect = func(server llm.UnconnectedMCPServer) {
					c.ToolCatalog.UnconnectedMCPServers = append(c.ToolCatalog.UnconnectedMCPServers, server)
					c.Tools.AddTools(connectCallableStore().GetTools())
				}
				return c
			},
			wantContains: []string{
				`MCP server "Jira" needs the user to reconnect`,
				`Call ConnectMCPServer with {"server":"Jira"}`,
				"Do not claim the action was performed",
			},
			wantReported: true,
		},
		{
			name:       "non-interactive turn asks the user to reconnect",
			llmContext: func() *llm.Context { return &llm.Context{} },
			wantContains: []string{
				`MCP server "Jira" needs the user to reconnect`,
				"Ask the user to reconnect Jira and then try again.",
			},
		},
		{
			name:       "nil context",
			llmContext: func() *llm.Context { return nil },
			wantContains: []string{
				"Ask the user to reconnect Jira",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llmContext := tt.llmContext()
			err := reconnectNeededToolError(llmContext, client, authURL, cause)
			require.Error(t, err)
			require.ErrorIs(t, err, cause)
			for _, want := range tt.wantContains {
				assert.Contains(t, err.Error(), want)
			}

			server, reported := llmContext.UnconnectedMCPServerByName("Jira")
			require.Equal(t, tt.wantReported, reported)
			if reported {
				assert.Equal(t, llm.UnconnectedMCPServer{
					Name:        "Jira",
					Origin:      llm.NormalizeMCPServerOrigin("https://jira.example.com"),
					Description: "Jira issues",
					AuthURL:     authURL,
					ToolNames:   []string{"create_issue"},
				}, server)
			}
		})
	}
}
