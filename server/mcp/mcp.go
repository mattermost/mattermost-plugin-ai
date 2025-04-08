// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package mcp provides a client for the Model Control Protocol (MCP) that allows
// the AI plugin to access external tools provided by MCP servers.
//
// This client manages connections to MCP servers on a per-user basis, providing
// tool access based on user context and maintaining connections efficiently.
package mcp
