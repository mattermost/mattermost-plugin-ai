// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

const MMUserIDHeader = "X-Mattermost-UserID"

// ServerConnection represents the connection to a single MCP server
type ServerConnection struct {
	client   *client.SSEMCPClient
	serverID string
	tools    map[string]mcp.Tool
}

// ServerConfig contains the configuration for a single MCP server
type ServerConfig struct {
	BaseURL string            `json:"baseURL"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ToolDefinition represents a tool provided by an MCP server
type ToolDefinition struct {
	tool     mcp.Tool
	serverID string
}

// UserClient represents a per-user MCP client with multiple server connections
type UserClient struct {
	clients      map[string]*ServerConnection
	toolDefs     map[string]ToolDefinition
	lastActivity time.Time
	userID       string
	log          pluginapi.LogService
}

// ConnectToAllServers initializes connections to all provided servers
func (c *UserClient) ConnectToAllServers(servers map[string]ServerConfig) error {
	if len(servers) == 0 {
		c.log.Debug("No MCP servers provided for user", "userID", c.userID)
		return nil
	}

	// Initialize clients for each server
	for serverID, serverConfig := range servers {
		if serverConfig.BaseURL == "" {
			c.log.Warn("Skipping MCP server with empty BaseURL", "serverID", serverID)
			continue
		}

		if err := c.connectToServer(context.Background(), serverID, serverConfig); err != nil {
			c.log.Error("Failed to connect to MCP server", "userID", c.userID, "serverID", serverID, "error", err)
			continue
		}
	}

	// If no servers were successfully connected, return error
	if len(c.clients) == 0 {
		c.log.Warn("No MCP servers were successfully connected for user", "userID", c.userID)
		return fmt.Errorf("no MCP servers were successfully connected")
	}

	// Update last activity time
	c.lastActivity = time.Now()

	return nil
}

// connectToServer establishes a connection to a single server and registers its tools
func (c *UserClient) connectToServer(ctx context.Context, serverID string, serverConfig ServerConfig) error {
	var opts []client.ClientOption
	headers := make(map[string]string)
	headers[MMUserIDHeader] = c.userID
	if serverConfig.Headers != nil {
		maps.Copy(headers, serverConfig.Headers)
	}
	opts = append(opts, client.WithHeaders(serverConfig.Headers))

	sseClient, err := client.NewSSEMCPClient(serverConfig.BaseURL, opts...)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Ensure client is closed on error
	success := false
	defer func() {
		if !success {
			sseClient.Close()
		}
	}()

	if startErr := sseClient.Start(ctx); startErr != nil {
		return fmt.Errorf("failed to start MCP client: %w", startErr)
	}

	initResult, err := sseClient.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	c.log.Debug("MCP client initialized successfully",
		"userID", c.userID,
		"serverID", serverID,
		"serverInfo", initResult.ServerInfo)

	serverClient := &ServerConnection{
		client:   sseClient,
		serverID: serverID,
		tools:    make(map[string]mcp.Tool),
	}
	c.clients[serverID] = serverClient

	// List and register available tools
	result, err := sseClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	// Store the tools for this server
	for _, tool := range result.Tools {
		serverClient.tools[tool.Name] = tool

		// Check for tool name conflicts across servers
		if existingTool, exists := c.toolDefs[tool.Name]; exists {
			c.log.Warn("Tool name conflict detected",
				"userID", c.userID,
				"tool", tool.Name,
				"server1", existingTool.serverID,
				"server2", serverID)
			// For now, last server wins for conflicts
		}

		c.toolDefs[tool.Name] = ToolDefinition{
			tool:     tool,
			serverID: serverID,
		}

		c.log.Debug("Registered MCP tool",
			"userID", c.userID,
			"name", tool.Name,
			"description", tool.Description,
			"server", serverID)
	}

	success = true
	return nil
}

// Close closes all server connections for a user client
func (c *UserClient) Close() error {
	if len(c.clients) == 0 {
		return nil
	}

	var lastErr error

	// Close all MCP server clients
	for serverID, client := range c.clients {
		if err := client.client.Close(); err != nil {
			c.log.Error("Failed to close MCP client", "userID", c.userID, "serverID", serverID, "error", err)
			lastErr = err
		}
	}

	// Clear clients and tool definitions
	c.clients = make(map[string]*ServerConnection)
	c.toolDefs = make(map[string]ToolDefinition)

	return lastErr
}

// ConvertPropertiesToOrderedMap converts a map of properties to an OrderedMap using JSON marshaling
func ConvertPropertiesToOrderedMap(source map[string]any) (*orderedmap.OrderedMap[string, *jsonschema.Schema], error) {
	var target orderedmap.OrderedMap[string, *jsonschema.Schema]
	jsonData, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonData, &target)
	return &target, err
}

// GetTools returns the tools available from the client
func (c *UserClient) GetTools() []llm.Tool {
	if len(c.clients) == 0 {
		return nil
	}

	tools := make([]llm.Tool, 0, len(c.toolDefs))
	for name, toolInfo := range c.toolDefs {
		properties, err := ConvertPropertiesToOrderedMap(toolInfo.tool.InputSchema.Properties)
		if err != nil {
			c.log.Error("Failed to convert tool input schema properties", "userID", c.userID, "tool", name, "error", err)
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
			Resolver:    c.createToolResolver(name),
		})
	}

	return tools
}

// createToolResolver creates a resolver function for the given tool
func (c *UserClient) createToolResolver(toolName string) func(llmContext *llm.Context, argsGetter llm.ToolArgumentGetter) (string, error) {
	return func(llmContext *llm.Context, argsGetter llm.ToolArgumentGetter) (string, error) {
		if len(c.clients) == 0 {
			return "", fmt.Errorf("MCP client has no active connections")
		}

		// Update last activity time for this client
		c.lastActivity = time.Now()

		// Find which server has this tool
		toolInfo, exists := c.toolDefs[toolName]
		if !exists {
			return "", fmt.Errorf("tool %s not found", toolName)
		}
		serverID := toolInfo.serverID

		// Get the server client
		serverClient, exists := c.clients[serverID]
		if !exists {
			return "", fmt.Errorf("server %s for tool %s not found", serverID, toolName)
		}

		// Get the raw arguments
		var rawArgs json.RawMessage
		if err := argsGetter(&rawArgs); err != nil {
			return "", fmt.Errorf("failed to get arguments for tool %s: %w", toolName, err)
		}

		// Create the context for the tool call with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

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
