// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server-supplied text is rendered into the system prompt for users who have
// not connected yet, so every field is flattened to one line and capped.
const (
	maxServerNameRunes         = 100
	maxServerDescriptionRunes  = 300
	maxServerInstructionsRunes = 1000
	maxKnownToolDescRunes      = 200
	maxKnownToolsPerServer     = 30
)

// promptSafeText escapes invisible characters, collapses all whitespace
// (including newlines) to single spaces, and truncates to maxRunes.
func promptSafeText(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(llm.SanitizeNonPrintableChars(s)), " ")
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}

// recordServerProfile stores what the server reported about itself during
// initialize. Instructions can be written for the connected identity, so they
// are only kept where the user-mode tool catalog is already shared across
// users. Service-account handshakes never write the user-facing profile.
func (c *Client) recordServerProfile(session *mcp.ClientSession) {
	if c.toolsCache == nil || session == nil || c.serviceAccount {
		return
	}
	result := session.InitializeResult()
	if result == nil {
		return
	}

	profile := ServerProfile{ServerURL: c.config.BaseURL}
	if info := result.ServerInfo; info != nil {
		profile.Name = promptSafeText(info.Name, maxServerNameRunes)
		profile.Title = promptSafeText(info.Title, maxServerNameRunes)
		profile.Description = promptSafeText(info.Description, maxServerDescriptionRunes)
	}
	if sharedToolsCacheAllowedForServer(c.config) {
		profile.Instructions = promptSafeText(result.Instructions, maxServerInstructionsRunes)
	}

	if err := c.toolsCache.SetServerProfile(c.config.Name, profile, time.Now()); err != nil {
		c.log.Warn("Failed to store MCP server profile", "server", c.config.Name, "error", err)
	}
}

// DescribeUnconnectedMCPServers returns what is known about each remote server
// named in authErrors: the admin-configured name, what the server reported
// about itself on an earlier connection, and the tools it is known to offer.
func (m *ClientManager) DescribeUnconnectedMCPServers(authErrors []llm.ToolAuthError) []llm.UnconnectedMCPServer {
	if len(authErrors) == 0 {
		return nil
	}
	cfg, _ := m.snapshotRuntime()

	described := make([]llm.UnconnectedMCPServer, 0, len(authErrors))
	seen := make(map[string]bool, len(authErrors))
	for _, authErr := range authErrors {
		if authErr.ServerName == "" || seen[authErr.ServerName] {
			continue
		}
		seen[authErr.ServerName] = true

		server := findRemoteServer(cfg, authErr.ServerName, authErr.ServerOrigin)
		if server == nil {
			described = append(described, llm.UnconnectedMCPServer{
				Name:         authErr.ServerName,
				ServerOrigin: authErr.ServerOrigin,
			})
			continue
		}
		described = append(described, describeUnconnectedServer(server, m.toolsCache))
	}
	return described
}

// MCPServerOriginsByName maps each enabled remote server's configured name to
// its origin.
func (m *ClientManager) MCPServerOriginsByName() map[string]string {
	cfg, _ := m.snapshotRuntime()
	origins := make(map[string]string, len(cfg.Servers))
	for _, server := range cfg.Servers {
		if server.Enabled && server.Name != "" && server.BaseURL != "" {
			origins[server.Name] = server.BaseURL
		}
	}
	return origins
}

func findRemoteServer(cfg Config, name, origin string) *ServerConfig {
	for i := range cfg.Servers {
		server := &cfg.Servers[i]
		if server.Enabled && server.Name == name && server.BaseURL == origin {
			return server
		}
	}
	return nil
}

func describeUnconnectedServer(server *ServerConfig, cache *ToolsCache) llm.UnconnectedMCPServer {
	described := llm.UnconnectedMCPServer{
		Name:         server.Name,
		ServerOrigin: server.BaseURL,
	}

	if cache != nil {
		if profile := cache.GetServerProfile(server.Name, server.BaseURL); profile != nil {
			title := cmp.Or(profile.Title, profile.Name)
			if !strings.EqualFold(title, server.Name) {
				described.Title = title
			}
			described.Description = profile.Description
			described.Instructions = profile.Instructions
		}
	}

	described.Tools = knownServerTools(server, cache)
	return described
}

// knownServerTools lists the tools a server is known to offer: the shared
// catalog discovered by earlier connections where catalogs are shared, else
// the tools the admin configured. Admin-disabled tools are left out.
func knownServerTools(server *ServerConfig, cache *ToolsCache) []llm.ToolInfo {
	overrides := make(map[string]string, len(server.ToolConfigs))
	for _, toolConfig := range server.ToolConfigs {
		if toolConfig.RetrievalDescriptionOverride != "" {
			overrides[toolConfig.Name] = toolConfig.RetrievalDescriptionOverride
		}
	}

	var tools []llm.ToolInfo
	add := func(name, description string) {
		if _, enabled := server.GetToolPolicy(name); !enabled {
			return
		}
		tools = append(tools, llm.ToolInfo{
			Name:         promptSafeText(name, maxServerNameRunes),
			Description:  promptSafeText(cmp.Or(overrides[name], description), maxKnownToolDescRunes),
			ServerOrigin: server.BaseURL,
		})
	}

	if cache != nil && sharedToolsCacheAllowedForServer(*server) {
		for name, tool := range cache.GetToolsForURL(server.Name, server.BaseURL) {
			if tool != nil {
				add(name, tool.Description)
			}
		}
	}
	if len(tools) == 0 {
		for _, toolConfig := range server.ToolConfigs {
			add(toolConfig.Name, "")
		}
	}

	slices.SortFunc(tools, func(a, b llm.ToolInfo) int { return cmp.Compare(a.Name, b.Name) })
	tools = slices.CompactFunc(tools, func(a, b llm.ToolInfo) bool { return a.Name == b.Name })
	if len(tools) > maxKnownToolsPerServer {
		tools = tools[:maxKnownToolsPerServer]
	}
	return tools
}
