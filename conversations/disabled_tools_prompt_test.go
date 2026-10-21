// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations_test

import (
	"context"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/conversations"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChannelMentionSystemPromptExplainsDisabledTools drives channel mentions
// through MessageHasBeenPosted and checks that the system prompt sent to the
// model gives guidance only for tools it can call and explains each disabled
// tool with the reason that applies.
func TestChannelMentionSystemPromptExplainsDisabledTools(t *testing.T) {
	const (
		createFileGuidance = "should use the CreateFile tool"
		outsideDMSection   = "can only be used in a Direct Message (DM) or via the Agents tab"
		unattendedSection  = "This request was started by a bot or integration"
		lookupToolLine     = "- lookup_thing: Look up a thing"
		createFileToolLine = "- CreateFile: Create a file"
	)
	resolver := func(context.Context, *llm.Context, llm.ToolArgumentGetter) (string, error) { return "", nil }
	builtInTools := []llm.Tool{
		{Name: "lookup_thing", Description: "Look up a thing", Resolver: resolver},
		{Name: "CreateFile", Description: "Create a file", AutoExecute: true, Resolver: resolver},
	}
	mention := "@" + autoReplyBotUsername + " help me"

	tests := []struct {
		name               string
		channelToolCalling bool
		policyChecker      bool
		post               func(*autoReplyTestEnv) *model.Post
		contains           []string
		notContains        []string
	}{
		{
			name: "human mention with channel tool calling off points to a DM",
			post: func(env *autoReplyTestEnv) *model.Post {
				return env.rootPost(autoReplyUserID, mention)
			},
			contains:    []string{outsideDMSection, lookupToolLine, createFileToolLine},
			notContains: []string{unattendedSection, createFileGuidance},
		},
		{
			name:               "bot activate_ai mention keeps guidance for unattended-safe tools",
			channelToolCalling: true,
			policyChecker:      true,
			post: func(env *autoReplyTestEnv) *model.Post {
				post := env.rootPost(autoReplyForeignBotID, mention)
				post.AddProp(conversations.ActivateAIProp, true)
				return post
			},
			contains:    []string{createFileGuidance, unattendedSection, lookupToolLine},
			notContains: []string{outsideDMSection, createFileToolLine},
		},
		{
			name:               "integration mention with channel tool calling on is unattended",
			channelToolCalling: true,
			post: func(env *autoReplyTestEnv) *model.Post {
				post := env.rootPost(autoReplyUserID, mention)
				post.AddProp(conversations.FromPluginProp, true)
				post.AddProp(conversations.ActivateAIProp, true)
				return post
			},
			contains:    []string{unattendedSection, lookupToolLine, createFileToolLine},
			notContains: []string{outsideDMSection, createFileGuidance},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupAutoReplyTestEnvWithTools(t,
				[]llm.BotConfig{autoReplyBotConfig()},
				builtInTools,
				&testToolCallingConfig{enableChannelMentionToolCalling: tt.channelToolCalling},
				dmMakeTextStream("done"),
			)
			if tt.policyChecker {
				env.conversations.SetToolPolicyChecker(newDMPolicyChecker())
			}

			env.conversations.MessageHasBeenPosted(nil, tt.post(env))

			require.NotEmpty(t, env.fakeLLM.requests, "the mention must reach the model")
			systemPost := env.fakeLLM.requests[0].Posts[0]
			require.Equal(t, llm.PostRoleSystem, systemPost.Role)
			for _, expected := range tt.contains {
				assert.Contains(t, systemPost.Message, expected)
			}
			for _, unexpected := range tt.notContains {
				assert.NotContains(t, systemPost.Message, unexpected)
			}
		})
	}
}
