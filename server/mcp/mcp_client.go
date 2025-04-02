// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// ServerClient represents the connection to a single MCP server
type ServerClient struct {
	client   *client.SSEMCPClient
	serverID string
	tools    map[string]mcp.Tool
}

// MCPClient represents a wrapper for multiple MCP server clients
type MCPClient struct {
	clients  map[string]*ServerClient
	log      pluginapi.LogService
	enabled  bool
	toolsMu  sync.RWMutex
	toolDefs map[string]struct {
		tool     mcp.Tool
		serverID string
	}
}

// ServerConfig contains the configuration for a single MCP server
type ServerConfig struct {
	BaseURL string            `json:"baseURL"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Config contains the configuration for the MCP clients
type Config struct {
	Enabled bool                    `json:"enabled"`
	Servers map[string]ServerConfig `json:"servers"`
}

// NewMCPClient creates a new MCP client with multiple servers
func NewMCPClient(config Config, log pluginapi.LogService) (*MCPClient, error) {
	// If not enabled or no servers configured, return a disabled client
	if !config.Enabled || len(config.Servers) == 0 {
		log.Debug("MCP client is disabled or no servers configured")
		return &MCPClient{
			enabled: false,
			log:     log,
			clients: make(map[string]*ServerClient),
			toolDefs: make(map[string]struct {
				tool     mcp.Tool
				serverID string
			}),
		}, nil
	}

	mcpClient := &MCPClient{
		log:     log,
		enabled: true,
		clients: make(map[string]*ServerClient),
		toolDefs: make(map[string]struct {
			tool     mcp.Tool
			serverID string
		}),
	}

	ctx := context.Background()

	// Initialize clients for each server
	for serverID, serverConfig := range config.Servers {
		if serverConfig.BaseURL == "" {
			log.Warn("Skipping MCP server with empty BaseURL", "serverID", serverID)
			continue
		}

		var opts []client.ClientOption
		if serverConfig.Headers != nil {
			opts = append(opts, client.WithHeaders(serverConfig.Headers))
		}

		opts = append(opts, client.WithSSEReadTimeout(time.Hour*10000))

		sseClient, err := client.NewSSEMCPClient(serverConfig.BaseURL, opts...)
		if err != nil {
			log.Error("Failed to create MCP client", "serverID", serverID, "error", err)
			continue
		}

		// Initialize the client
		if err := sseClient.Start(ctx); err != nil {
			log.Error("Failed to start MCP client", "serverID", serverID, "error", err)
			sseClient.Close()
			continue
		}

		// Send initialize request
		initResult, err := sseClient.Initialize(ctx, mcp.InitializeRequest{})
		if err != nil {
			log.Error("Failed to initialize MCP client", "serverID", serverID, "error", err)
			sseClient.Close()
			continue
		}

		log.Debug("MCP client initialized successfully",
			"serverID", serverID,
			"serverInfo", initResult.ServerInfo)

		// Create the server client
		serverClient := &ServerClient{
			client:   sseClient,
			serverID: serverID,
			tools:    make(map[string]mcp.Tool),
		}

		mcpClient.clients[serverID] = serverClient

		// Fetch available tools for this server
		result, err := sseClient.ListTools(ctx, mcp.ListToolsRequest{})
		if err != nil {
			log.Error("Failed to list tools", "serverID", serverID, "error", err)
			continue
		}

		// Store the tools for this server
		for _, tool := range result.Tools {
			serverClient.tools[tool.Name] = tool

			// Check for tool name conflicts across servers
			if existingTool, exists := mcpClient.toolDefs[tool.Name]; exists {
				log.Warn("Tool name conflict detected",
					"tool", tool.Name,
					"server1", existingTool.serverID,
					"server2", serverID)
				// For now, last server wins for conflicts
			}

			mcpClient.toolDefs[tool.Name] = struct {
				tool     mcp.Tool
				serverID string
			}{
				tool:     tool,
				serverID: serverID,
			}

			log.Debug("Registered MCP tool",
				"name", tool.Name,
				"description", tool.Description,
				"server", serverID)
		}
	}

	// If no servers were successfully connected, disable the client
	if len(mcpClient.clients) == 0 {
		log.Warn("No MCP servers were successfully connected, disabling MCP client")
		mcpClient.enabled = false
	}

	return mcpClient, nil
}

// Close closes all MCP clients
func (m *MCPClient) Close() error {
	if !m.enabled || len(m.clients) == 0 {
		return nil
	}

	var lastErr error

	// Close all MCP clients
	for serverID, client := range m.clients {
		if err := client.client.Close(); err != nil {
			m.log.Error("Failed to close MCP client", "serverID", serverID, "error", err)
			lastErr = err
		}
	}

	return lastErr
}

func ConvertViaJSON(source map[string]any) (*orderedmap.OrderedMap[string, *jsonschema.Schema], error) {
	var target orderedmap.OrderedMap[string, *jsonschema.Schema]
	jsonData, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonData, &target)
	return &target, err
}

// GetTools returns the tools available from all MCP clients
func (m *MCPClient) GetTools() []llm.Tool {
	if !m.enabled {
		return nil
	}

	m.toolsMu.RLock()
	defer m.toolsMu.RUnlock()

	tools := make([]llm.Tool, 0, len(m.toolDefs))
	for name, toolInfo := range m.toolDefs {
		properties, err := ConvertViaJSON(toolInfo.tool.InputSchema.Properties)
		if err != nil {
			m.log.Error("Failed to convert tool input schema properties", "tool", name, "error", err)
			continue
		}
		schema := &jsonschema.Schema{
			Type:       toolInfo.tool.InputSchema.Type,
			Properties: properties,
			Required:   toolInfo.tool.InputSchema.Required,
		}
		tools = append(tools, llm.Tool{
			Name:        name,
			Description: toolInfo.tool.Description,
			Schema:      schema,
			Resolver:    m.createToolResolver(name),
		})
	}

	return tools
}

// createToolResolver creates a resolver function for the given tool
func (m *MCPClient) createToolResolver(toolName string) func(llmContext *llm.Context, argsGetter llm.ToolArgumentGetter) (string, error) {
	return func(llmContext *llm.Context, argsGetter llm.ToolArgumentGetter) (string, error) {
		if !m.enabled {
			return "", fmt.Errorf("MCP client is not enabled")
		}

		// Find which server has this tool
		m.toolsMu.RLock()
		toolInfo, exists := m.toolDefs[toolName]
		if !exists {
			m.toolsMu.RUnlock()
			return "", fmt.Errorf("tool %s not found", toolName)
		}
		serverID := toolInfo.serverID
		m.toolsMu.RUnlock()

		// Get the server client
		serverClient, exists := m.clients[serverID]
		if !exists {
			return "", fmt.Errorf("server %s for tool %s not found", serverID, toolName)
		}

		// Get the raw arguments
		var rawArgs json.RawMessage
		if err := argsGetter(&rawArgs); err != nil {
			return "", fmt.Errorf("failed to get arguments for tool %s: %w", toolName, err)
		}

		// Create the context for the tool call
		ctx := context.Background()

		// Call the tool
		callRequest := mcp.CallToolRequest{}
		callRequest.Params.Name = toolName
		callRequest.Params.Arguments = make(map[string]interface{})

		// Parse the raw arguments into a map
		var args map[string]interface{}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return "", fmt.Errorf("failed to parse arguments for tool %s: %w", toolName, err)
		}
		callRequest.Params.Arguments = args

		result, err := serverClient.client.CallTool(ctx, callRequest)
		if err != nil {
			return "", fmt.Errorf("failed to call tool %s on server %s: %w", toolName, serverID, err)
		}

		// Extract text content from the result
		if len(result.Content) > 0 {
			text := ""
			for _, content := range result.Content {
				if textContent, ok := mcp.AsTextContent(content); ok {
					text += textContent.Text + "\n"
				}
			}
			return text, nil
		}

		return "", fmt.Errorf("no text content found in response from tool %s on server %s", toolName, serverID)
	}
}
