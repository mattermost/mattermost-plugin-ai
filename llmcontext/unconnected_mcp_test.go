// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llmcontext

import (
	stdcontext "context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mcp"
)

const (
	unconnectedJiraOrigin   = "https://jira.example.com"
	unconnectedGitHubOrigin = "https://github.example.com"
)

func unconnectedTestAuthErrors() []llm.ToolAuthError {
	return []llm.ToolAuthError{
		{
			ServerName:        "Jira",
			ServerOrigin:      unconnectedJiraOrigin,
			AuthURL:           "https://mm.example.com/plugins/mattermost-ai/mcp/oauth/Jira/start",
			ServerDescription: "Jira issues",
			ToolNames:         []string{"create_issue", "search_issues"},
		},
		{
			ServerName:   "GitHub",
			ServerOrigin: unconnectedGitHubOrigin,
			AuthURL:      "https://mm.example.com/plugins/mattermost-ai/mcp/oauth/GitHub/start",
			ToolNames:    []string{"create_pr"},
		},
	}
}

func unconnectedNames(servers []llm.UnconnectedMCPServer) []string {
	names := make([]string, 0, len(servers))
	for _, server := range servers {
		names = append(names, server.Name)
	}
	return names
}

func TestBuilderAdvertisesUnconnectedMCPServers(t *testing.T) {
	autoEnable := llm.BotConfig{ID: "bot-id", Name: "matty", DisplayName: "Matty", AutoEnableNewMCPTools: true}

	tests := []struct {
		name        string
		botConfig   llm.BotConfig
		opts        func(b *Builder, bot *bots.Bot) []llm.ContextOption
		concrete    bool
		wantServers []string
	}{
		{
			name:      "interactive turn advertises every eligible server",
			botConfig: autoEnable,
			opts: func(b *Builder, _ *bots.Bot) []llm.ContextOption {
				return []llm.ContextOption{b.WithLLMContextInteractive()}
			},
			wantServers: []string{"Jira", "GitHub"},
		},
		{
			name:      "non-interactive flow advertises nothing",
			botConfig: autoEnable,
			opts:      func(*Builder, *bots.Bot) []llm.ContextOption { return nil },
		},
		{
			name: "dynamic tool loading still advertises when no server is connected",
			botConfig: llm.BotConfig{
				ID: "bot-id", Name: "matty", DisplayName: "Matty",
				AutoEnableNewMCPTools: true,
				MCPDynamicToolLoading: true,
			},
			opts: func(b *Builder, _ *bots.Bot) []llm.ContextOption {
				return []llm.ContextOption{b.WithLLMContextInteractive()}
			},
			wantServers: []string{"Jira", "GitHub"},
		},
		{
			name: "agent allowlist excludes servers it does not allow",
			botConfig: llm.BotConfig{
				ID: "bot-id", Name: "matty", DisplayName: "Matty",
				EnabledMCPTools: []llm.EnabledMCPTool{{ServerOrigin: unconnectedJiraOrigin, ToolName: llm.MCPServerToolWildcard}},
			},
			opts: func(b *Builder, _ *bots.Bot) []llm.ContextOption {
				return []llm.ContextOption{b.WithLLMContextInteractive()}
			},
			wantServers: []string{"Jira"},
		},
		{
			name:      "user-disabled origins are not advertised",
			botConfig: autoEnable,
			opts: func(b *Builder, _ *bots.Bot) []llm.ContextOption {
				return []llm.ContextOption{
					b.WithLLMContextInteractive(),
					b.WithLLMContextDisabledMCPServers([]string{unconnectedGitHubOrigin}),
				}
			},
			wantServers: []string{"Jira"},
		},
		{
			name:      "channel tool filter keeps only servers with an allowed configured tool",
			botConfig: autoEnable,
			opts: func(b *Builder, _ *bots.Bot) []llm.ContextOption {
				return []llm.ContextOption{
					b.WithLLMContextInteractive(),
					b.WithLLMContextMCPToolFilter(func(tool llm.Tool) bool { return tool.Name == "create_issue" }),
				}
			},
			wantServers: []string{"Jira"},
		},
		{
			name: "service account agents never advertise",
			botConfig: llm.BotConfig{
				ID: "bot-id", Name: "matty", DisplayName: "Matty",
				AutoEnableNewMCPTools: true,
				UseServiceAccountAuth: true,
			},
			opts: func(b *Builder, _ *bots.Bot) []llm.ContextOption {
				return []llm.ContextOption{b.WithLLMContextInteractive()}
			},
		},
		{
			name:      "bridge concrete catalog never advertises",
			botConfig: autoEnable,
			opts: func(b *Builder, _ *bots.Bot) []llm.ContextOption {
				return []llm.ContextOption{b.WithLLMContextInteractive()}
			},
			concrete: true,
		},
		{
			name: "tools disabled for the agent",
			botConfig: llm.BotConfig{
				ID: "bot-id", Name: "matty", DisplayName: "Matty",
				AutoEnableNewMCPTools: true,
				DisableTools:          true,
			},
			opts: func(b *Builder, _ *bots.Bot) []llm.ContextOption {
				return []llm.ContextOption{b.WithLLMContextInteractive()}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &staticMCPToolProvider{
				errors: &mcp.Errors{ToolAuthErrors: unconnectedTestAuthErrors()},
			}
			builder := newTestBuilder(t, &emptyToolProvider{}, provider)
			bot := newTestBotWithConfig(tt.botConfig)

			opts := tt.opts(builder, bot)
			if tt.concrete {
				opts = append(opts, builder.WithLLMContextConcreteTools(stdcontext.Background(), bot))
			} else {
				opts = append(opts, builder.WithLLMContextTools(stdcontext.Background(), bot))
			}
			c := builder.BuildLLMContextUserRequest(bot, testUser(), testChannel(), opts...)

			if len(tt.wantServers) == 0 {
				assert.Empty(t, c.ToolCatalog.UnconnectedMCPServers)
				if c.Tools != nil {
					assert.Nil(t, c.Tools.GetTool(llm.ConnectMCPServerToolName))
				}
				return
			}

			assert.Equal(t, tt.wantServers, unconnectedNames(c.ToolCatalog.UnconnectedMCPServers))
			connect := c.Tools.GetTool(llm.ConnectMCPServerToolName)
			require.NotNil(t, connect)
			assert.Equal(t, llm.UserInteractionConnect, connect.UserInteraction)
			for _, name := range tt.wantServers {
				assert.Contains(t, connect.Description, "- "+name)
			}
			assert.True(t, c.IsToolCallable(llm.ConnectMCPServerToolName))
		})
	}
}

func TestUnconnectedMCPServersFiltering(t *testing.T) {
	tests := []struct {
		name          string
		authErrors    []llm.ToolAuthError
		botConfig     llm.BotConfig
		catalog       llm.ToolCatalogContext
		wantServers   []string
		wantToolNames map[string][]string
	}{
		{
			name:          "auto-enable keeps all servers and admin tool names",
			authErrors:    unconnectedTestAuthErrors(),
			botConfig:     llm.BotConfig{AutoEnableNewMCPTools: true},
			wantServers:   []string{"Jira", "GitHub"},
			wantToolNames: map[string][]string{"Jira": {"create_issue", "search_issues"}, "GitHub": {"create_pr"}},
		},
		{
			name:       "allowlist narrows advertised tool names",
			authErrors: unconnectedTestAuthErrors(),
			botConfig: llm.BotConfig{EnabledMCPTools: []llm.EnabledMCPTool{
				{ServerOrigin: unconnectedJiraOrigin, ToolName: "search_issues"},
			}},
			wantServers:   []string{"Jira"},
			wantToolNames: map[string][]string{"Jira": {"search_issues"}},
		},
		{
			name:        "disabled origins are normalized before matching",
			authErrors:  unconnectedTestAuthErrors(),
			botConfig:   llm.BotConfig{AutoEnableNewMCPTools: true},
			catalog:     llm.ToolCatalogContext{DisabledMCPServerOrigins: []string{" " + unconnectedJiraOrigin + "/ "}},
			wantServers: []string{"GitHub"},
		},
		{
			name:       "context tool filter drops servers without admin tool names",
			authErrors: []llm.ToolAuthError{{ServerName: "Bare", ServerOrigin: "https://bare.example.com", AuthURL: "https://mm/bare/start"}},
			botConfig:  llm.BotConfig{AutoEnableNewMCPTools: true},
			catalog:    llm.ToolCatalogContext{KeepMCPTool: func(llm.Tool) bool { return true }},
		},
		{
			name:       "no context filter keeps servers without admin tool names",
			authErrors: []llm.ToolAuthError{{ServerName: "Bare", ServerOrigin: "https://bare.example.com", AuthURL: "https://mm/bare/start"}},
			botConfig:  llm.BotConfig{AutoEnableNewMCPTools: true},
			wantServers: []string{
				"Bare",
			},
		},
		{
			name: "duplicate origins and servers without a start URL are skipped",
			authErrors: append(unconnectedTestAuthErrors()[:1],
				llm.ToolAuthError{ServerName: "Jira again", ServerOrigin: unconnectedJiraOrigin, AuthURL: "https://mm/x"},
				llm.ToolAuthError{ServerName: "NoURL", ServerOrigin: "https://nourl.example.com"},
			),
			botConfig:   llm.BotConfig{AutoEnableNewMCPTools: true},
			wantServers: []string{"Jira"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unconnectedMCPServers(tt.authErrors, tt.botConfig, tt.catalog)
			if len(tt.wantServers) == 0 {
				assert.Empty(t, got)
				return
			}
			require.Equal(t, tt.wantServers, unconnectedNames(got))
			for _, server := range got {
				if want, ok := tt.wantToolNames[server.Name]; ok {
					assert.Equal(t, want, server.ToolNames, server.Name)
				}
			}
		})
	}
}

// An MCP grant that lapses mid-turn adds the server to the turn's list and
// catalogs ConnectMCPServer so the model can ask the user to reconnect.
func TestBuilderReconnectHookAddsServerMidTurn(t *testing.T) {
	expired := llm.UnconnectedMCPServer{
		Name:    "Confluence",
		Origin:  "https://confluence.example.com",
		AuthURL: "https://mm.example.com/plugins/mattermost-ai/mcp/oauth/Confluence/start",
	}

	tests := []struct {
		name         string
		authErrors   []llm.ToolAuthError
		interactive  bool
		wantCallable bool
		wantServers  []string
	}{
		{
			name:         "adds connect tool when nothing was unconnected",
			interactive:  true,
			wantCallable: true,
			wantServers:  []string{"Confluence"},
		},
		{
			name:         "extends an existing list",
			authErrors:   unconnectedTestAuthErrors()[:1],
			interactive:  true,
			wantCallable: true,
			wantServers:  []string{"Jira", "Confluence"},
		},
		{
			name: "non-interactive turn has no hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &staticMCPToolProvider{
				tools: []llm.Tool{testMCPTool("confluence__search", expired.Origin, "search pages")},
			}
			if tt.authErrors != nil {
				provider.errors = &mcp.Errors{ToolAuthErrors: tt.authErrors}
			}
			builder := newTestBuilder(t, &emptyToolProvider{}, provider)
			bot := newTestBotWithConfig(llm.BotConfig{ID: "bot-id", Name: "matty", DisplayName: "Matty", AutoEnableNewMCPTools: true})

			var opts []llm.ContextOption
			if tt.interactive {
				opts = append(opts, builder.WithLLMContextInteractive())
			}
			c := buildToolsContext(builder, bot, opts...)

			assert.Equal(t, tt.wantCallable, c.ReportMCPServerNeedsReconnect(expired))
			assert.Equal(t, tt.wantServers, nilIfEmpty(unconnectedNames(c.ToolCatalog.UnconnectedMCPServers)))
			if !tt.wantCallable {
				assert.Nil(t, c.Tools.GetTool(llm.ConnectMCPServerToolName))
				return
			}

			connect := c.Tools.GetTool(llm.ConnectMCPServerToolName)
			require.NotNil(t, connect)
			assert.Contains(t, connect.Description, "- Confluence")
			require.NoError(t, connect.ValidateArguments([]byte(`{"server":"Confluence"}`)))

			// Reporting the same server again is idempotent.
			require.True(t, c.ReportMCPServerNeedsReconnect(expired))
			assert.Equal(t, tt.wantServers, unconnectedNames(c.ToolCatalog.UnconnectedMCPServers))
		})
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
