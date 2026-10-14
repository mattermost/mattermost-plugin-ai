// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

func TestNewClientRecordsServerProfile(t *testing.T) {
	tests := []struct {
		name             string
		clientID         string
		wantInstructions string
	}{
		{
			name:             "shared catalog server keeps instructions",
			wantInstructions: "Use search_issues before create_issue.",
		},
		{
			name:             "static credentials server drops instructions",
			clientID:         "static-client",
			wantInstructions: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{
				Name:        "jira-mcp",
				Title:       "Jira\nCloud",
				Description: "Issues and sprints",
				Version:     "1.0.0",
			}, &mcp.ServerOptions{Instructions: "Use search_issues\nbefore create_issue."})
			addTestMCPTool(server, "search_issues")
			httpServer := startStreamableMCPServer(t, server)
			cache := newTestToolsCache()

			client, err := NewClient(context.Background(), "user-id", ServerConfig{
				Name:     "Jira",
				BaseURL:  httpServer.URL,
				Enabled:  true,
				ClientID: tt.clientID,
			}, newTestLogService(), newTestOAuthManager(), httpServer.Client(), cache, false)
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })

			profile := cache.GetServerProfile("Jira", httpServer.URL)
			require.NotNil(t, profile)
			assert.Equal(t, "jira-mcp", profile.Name)
			assert.Equal(t, "Jira Cloud", profile.Title)
			assert.Equal(t, "Issues and sprints", profile.Description)
			assert.Equal(t, tt.wantInstructions, profile.Instructions)

			assert.Nil(t, cache.GetServerProfile("Jira", "https://elsewhere.example.com"),
				"a profile recorded for another URL must not describe this server")
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
		ServerURL:    jiraURL,
		Name:         "jira-mcp",
		Title:        "Jira Cloud",
		Description:  "Issues and sprints",
		Instructions: "Search before creating.",
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
				Instructions: "Search before creating.",
				Tools: []llm.ToolInfo{
					{Name: "create_issue", Description: "Create an issue", ServerOrigin: jiraURL},
					{Name: "search", Description: "Find Jira issues", ServerOrigin: jiraURL},
				},
			}},
		},
		{
			name:     "static credentials servers list admin-configured tools, not the shared catalog",
			server:   ServerConfig{Name: "Jira", BaseURL: jiraURL, Enabled: true, ClientID: "static", ToolConfigs: toolConfigs},
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
