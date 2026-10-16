// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llmcontext

import (
	"errors"
	"slices"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mcp"
)

// MCPToolErrorResult returns the tool result text for a failed tool call.
// When the call failed because the user's authorization for its MCP server
// lapsed, the server's remaining tools are removed for the rest of the turn,
// and where connection requests can be offered, RequestMCPConnection is
// offered for it.
//
// It changes c's tool store, so it must run on the goroutine that owns the
// tool loop, between tool calls, never from inside a resolver.
func MCPToolErrorResult(c *llm.Context, err error) string {
	needed, ok := errors.AsType[*mcp.ReconnectNeededError](err)
	if !ok {
		return err.Error()
	}
	if c == nil || c.Tools == nil {
		return needed.Error()
	}
	c.Tools.RemoveMCPToolsFromOrigin(needed.Server.ServerOrigin)
	if c.ToolCatalog.AdmitUnconnectedMCPServer == nil {
		return needed.Error()
	}

	listed := slices.ContainsFunc(c.UnconnectedMCPServers, func(server llm.UnconnectedMCPServer) bool {
		return server.Name == needed.Server.Name
	})
	if !listed {
		c.UnconnectedMCPServers = append(c.UnconnectedMCPServers, c.ToolCatalog.AdmitUnconnectedMCPServer(needed.Server))
	}
	c.Tools.AddTools([]llm.Tool{mcp.NewRequestConnectionTool(c.UnconnectedMCPServers)})
	return needed.ConnectGuidance()
}
