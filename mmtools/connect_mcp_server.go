// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mmtools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
)

// ConnectMCPServerToolName is the runtime name of the built-in connect tool.
const ConnectMCPServerToolName = llm.ConnectMCPServerToolName

const connectMCPServerDescription = "Ask the requesting user to connect (sign in to) an MCP server they have not connected yet, so its tools become available. " +
	"Call this when the user's request needs one of the servers listed below instead of saying the capability does not exist. " +
	"Only call it when the request actually needs that server. " +
	"The user is shown a Connect card and may connect or skip; the tool result tells you which. " +
	"Never claim to have performed an action with a server that is not connected."

// ConnectMCPServerArgs is the LLM-visible input schema for the connect tool.
type ConnectMCPServerArgs struct {
	Server string `json:"server" jsonschema_description:"Name of the MCP server to connect, exactly as listed in the tool description."`
}

// NewConnectMCPServerTool returns the connect tool for this turn's unconnected
// servers. Like AskUserQuestion it is answered through the tool-approval flow
// (Conversations.HandleToolCall), never executed server-side.
func NewConnectMCPServerTool(servers []llm.UnconnectedMCPServer) llm.Tool {
	names := make([]string, 0, len(servers))
	for _, server := range servers {
		names = append(names, server.Name)
	}

	schema := llm.NewJSONSchemaFromStruct[ConnectMCPServerArgs]()
	if prop := schema.Properties["server"]; prop != nil && len(names) > 0 {
		prop.Enum = make([]any, 0, len(names))
		for _, name := range names {
			prop.Enum = append(prop.Enum, name)
		}
	}

	return llm.Tool{
		Name:            ConnectMCPServerToolName,
		Description:     connectMCPServerToolDescription(servers),
		Schema:          schema,
		UserInteraction: llm.UserInteractionConnect,
		ValidateArguments: func(input json.RawMessage) error {
			args, err := parseConnectMCPServerArgs(input)
			if err != nil {
				return err
			}
			for _, name := range names {
				if args.Server == name {
					return nil
				}
			}
			return fmt.Errorf("server %q is not an unconnected MCP server; use one of: %s", args.Server, strings.Join(names, ", "))
		},
		Resolver: func(_ context.Context, _ *llm.Context, _ llm.ToolArgumentGetter) (string, error) {
			return "", errors.New("ConnectMCPServer must be answered by the user and cannot be executed directly")
		},
	}
}

// connectMCPServerToolDescription embeds the current server list because the
// conversation's system prompt is fixed when it is created, while the tool
// catalog is rebuilt every turn.
func connectMCPServerToolDescription(servers []llm.UnconnectedMCPServer) string {
	var b strings.Builder
	b.WriteString(connectMCPServerDescription)
	b.WriteString("\n\nServers the user has not connected:")
	for _, server := range servers {
		b.WriteString("\n- ")
		b.WriteString(server.Name)
		if server.Description != "" {
			b.WriteString(": ")
			b.WriteString(server.Description)
		}
		if len(server.ToolNames) > 0 {
			b.WriteString(" (tools include: ")
			b.WriteString(server.ToolNamesList())
			b.WriteString(")")
		}
	}
	return b.String()
}

// resolveConnectMCPServerAccepted computes the tool result for an accepted
// connect call from the catalog rebuilt after the user's click, so the result
// reflects whether the connection actually completed.
func resolveConnectMCPServerAccepted(input json.RawMessage, llmContext *llm.Context) (string, error) {
	args, err := parseConnectMCPServerArgs(input)
	if err != nil {
		return "", err
	}
	if llmContext == nil {
		return "", errors.New("connect result requires the rebuilt tool context")
	}

	if _, stillUnconnected := llmContext.UnconnectedMCPServerByName(args.Server); stillUnconnected {
		return fmt.Sprintf("%s is still not connected: the user did not finish connecting it. "+
			"Do not perform actions that need %s. Tell the user they can connect %s and ask again.",
			args.Server, args.Server, args.Server), nil
	}

	result := fmt.Sprintf("User connected %s. Its tools are now available.", args.Server)
	if llmContext.ToolCatalog.MCPDynamicToolLoading && llmContext.IsToolCallable("search_tools") {
		result += " Use search_tools to find them and load_tool to load one before calling it."
	}
	return result, nil
}

// connectMCPServerSkippedResult is the tool result for a skipped connect call.
func connectMCPServerSkippedResult(input json.RawMessage) string {
	args, err := parseConnectMCPServerArgs(input)
	if err != nil {
		return "User declined to connect the MCP server"
	}
	return fmt.Sprintf("User declined to connect %s", args.Server)
}

// ConnectMCPServerName returns the server named by a connect call's arguments.
func ConnectMCPServerName(input json.RawMessage) (string, bool) {
	args, err := parseConnectMCPServerArgs(input)
	if err != nil {
		return "", false
	}
	return args.Server, true
}

func parseConnectMCPServerArgs(input json.RawMessage) (ConnectMCPServerArgs, error) {
	var args ConnectMCPServerArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return args, fmt.Errorf("failed to parse connect arguments: %w", err)
	}
	if strings.TrimSpace(args.Server) == "" {
		return args, errors.New("server must not be empty")
	}
	return args, nil
}
