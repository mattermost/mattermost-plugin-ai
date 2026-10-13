// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-agents/v2/i18n"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmtools"
	"github.com/mattermost/mattermost-plugin-agents/v2/store"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	mcpConnectFallbackDefaultMessage = "To continue, connect **%s**: [Connect %s](%s)"
	mcpConnectFallbackKVPrefix       = "mcp_connect_link_"
)

// The fallback message is the only markdown link this package builds, and the
// codebase has no shared helper for one. The name is escaped so it renders as
// literal text, and the URL so it cannot close the link target early.
var (
	mcpConnectFallbackURLEscaper      = strings.NewReplacer("(", "%28", ")", "%29", " ", "%20")
	mcpConnectFallbackMarkdownEscaper = strings.NewReplacer(
		`\`, `\\`, "*", `\*`, "_", `\_`, "`", "\\`", "[", `\[`, "]", `\]`, "~", `\~`,
	)
)

// sendMCPConnectFallbacks sends the requester an ephemeral link to connect
// each MCP server a pending ConnectMCPServer call names, for clients (e.g.
// mobile) that cannot render the Connect card. Ephemeral posts reach every
// client the requester has open, so web users see the link next to the card;
// that duplication is accepted rather than guessing which client is in use.
// The link is built from the server-side AuthURL, never from model output, and
// each server is linked at most once per conversation.
func (c *Conversations) sendMCPConnectFallbacks(convID string, llmContext *llm.Context, pending []llm.ToolCall) {
	if llmContext == nil || c.convService == nil || c.mmClient == nil {
		return
	}

	var conv *store.Conversation
	for _, tc := range pending {
		if tc.Name != llm.ConnectMCPServerToolName {
			continue
		}
		name, ok := mmtools.ConnectMCPServerName(tc.Arguments)
		if !ok {
			continue
		}
		server, listed := llmContext.UnconnectedMCPServerByName(name)
		if !listed || server.AuthURL == "" {
			continue
		}
		if conv == nil {
			loaded, err := c.convService.GetConversation(convID)
			if err != nil || loaded.ChannelID == nil {
				c.mmClient.LogWarn("Failed to load conversation for MCP connect link", "conversation_id", convID)
				return
			}
			conv = loaded
		}
		if !c.claimMCPConnectFallback(convID, name) {
			continue
		}
		c.sendMCPConnectFallback(conv, llmContext.RequestingUser, server)
	}
}

// claimMCPConnectFallback atomically records that the server's link was sent
// for the conversation. It reports false when it already was, or when the
// claim cannot be recorded, so the link is never sent twice.
func (c *Conversations) claimMCPConnectFallback(convID, serverName string) bool {
	sum := sha256.Sum256([]byte(convID + "\x00" + serverName))
	key := mcpConnectFallbackKVPrefix + hex.EncodeToString(sum[:16])
	claimed, err := c.mmClient.KVCompareAndSet(key, nil, true)
	if err != nil {
		c.mmClient.LogWarn("Failed to record MCP connect link", "conversation_id", convID, "error", err.Error())
		return false
	}
	return claimed
}

func (c *Conversations) sendMCPConnectFallback(conv *store.Conversation, requester *model.User, server llm.UnconnectedMCPServer) {
	name := mcpConnectFallbackMarkdownEscaper.Replace(server.Name)
	authURL := mcpConnectFallbackURLEscaper.Replace(server.AuthURL)
	message := fmt.Sprintf(mcpConnectFallbackDefaultMessage, name, name, authURL)
	if c.i18n != nil {
		locale := ""
		if requester != nil {
			locale = requester.Locale
		}
		T := i18n.LocalizerFunc(c.i18n, c.fallbackLocale(locale))
		message = T("agents.mcp_connect_fallback", mcpConnectFallbackDefaultMessage, name, name, authURL)
	}

	post := &model.Post{
		UserId:    conv.BotID,
		ChannelId: *conv.ChannelID,
		Message:   message,
	}
	if conv.RootPostID != nil {
		post.RootId = *conv.RootPostID
	}
	c.mmClient.SendEphemeralPost(conv.UserID, post)
}
