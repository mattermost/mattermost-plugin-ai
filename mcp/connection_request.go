// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

// RequestConnectionToolName is the runtime name of the built-in tool that asks
// the user to connect their account to an MCP server.
const RequestConnectionToolName = "RequestMCPConnection"

const (
	requestConnectionDescription = "Ask the requesting user to connect their own account to an integration they have not connected yet. " +
		"Use this when fulfilling the request needs one of these integrations, instead of saying you cannot do it. " +
		"The user is shown a Connect button and the conversation pauses until they connect or skip. " +
		"The result says whether the integration is now connected and how to start using its tools. " +
		"Request one integration per call, and only when the current request actually needs it."

	// maxConnectedToolNames bounds the tool names listed in a connection
	// result; the model can still find the rest through the tool catalog.
	maxConnectedToolNames = 50
)

// RequestConnectionArgs is the LLM-visible input schema for the connection
// request tool.
type RequestConnectionArgs struct {
	Server string `json:"server" jsonschema:"Exact name of the integration to connect"`
	Reason string `json:"reason" jsonschema:"One short sentence, shown to the user, saying what you need the integration for"`
}

// RequestConnectionResult is the tool result written once the user decides.
type RequestConnectionResult struct {
	Server    string   `json:"server"`
	Connected bool     `json:"connected"`
	Tools     []string `json:"tools,omitempty"`
	Message   string   `json:"message"`
}

// NewRequestConnectionTool returns the connection request tool for servers.
// The call is answered through the tool-approval flow
// (Conversations.HandleToolCall), so the resolver is only an error backstop.
func NewRequestConnectionTool(servers []llm.UnconnectedMCPServer) llm.Tool {
	names := make([]string, 0, len(servers))
	for _, server := range servers {
		names = append(names, server.Name)
	}

	schema := llm.NewJSONSchemaFromStruct[RequestConnectionArgs]()
	if serverProp := schema.Properties["server"]; serverProp != nil {
		restricted := *serverProp
		restricted.Enum = make([]any, 0, len(names))
		for _, name := range names {
			restricted.Enum = append(restricted.Enum, name)
		}
		schema.Properties = maps.Clone(schema.Properties)
		schema.Properties["server"] = &restricted
	}

	return llm.Tool{
		Name:            RequestConnectionToolName,
		Description:     requestConnectionDescription,
		Schema:          schema,
		UserInteraction: llm.UserInteractionConnect,
		ValidateArguments: func(input json.RawMessage) error {
			args, err := parseRequestConnectionArgs(input)
			if err != nil {
				return err
			}
			if !slices.Contains(names, args.Server) {
				return fmt.Errorf("%q is not an integration the user can connect; use one of: %s", args.Server, strings.Join(names, ", "))
			}
			return nil
		},
		Resolver: func(context.Context, *llm.Context, llm.ToolArgumentGetter) (string, error) {
			return "", errors.New(RequestConnectionToolName + " must be answered by the user and cannot be executed directly")
		},
	}
}

// ResolveConnectionRequest builds the tool result for an accepted connection
// request. llmContext must be built after the user's decision so its tool
// catalog reflects whether the connection succeeded. listToolNames must be
// false when anyone but the requester can read the result: the tools a server
// offers after connecting can be specific to the connected account.
func ResolveConnectionRequest(input json.RawMessage, llmContext *llm.Context, listToolNames bool) (string, error) {
	args, err := parseRequestConnectionArgs(input)
	if err != nil {
		return "", err
	}

	result := RequestConnectionResult{Server: args.Server}
	if tools := connectedServerTools(args.Server, llmContext); len(tools) > 0 {
		result.Connected = true
		// The conversation's system prompt is fixed when it starts and still
		// lists the server as unconnected, so the result must override it.
		result.Message = fmt.Sprintf("The user connected %[1]s. Its tools are now available in this conversation. This replaces any earlier note that %[1]s is not connected; do not request the connection again.", args.Server)
		switch {
		case listToolNames:
			result.Tools = tools
			if llmContext.ToolCatalog.MCPDynamicToolLoading {
				result.Message += fmt.Sprintf(" Its tools are not loaded yet: call %s with one of the exact names below (or %s to find others) before calling a tool.", LoadToolName, SearchToolsName)
			}
		case llmContext.ToolCatalog.MCPDynamicToolLoading:
			result.Message += fmt.Sprintf(" Its tools are not loaded yet: call %s to find the ones you need, then %s, before calling a tool.", SearchToolsName, LoadToolName)
		}
	} else {
		result.Message = fmt.Sprintf("%s is still not connected for the user, so its tools are unavailable. Continue without it and do not request the connection again unless the user asks.", args.Server)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal connection request result: %w", err)
	}
	return string(data), nil
}

// ConnectionRequestDeclinedResult is the tool result written when the user
// skips a connection request.
func ConnectionRequestDeclinedResult(input json.RawMessage) string {
	server := "the integration"
	if args, err := parseRequestConnectionArgs(input); err == nil {
		server = args.Server
	}
	return fmt.Sprintf("The user chose not to connect %s. Continue without it and do not request the connection again unless the user asks.", server)
}

func connectedServerTools(server string, llmContext *llm.Context) []string {
	if llmContext == nil || llmContext.Tools == nil {
		return nil
	}
	stillNeedsAuth := slices.ContainsFunc(llmContext.Tools.GetAuthErrors(), func(authErr llm.ToolAuthError) bool {
		return authErr.ServerName == server
	})
	if stillNeedsAuth {
		return nil
	}

	names := llmContext.Tools.MCPToolNamesFromOrigin(llmContext.ToolCatalog.MCPServerOrigins[server])
	if len(names) > maxConnectedToolNames {
		names = names[:maxConnectedToolNames]
	}
	return names
}

func parseRequestConnectionArgs(input json.RawMessage) (RequestConnectionArgs, error) {
	var args RequestConnectionArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return args, fmt.Errorf("failed to parse connection request arguments: %w", err)
	}
	args.Server = strings.TrimSpace(args.Server)
	if args.Server == "" {
		return args, errors.New("server must not be empty")
	}
	return args, nil
}
