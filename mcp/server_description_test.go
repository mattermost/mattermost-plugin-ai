// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

func TestNewClientRecordsServerProfile(t *testing.T) {
	const instructions = "Connected as alice@example.com; default project SECRET."

	tests := []struct {
		name            string
		clientID        string
		serviceAccount  bool
		rejectToolsList bool
		wantProfile     bool
	}{
		{name: "shared catalog server records its identity", wantProfile: true},
		{name: "static credentials server records nothing", clientID: "static-client"},
		{name: "service account connection records nothing", serviceAccount: true},
		{name: "unauthenticated initialize rejected at tools/list records nothing", rejectToolsList: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{
				Name:        "jira-mcp",
				Title:       "Jira\nCloud",
				Description: "Issues and sprints",
				Version:     "1.0.0",
			}, &mcp.ServerOptions{Instructions: instructions})
			addTestMCPTool(server, "search_issues")
			streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				if tt.rejectToolsList && bytes.Contains(body, []byte(`"method":"`+listToolsMethod+`"`)) {
					w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				streamable.ServeHTTP(w, r)
			}))
			t.Cleanup(httpServer.Close)

			kv := newMockKVService()
			cache := NewToolsCache(kv, &mockLogService{})
			params := clientParams{
				log:            newTestLogService(),
				httpClient:     httpServer.Client(),
				toolsCache:     cache,
				serviceAccount: tt.serviceAccount,
			}
			if !tt.serviceAccount {
				params.oauthManager = newTestOAuthManager()
			}
			client, err := newClient(context.Background(), "user-id", ServerConfig{
				Name:     "Jira",
				BaseURL:  httpServer.URL,
				Enabled:  true,
				ClientID: tt.clientID,
			}, params)
			if tt.rejectToolsList {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				t.Cleanup(func() { _ = client.Close() })
			}

			for key, raw := range kv.store {
				assert.NotContains(t, string(raw), "alice@example.com", "initialize instructions must never be stored (key %s)", key)
			}

			profile := cache.GetServerProfile("Jira", httpServer.URL)
			if !tt.wantProfile {
				assert.Nil(t, profile)
				return
			}
			require.NotNil(t, profile)
			assert.Equal(t, "jira-mcp", profile.Name)
			assert.Equal(t, "Jira Cloud", profile.Title)
			assert.Equal(t, "Issues and sprints", profile.Description)
			assert.Nil(t, cache.GetServerProfile("Jira", "https://elsewhere.example.com"),
				"a profile recorded for another URL must not describe this server")
		})
	}
}

func TestServerProfileInvalidation(t *testing.T) {
	const url = "https://jira.example.com/mcp"
	tests := []struct {
		name       string
		invalidate func(t *testing.T, cache *ToolsCache, manager *ClientManager)
	}{
		{
			name: "refreshing tool providers drops profiles",
			invalidate: func(t *testing.T, _ *ToolsCache, manager *ClientManager) {
				require.NoError(t, manager.invalidateSharedToolsCacheForRefresh())
			},
		},
		{
			name: "clearing the tools cache drops profiles",
			invalidate: func(t *testing.T, cache *ToolsCache, _ *ClientManager) {
				_, err := cache.ClearAll()
				require.NoError(t, err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv := newMockKVService()
			cache := NewToolsCache(kv, &mockLogService{})
			require.NoError(t, cache.SetServerProfile("Jira", ServerProfile{ServerURL: url, Title: "Jira Cloud"}, time.Now()))
			_, err := kv.Set("unrelated_key", "keep")
			require.NoError(t, err)
			manager := &ClientManager{
				config:     Config{Enabled: true, Servers: []ServerConfig{{Name: "Jira", BaseURL: url, Enabled: true, ClientID: "static"}}},
				toolsCache: cache,
			}

			tt.invalidate(t, cache, manager)
			assert.Nil(t, cache.GetServerProfile("Jira", url))
			assert.Contains(t, kv.store, "unrelated_key")
		})
	}
}

func TestDescribeUnconnectedMCPServers(t *testing.T) {
	const jiraURL = "https://jira.example.com/mcp"

	cachedTools := map[string]*mcp.Tool{
		"create_issue": {Name: "create_issue", Description: "Create an issue"},
		"delete_issue": {Name: "delete_issue", Description: "Delete an issue"},
		"search":       {Name: "search", Description: "Search\nissues"},
	}
	profile := ServerProfile{
		ServerURL:   jiraURL,
		Name:        "jira-mcp",
		Title:       "Jira Cloud",
		Description: "Issues and sprints",
	}
	toolConfigs := []ToolConfig{
		{Name: "delete_issue", Policy: ToolPolicyAsk, Enabled: false},
		{Name: "search", Policy: ToolPolicyAsk, Enabled: true, RetrievalDescriptionOverride: "Find Jira issues"},
	}

	tests := []struct {
		name       string
		server     ServerConfig
		profile    *ServerProfile
		cacheURL   string
		authErrors []llm.ToolAuthError
		want       []llm.UnconnectedMCPServer
	}{
		{
			name:     "uses the server profile and shared catalog without disabled tools",
			server:   ServerConfig{Name: "Jira", BaseURL: jiraURL, Enabled: true, ToolConfigs: toolConfigs},
			profile:  &profile,
			cacheURL: jiraURL,
			want: []llm.UnconnectedMCPServer{{
				Name:         "Jira",
				ServerOrigin: jiraURL,
				Title:        "Jira Cloud",
				Description:  "Issues and sprints",
				Tools: []llm.ToolInfo{
					{Name: "create_issue", Description: "Create an issue", ServerOrigin: jiraURL},
					{Name: "search", Description: "Find Jira issues", ServerOrigin: jiraURL},
				},
			}},
		},
		{
			name:     "static credentials servers list admin-configured tools and ignore any stored profile or catalog",
			server:   ServerConfig{Name: "Jira", BaseURL: jiraURL, Enabled: true, ClientID: "static", ToolConfigs: toolConfigs},
			profile:  &profile,
			cacheURL: jiraURL,
			want: []llm.UnconnectedMCPServer{{
				Name:         "Jira",
				ServerOrigin: jiraURL,
				Tools:        []llm.ToolInfo{{Name: "search", Description: "Find Jira issues", ServerOrigin: jiraURL}},
			}},
		},
		{
			name:     "ignores profile and catalog recorded for another URL",
			server:   ServerConfig{Name: "Jira", BaseURL: jiraURL, Enabled: true},
			profile:  &ServerProfile{ServerURL: "https://old.example.com", Title: "Old"},
			cacheURL: "https://old.example.com",
			want:     []llm.UnconnectedMCPServer{{Name: "Jira", ServerOrigin: jiraURL}},
		},
		{
			name:    "drops a reported title that repeats the admin name",
			server:  ServerConfig{Name: "Jira", BaseURL: jiraURL, Enabled: true},
			profile: &ServerProfile{ServerURL: jiraURL, Name: "jira"},
			want:    []llm.UnconnectedMCPServer{{Name: "Jira", ServerOrigin: jiraURL}},
		},
		{
			name:   "deduplicates servers and keeps ones missing from config by name",
			server: ServerConfig{Name: "Jira", BaseURL: jiraURL, Enabled: true},
			authErrors: []llm.ToolAuthError{
				{ServerName: "Jira", ServerOrigin: jiraURL},
				{ServerName: "Jira", ServerOrigin: jiraURL},
				{ServerName: "Removed", ServerOrigin: "https://removed.example.com"},
			},
			want: []llm.UnconnectedMCPServer{
				{Name: "Jira", ServerOrigin: jiraURL},
				{Name: "Removed", ServerOrigin: "https://removed.example.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newTestToolsCache()
			if tt.profile != nil {
				require.NoError(t, cache.SetServerProfile(tt.server.Name, *tt.profile, time.Now()))
			}
			if tt.cacheURL != "" {
				require.NoError(t, cache.SetTools(tt.server.Name, tt.server.Name, tt.cacheURL, cachedTools, time.Now()))
			}
			manager := &ClientManager{
				config:     Config{Enabled: true, Servers: []ServerConfig{tt.server}},
				toolsCache: cache,
			}

			authErrors := tt.authErrors
			if authErrors == nil {
				authErrors = []llm.ToolAuthError{{ServerName: tt.server.Name, ServerOrigin: tt.server.BaseURL}}
			}
			assert.Equal(t, tt.want, manager.DescribeUnconnectedMCPServers(authErrors))
		})
	}
}

func TestSetServerProfileSkipsUnchangedRecentWrites(t *testing.T) {
	kv := newMockKVService()
	cache := NewToolsCache(kv, &mockLogService{})
	profile := ServerProfile{ServerURL: "https://jira.example.com", Title: "Jira"}
	start := time.Now()

	tests := []struct {
		name          string
		profile       ServerProfile
		at            time.Time
		wantTimestamp time.Time
	}{
		{name: "first write", profile: profile, at: start, wantTimestamp: start},
		{name: "unchanged within the refresh age", profile: profile, at: start.Add(time.Hour), wantTimestamp: start},
		{name: "unchanged after the refresh age", profile: profile, at: start.Add(25 * time.Hour), wantTimestamp: start.Add(25 * time.Hour)},
		{name: "changed content", profile: ServerProfile{ServerURL: profile.ServerURL, Title: "Jira Cloud"}, at: start.Add(26 * time.Hour), wantTimestamp: start.Add(26 * time.Hour)},
	}

	// Steps build on each other, so they run in order on one cache.
	for _, tt := range tests {
		require.NoError(t, cache.SetServerProfile("Jira", tt.profile, tt.at), tt.name)
		stored := cache.GetServerProfile("Jira", profile.ServerURL)
		require.NotNil(t, stored, tt.name)
		assert.True(t, tt.wantTimestamp.Equal(stored.Timestamp), tt.name)
		assert.Equal(t, tt.profile.Title, stored.Title, tt.name)
	}
}

func TestPromptSafeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxRunes int
		want     string
	}{
		{name: "collapses newlines and runs of spaces", input: "  Search\n\n  issues\tfast ", maxRunes: 100, want: "Search issues fast"},
		{name: "escapes invisible spoofing characters", input: "Jira\u202eCloud", maxRunes: 100, want: "Jira[U+202E]Cloud"},
		{name: "truncates long text with an ellipsis", input: "abcdefghij", maxRunes: 5, want: "abcd…"},
		{name: "keeps text at the limit", input: "abcde", maxRunes: 5, want: "abcde"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, promptSafeText(tt.input, tt.maxRunes))
		})
	}
}

func TestKnownServerToolsCapsListSize(t *testing.T) {
	const url = "https://big.example.com"
	tools := make(map[string]*mcp.Tool, maxKnownToolsPerServer+10)
	for i := range maxKnownToolsPerServer + 10 {
		name := "tool_" + strings.Repeat("x", i)
		tools[name] = &mcp.Tool{Name: name}
	}
	cache := newTestToolsCache()
	require.NoError(t, cache.SetTools("Big", "Big", url, tools, time.Now()))

	got := knownServerTools(&ServerConfig{Name: "Big", BaseURL: url, Enabled: true}, cache)
	assert.Len(t, got, maxKnownToolsPerServer)
}
