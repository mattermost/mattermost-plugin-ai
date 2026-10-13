// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/conversation"
	"github.com/mattermost/mattermost-plugin-agents/v2/i18n"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmtools"
	"github.com/mattermost/mattermost-plugin-agents/v2/store"
	"github.com/mattermost/mattermost/server/public/model"
)

const mcpConnectFallbackDefaultMessage = "To continue, connect **%s**: [Connect %s](%s)"

var mcpConnectFallbackURLEscaper = strings.NewReplacer("(", "%28", ")", "%29", " ", "%20")

var mcpConnectFallbackMarkdownEscaper = strings.NewReplacer(
	`\`, `\\`, "*", `\*`, "_", `\_`, "`", "\\`", "[", `\[`, "]", `\]`, "~", `\~`,
)

// decorateStreamWithMCPConnectFallback sends the requester an ephemeral link
// to connect an MCP server when the model's ConnectMCPServer call goes
// pending, for clients (e.g. mobile) that cannot render the Connect card. The
// link is built from the server-side AuthURL, never from model output. Each
// server is linked at most once per conversation.
func (c *Conversations) decorateStreamWithMCPConnectFallback(bot *bots.Bot, stream *llm.TextStreamResult, post *model.Post, convID string, llmContext *llm.Context) *llm.TextStreamResult {
	if stream == nil || bot == nil || post == nil || llmContext == nil || llmContext.RequestingUser == nil {
		return stream
	}
	if len(llmContext.ToolCatalog.UnconnectedMCPServers) == 0 && llmContext.ToolCatalog.UnconnectedMCPServerScope == nil {
		return stream
	}

	output := make(chan llm.TextStreamEvent)
	go func() {
		defer close(output)
		linked := map[string]bool{}
		for event := range stream.Stream {
			if event.Type == llm.EventTypeToolCalls {
				if toolCalls, ok := event.Value.([]llm.ToolCall); ok {
					c.sendMCPConnectFallbacks(bot, post, convID, llmContext, toolCalls, linked)
				}
			}
			output <- event
		}
	}()
	return &llm.TextStreamResult{Stream: output}
}

func (c *Conversations) sendMCPConnectFallbacks(bot *bots.Bot, post *model.Post, convID string, llmContext *llm.Context, toolCalls []llm.ToolCall, linked map[string]bool) {
	var earlier map[string]bool
	for _, tc := range toolCalls {
		if tc.Name != mmtools.ConnectMCPServerToolName || tc.Status != llm.ToolCallStatusPending {
			continue
		}
		name, ok := mmtools.ConnectMCPServerName(tc.Arguments)
		if !ok || linked[name] {
			continue
		}
		server, listed := llmContext.UnconnectedMCPServerByName(name)
		if !listed || server.AuthURL == "" {
			continue
		}
		if earlier == nil {
			earlier = c.earlierMCPConnectRequests(convID, toolCalls)
		}
		linked[name] = true
		if earlier[name] {
			continue
		}
		c.sendMCPConnectFallback(bot, post, llmContext.RequestingUser, server)
	}
}

// earlierMCPConnectRequests returns the servers already requested through
// ConnectMCPServer in persisted turns of the conversation, excluding the
// calls in the current batch so a batch persisted before this check still
// gets its link.
func (c *Conversations) earlierMCPConnectRequests(convID string, current []llm.ToolCall) map[string]bool {
	requested := map[string]bool{}
	if c.convService == nil || convID == "" {
		return requested
	}
	turns, err := c.convService.GetTurns(convID)
	if err != nil {
		c.mmClient.LogWarn("Failed to load turns for MCP connect fallback", "error", err.Error(), "conversation_id", convID)
		return requested
	}
	return mcpConnectRequestsInTurns(turns, current)
}

func mcpConnectRequestsInTurns(turns []store.Turn, current []llm.ToolCall) map[string]bool {
	currentIDs := make(map[string]bool, len(current))
	for _, tc := range current {
		currentIDs[tc.ID] = true
	}
	requested := map[string]bool{}
	for _, turn := range turns {
		if turn.Role != "assistant" {
			continue
		}
		blocks, err := conversation.UnmarshalBlocks(turn.Content)
		if err != nil {
			continue
		}
		for _, block := range blocks {
			if block.Type != conversation.BlockTypeToolUse || block.Name != mmtools.ConnectMCPServerToolName || currentIDs[block.ID] {
				continue
			}
			if name, ok := mmtools.ConnectMCPServerName(block.Input); ok {
				requested[name] = true
			}
		}
	}
	return requested
}

func (c *Conversations) sendMCPConnectFallback(bot *bots.Bot, post *model.Post, requester *model.User, server llm.UnconnectedMCPServer) {
	mmBot := bot.GetMMBot()
	if mmBot == nil {
		return
	}
	rootID := post.RootId
	if rootID == "" {
		rootID = post.Id
	}

	name := mcpConnectFallbackMarkdownEscaper.Replace(server.Name)
	authURL := mcpConnectFallbackURLEscaper.Replace(server.AuthURL)
	message := fmt.Sprintf(mcpConnectFallbackDefaultMessage, name, name, authURL)
	if c.i18n != nil {
		T := i18n.LocalizerFunc(c.i18n, c.fallbackLocale(requester.Locale))
		message = T("agents.mcp_connect_fallback", mcpConnectFallbackDefaultMessage, name, name, authURL)
	}

	c.mmClient.SendEphemeralPost(requester.Id, &model.Post{
		UserId:    mmBot.UserId,
		ChannelId: post.ChannelId,
		RootId:    rootID,
		Message:   message,
	})
}
