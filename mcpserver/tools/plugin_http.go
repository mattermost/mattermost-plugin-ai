// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package tools

import (
	"context"

	"github.com/mattermost/mattermost-plugin-agents/mcpserver/auth"
)

// authorizationTokenFromContext returns a Mattermost bearer token for plugin
// HTTP callbacks. Direct tokens (OAuth/PAT on the MCP context) win; otherwise
// a session ID plus token resolver (plugin-hosted MCP) is used so ServeHTTP
// can restore plugin.Context.SessionId from Mattermost's session lookup.
func authorizationTokenFromContext(ctx context.Context) string {
	if token, ok := ctx.Value(auth.AuthTokenContextKey).(string); ok && token != "" {
		return token
	}
	sessionID := auth.SessionIDFromContext(ctx)
	if sessionID == "" {
		return ""
	}
	resolver, ok := ctx.Value(auth.TokenResolverContextKey).(auth.TokenResolver)
	if !ok || resolver == nil {
		return ""
	}
	token, err := resolver(sessionID)
	if err != nil || token == "" {
		return ""
	}
	return token
}
