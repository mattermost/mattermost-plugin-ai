// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	stdctx "context" // Import standard context as stdctx to avoid confusion
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

// MCPClient represents a wrapper for the SSEMCPClient
type MCPClient struct {
	client   *client.SSEMCPClient
	log      pluginapi.LogService
	enabled  bool
	toolsMu  sync.RWMutex
	toolDefs map[string]mcp.Tool
}

// Config contains the configuration for the MCP client
type Config struct {
	Enabled  bool   `json:"enabled"`
	BaseURL  string `json:"baseURL"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// NewMCPClient creates a new MCP client
func NewMCPClient(config Config, log pluginapi.LogService) (*MCPClient, error) {
	if !config.Enabled {
		return &MCPClient{
			enabled: false,
			log:     log,
		}, nil
	}

	var opts []client.ClientOption
	if config.Headers != nil {
		opts = append(opts, client.WithHeaders(config.Headers))
	}

	mcpClient, err := client.NewSSEMCPClient(config.BaseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Initialize the client
	ctx := stdctx.Background()
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start MCP client: %w", err)
	}

	// Send initialize request
	initResult, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	log.Debug("MCP client initialized successfully", "server", initResult.ServerInfo)

	// Create the client wrapper
	client := &MCPClient{
		client:   mcpClient,
		log:      log,
		enabled:  true,
		toolDefs: make(map[string]mcp.Tool),
	}

	// Get available tools
	if err := client.fetchAvailableTools(ctx); err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("failed to fetch available tools: %w", err)
	}

	return client, nil
}

// fetchAvailableTools fetches the available tools from the MCP server
func (m *MCPClient) fetchAvailableTools(ctx stdctx.Context) error {
	if !m.enabled {
		return nil
	}

	result, err := m.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	m.toolsMu.Lock()
	defer m.toolsMu.Unlock()

	for _, tool := range result.Tools {
		m.toolDefs[tool.Name] = tool
		m.log.Debug("Registered MCP tool", "name", tool.Name, "description", tool.Description)
	}

	return nil
}

// Close closes the MCP client
func (m *MCPClient) Close() error {
	if !m.enabled || m.client == nil {
		return nil
	}
	return m.client.Close()
}

// GetTools returns the tools available from the MCP client
func (m *MCPClient) GetTools() []llm.Tool {
	if !m.enabled {
		return nil
	}

	m.toolsMu.RLock()
	defer m.toolsMu.RUnlock()

	tools := make([]llm.Tool, 0, len(m.toolDefs))
	for name, toolDef := range m.toolDefs {
		// Create a closure to capture the tool name for the resolver
		toolName := name
		tools = append(tools, llm.Tool{
			Name:        toolName,
			Description: toolDef.Description,
			Schema:      toolDef.InputSchema,
			Resolver:    m.createToolResolver(toolName),
		})
	}

	return tools
}

// createToolResolver creates a resolver function for the given tool
func (m *MCPClient) createToolResolver(toolName string) func(context *llm.Context, argsGetter llm.ToolArgumentGetter) (string, error) {
	return func(context *llm.Context, argsGetter llm.ToolArgumentGetter) (string, error) {
		if !m.enabled {
			return "", fmt.Errorf("MCP client is not enabled")
		}

		// Get the raw arguments
		var rawArgs json.RawMessage
		if err := argsGetter(&rawArgs); err != nil {
			return "", fmt.Errorf("failed to get arguments for tool %s: %w", toolName, err)
		}

		// Create the context for the tool call
		stdCtx := stdctx.Background()

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
			
		result, err := m.client.CallTool(stdCtx, callRequest)
		if err != nil {
			return "", fmt.Errorf("failed to call tool %s: %w", toolName, err)
		}

		// Extract text content from the result
		if len(result.Content) > 0 {
			for _, content := range result.Content {
				if textContent, ok := mcp.AsTextContent(content); ok {
					return textContent.Text, nil
				}
			}
		}
		
		return "", fmt.Errorf("no text content found in response from tool %s", toolName)
	}
}