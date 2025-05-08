// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package mcp provides a client for the Model Control Protocol (MCP) that allows
// the AI plugin to access external tools provided by MCP servers.
//
// The UserClient represents a single user's connection to multiple MCP servers.
// The UserClient currents only supports authentication via Mattermost user ID header
// X-Mattermost-UserID. In the future it will support our OAuth implementation.
//
// The ClientManager manages multiple UserClients, allowing for efficient mangement
// of connections. It is responsible for creating and closing UserClients as needed.
//
// The organization reflects the need for each user to have their own connection to
// the MCP server given the design of MCP.
package mcp
