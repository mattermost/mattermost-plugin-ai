// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-agents/v2/autoreply"
	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/conversation"
	"github.com/mattermost/mattermost-plugin-agents/v2/conversations"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mcp"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmtools"
	"github.com/mattermost/mattermost-plugin-agents/v2/store"
	"github.com/mattermost/mattermost-plugin-agents/v2/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	policyToolOrigin     = "https://mcp.example.com"
	policyToolName       = "get_weather"
	policyDMChannelID    = "ardmchannel-id"
	policySecretChannel  = "arsecret-channel-id"
	threadSourceDeniedEN = "I can't continue this conversation because you no longer have access to the thread it is about."
)

var policyWeatherCall = llm.ToolCall{
	ID:           "tc-weather",
	Name:         policyToolName,
	Arguments:    json.RawMessage(`{"city":"NYC"}`),
	ServerOrigin: policyToolOrigin,
}

// policyEnv is an auto-reply test environment with a remote MCP weather tool
// that counts its executions, a tool policy checker, and the agent's DM with
// autoReplyUserID.
type policyEnv struct {
	*autoReplyTestEnv
	dm         *model.Channel
	checker    *dmPolicyChecker
	executions *atomic.Int32
}

func policyBotConfig() llm.BotConfig {
	cfg := autoReplyBotConfig()
	cfg.AutoEnableNewMCPTools = true
	return cfg
}

func setupPolicyEnv(t *testing.T, botConfigs []llm.BotConfig, llmResponses ...*llm.TextStreamResult) *policyEnv {
	t.Helper()
	env := setupAutoReplyTestEnv(t, botConfigs, llmResponses...)

	executions := &atomic.Int32{}
	env.mcpMgr.tools = []llm.Tool{{
		Name:         policyToolName,
		Description:  "Gets the weather",
		ServerOrigin: policyToolOrigin,
		Resolver: func(context.Context, *llm.Context, llm.ToolArgumentGetter) (string, error) {
			executions.Add(1)
			return "72F and sunny", nil
		},
	}}
	checker := newDMPolicyChecker()
	env.conversations.SetToolPolicyChecker(checker)

	dm := &model.Channel{Id: policyDMChannelID, Type: model.ChannelTypeDirect, Name: autoReplyBotUserID + "__" + autoReplyUserID}
	env.mmClient.channels[dm.Id] = dm
	env.mmClient.posts = map[string]*model.Post{}
	env.streaming.finished = make(chan string, 8)

	return &policyEnv{autoReplyTestEnv: env, dm: dm, checker: checker, executions: executions}
}

func (env *policyEnv) waitForStream(t *testing.T) {
	t.Helper()
	select {
	case <-env.streaming.finished:
	case <-time.After(5 * time.Second):
		t.Fatal("response stream never finished")
	}
}

func (env *policyEnv) llmRequests() []llm.CompletionRequest {
	env.fakeLLM.mu.Lock()
	defer env.fakeLLM.mu.Unlock()
	return append([]llm.CompletionRequest(nil), env.fakeLLM.requests...)
}

// seedRun persists a conversation owned by autoReplyUserID in channel whose
// latest run was started by initiator and answered with botPost. toolUse is
// the content of botPost's assistant turn; toolResults, when set, become a
// tool_result turn after it. rootPost, when set, roots the thread instead of
// initiator.
func (env *policyEnv) seedRun(t *testing.T, channel *model.Channel, rootPost, initiator *model.Post, toolUse, toolResults []conversation.ContentBlock) *model.Post {
	t.Helper()
	if rootPost == nil {
		rootPost = initiator
	}
	initiator.ChannelId = channel.Id
	rootPost.ChannelId = channel.Id
	if initiator.Id != rootPost.Id {
		initiator.RootId = rootPost.Id
	}
	env.mmClient.posts[rootPost.Id] = rootPost
	env.mmClient.posts[initiator.Id] = initiator

	channelID := channel.Id
	rootID := rootPost.Id
	conv := &store.Conversation{
		ID:           model.NewId(),
		UserID:       autoReplyUserID,
		BotID:        autoReplyBotUserID,
		ChannelID:    &channelID,
		RootPostID:   &rootID,
		Operation:    "conversation",
		SystemPrompt: "system",
	}
	require.NoError(t, env.convStore.CreateConversation(conv))

	botPost := &model.Post{Id: model.NewId(), UserId: autoReplyBotUserID, ChannelId: channel.Id, RootId: rootID}
	botPost.AddProp(streaming.ConversationIDProp, conv.ID)
	botPost.AddProp(streaming.RespondingToProp, initiator.Id)
	env.mmClient.posts[botPost.Id] = botPost
	env.setThread(rootID, rootPost, initiator, botPost)

	userContent, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: initiator.Message}})
	require.NoError(t, err)
	assistantContent, err := json.Marshal(toolUse)
	require.NoError(t, err)
	initiatorID := initiator.Id
	botPostID := botPost.Id
	turns := []store.Turn{
		{ID: model.NewId(), ConversationID: conv.ID, PostID: &initiatorID, Role: "user", Content: userContent, Sequence: 1},
		{ID: model.NewId(), ConversationID: conv.ID, PostID: &botPostID, Role: "assistant", Content: assistantContent, Sequence: 2},
	}
	if toolResults != nil {
		resultContent, marshalErr := json.Marshal(toolResults)
		require.NoError(t, marshalErr)
		turns = append(turns, store.Turn{ID: model.NewId(), ConversationID: conv.ID, Role: "tool_result", Content: resultContent, Sequence: 3})
	}
	for i := range turns {
		require.NoError(t, env.convStore.CreateTurn(&turns[i]))
	}
	return botPost
}

func pendingWeatherBlock() []conversation.ContentBlock {
	return []conversation.ContentBlock{{
		Type:         conversation.BlockTypeToolUse,
		ID:           policyWeatherCall.ID,
		Name:         policyWeatherCall.Name,
		ServerOrigin: policyWeatherCall.ServerOrigin,
		Input:        policyWeatherCall.Arguments,
		Status:       conversation.StatusPending,
		Shared:       new(false),
	}}
}

func executedWeatherBlocks() ([]conversation.ContentBlock, []conversation.ContentBlock) {
	toolUse := pendingWeatherBlock()
	toolUse[0].Status = conversation.StatusSuccess
	return toolUse, []conversation.ContentBlock{{
		Type:      conversation.BlockTypeToolResult,
		ToolUseID: policyWeatherCall.ID,
		Content:   "72F and sunny",
		Status:    conversation.StatusSuccess,
		Shared:    new(false),
	}}
}

func humanPost(message string) *model.Post {
	return &model.Post{Id: model.NewId(), UserId: autoReplyUserID, Message: message, CreateAt: 100}
}

func botActivatePost(message string) *model.Post {
	post := &model.Post{Id: model.NewId(), UserId: autoReplyForeignBotID, Message: message, CreateAt: 200}
	post.AddProp(conversations.ActivateAIProp, "true")
	return post
}

// resumeEntryPoint drives one way of resuming a run on botPost as
// autoReplyUserID. accept approves (or shares) the run's tool call instead of
// declining it; regenerate ignores it.
type resumeEntryPoint struct {
	name string
	// seed returns the assistant and tool_result turn content for a run this
	// entry point can resume.
	seed   func() ([]conversation.ContentBlock, []conversation.ContentBlock)
	resume func(env *policyEnv, botPost *model.Post, channel *model.Channel, accept bool) error
}

func acceptedWeatherCall(accept bool) []string {
	if accept {
		return []string{policyWeatherCall.ID}
	}
	return []string{}
}

var resumeEntryPoints = []resumeEntryPoint{
	{
		name: "regenerate",
		seed: func() ([]conversation.ContentBlock, []conversation.ContentBlock) {
			return []conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: "earlier answer"}}, nil
		},
		resume: func(env *policyEnv, botPost *model.Post, channel *model.Channel, _ bool) error {
			return env.conversations.HandleRegenerate(context.Background(), autoReplyUserID, botPost, channel)
		},
	},
	{
		name: "tool approval",
		seed: func() ([]conversation.ContentBlock, []conversation.ContentBlock) {
			return pendingWeatherBlock(), nil
		},
		resume: func(env *policyEnv, botPost *model.Post, channel *model.Channel, accept bool) error {
			return env.conversations.HandleToolCall(context.Background(), autoReplyUserID, botPost, channel, acceptedWeatherCall(accept), nil)
		},
	},
	{
		name: "tool result share",
		seed: executedWeatherBlocks,
		resume: func(env *policyEnv, botPost *model.Post, channel *model.Channel, accept bool) error {
			return env.conversations.HandleToolResult(context.Background(), autoReplyUserID, botPost, channel, acceptedWeatherCall(accept))
		},
	},
}

// requireRunUntouched asserts a denied resume called no LLM, ran no tool, and
// left every persisted turn of the run as it was.
func requireRunUntouched(t *testing.T, env *policyEnv, botPost *model.Post, before []store.Turn) {
	t.Helper()
	assert.Empty(t, env.llmRequests(), "a denied resume must not call the LLM")
	assert.Zero(t, env.executions.Load(), "a denied resume must not run tools")
	convID, _ := botPost.GetProp(streaming.ConversationIDProp).(string)
	assert.Equal(t, before, env.convStore.turnsFor(convID), "a denied resume must not change the run")
}

// TestResumeChecksUsageRestrictionsForThePostsAgent pins that every resume
// re-checks whether the requester may still use the agent that wrote the post
// (bug 1): an agent that now blocks the user cannot be regenerated, approved,
// or shared, in a DM or a channel.
func TestResumeChecksUsageRestrictionsForThePostsAgent(t *testing.T) {
	for _, entry := range resumeEntryPoints {
		for _, inDM := range []bool{true, false} {
			name := entry.name + "/channel"
			if inDM {
				name = entry.name + "/DM"
			}
			t.Run(name, func(t *testing.T) {
				cfg := policyBotConfig()
				cfg.UserAccessLevel = llm.UserAccessLevelBlock
				cfg.UserIDs = []string{autoReplyUserID}
				env := setupPolicyEnv(t, []llm.BotConfig{cfg}, dmMakeTextStream("regenerated"))
				env.config.enableChannelMentionToolCalling = true
				channel := env.channel
				if inDM {
					channel = env.dm
				}

				toolUse, toolResults := entry.seed()
				botPost := env.seedRun(t, channel, nil, humanPost("what is the weather"), toolUse, toolResults)
				convID, _ := botPost.GetProp(streaming.ConversationIDProp).(string)
				before := env.convStore.turnsFor(convID)

				err := entry.resume(env, botPost, channel, true)

				require.ErrorIs(t, err, bots.ErrUsageRestriction)
				requireRunUntouched(t, env, botPost, before)
			})
		}
	}
}

// TestResumeInChannelRequiresMultiplayerLicense pins that resuming a channel
// run needs the same license as starting one (bug 4), while DM resumes do
// not.
func TestResumeInChannelRequiresMultiplayerLicense(t *testing.T) {
	for _, entry := range resumeEntryPoints {
		t.Run(entry.name, func(t *testing.T) {
			env := setupPolicyEnv(t, []llm.BotConfig{policyBotConfig()}, dmMakeTextStream("regenerated"))
			env.overrideLicense(nil)
			env.config.enableChannelMentionToolCalling = true

			toolUse, toolResults := entry.seed()
			botPost := env.seedRun(t, env.channel, nil, humanPost("what is the weather"), toolUse, toolResults)
			convID, _ := botPost.GetProp(streaming.ConversationIDProp).(string)
			before := env.convStore.turnsFor(convID)

			// Declining needs no remote MCP license, so only the channel
			// license can deny it.
			err := entry.resume(env, botPost, env.channel, false)

			var licErr *enterprise.LicenseError
			require.ErrorAs(t, err, &licErr)
			assert.Equal(t, enterprise.CapMultiplayerChannels, licErr.Capability)
			requireRunUntouched(t, env, botPost, before)
		})
	}

	t.Run("regenerate in a DM", func(t *testing.T) {
		env := setupPolicyEnv(t, []llm.BotConfig{policyBotConfig()}, dmMakeTextStream("regenerated"))
		env.overrideLicense(nil)

		toolUse, _ := resumeEntryPoints[0].seed()
		botPost := env.seedRun(t, env.dm, nil, humanPost("what is the weather"), toolUse, nil)

		require.NoError(t, env.conversations.HandleRegenerate(context.Background(), autoReplyUserID, botPost, env.dm))
		assert.Len(t, env.llmRequests(), 1)
	})
}

// TestDMMentionOfTheDMAgentIsADMTurn pins that @-mentioning an agent inside
// its own DM runs a DM turn with DM tool rules (bug 2), while mentioning a
// different agent in that DM still runs a channel-style mention turn.
func TestDMMentionOfTheDMAgentIsADMTurn(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		wantExecutions int32
	}{
		{name: "plain message", message: "what is the weather", wantExecutions: 1},
		{name: "message mentioning the DM's agent", message: "@" + autoReplyBotUsername + " what is the weather", wantExecutions: 1},
		{name: "message mentioning another agent", message: "@" + autoReplyBot2Username + " what is the weather", wantExecutions: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			second := autoReplySecondBotConfig()
			second.AutoEnableNewMCPTools = true
			env := setupPolicyEnv(t, []llm.BotConfig{policyBotConfig(), second},
				dmMakeToolCallStream([]llm.ToolCall{policyWeatherCall}),
				dmMakeTextStream("It's sunny"),
			)
			env.checker.setAutoRun(policyToolOrigin, policyToolName)

			post := &model.Post{Id: model.NewId(), UserId: autoReplyUserID, ChannelId: env.dm.Id, Message: tc.message}
			env.setThread(post.Id, post)
			env.conversations.MessageHasBeenPosted(nil, post)
			env.waitForStream(t)

			assert.Equal(t, tc.wantExecutions, env.executions.Load())
		})
	}
}

// TestToolApprovalUsesTheRunsActivation pins that whether a resumed channel
// run is limited to unattended tools follows the post that started that run,
// not the thread root (bug 3): approving a tool call from a run a bot started
// with activate_ai cannot run tools that run would not have been offered, and
// a run whose initiator cannot be verified fails closed.
func TestToolApprovalUsesTheRunsActivation(t *testing.T) {
	tests := []struct {
		name           string
		root           func() *model.Post
		initiator      func() *model.Post
		hideInitiator  bool
		wantExecutions int32
	}{
		{
			name:           "human run in a human thread",
			initiator:      func() *model.Post { return humanPost("what is the weather") },
			wantExecutions: 1,
		},
		{
			name:           "human run in a thread a bot started",
			root:           func() *model.Post { return botActivatePost("weather report please") },
			initiator:      func() *model.Post { return humanPost("what is the weather") },
			wantExecutions: 1,
		},
		{
			name:           "bot activate_ai run in a human thread",
			root:           func() *model.Post { return humanPost("hello") },
			initiator:      func() *model.Post { return botActivatePost("what is the weather") },
			wantExecutions: 0,
		},
		{
			name:           "run whose initiating post is gone",
			root:           func() *model.Post { return humanPost("hello") },
			initiator:      func() *model.Post { return humanPost("what is the weather") },
			hideInitiator:  true,
			wantExecutions: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := setupPolicyEnv(t, []llm.BotConfig{policyBotConfig()})
			env.config.enableChannelMentionToolCalling = true
			env.checker.setPolicy(policyToolOrigin, policyToolName, mcp.ToolPolicyAsk)

			var root *model.Post
			if tc.root != nil {
				root = tc.root()
			}
			initiator := tc.initiator()
			botPost := env.seedRun(t, env.channel, root, initiator, pendingWeatherBlock(), nil)
			if tc.hideInitiator {
				delete(env.mmClient.posts, initiator.Id)
			}

			require.NoError(t, env.conversations.HandleToolCall(context.Background(), autoReplyUserID, botPost, env.channel, []string{policyWeatherCall.ID}, nil))

			assert.Equal(t, tc.wantExecutions, env.executions.Load())
		})
	}
}

// TestThreadAnalysisFollowUpRechecksSourceAccess pins that continuing a
// thread summary re-checks that the user can still read the summarized
// thread (bug 5), and fails closed when the thread cannot be found.
func TestThreadAnalysisFollowUpRechecksSourceAccess(t *testing.T) {
	const threadPostID = "arthread-post-id"

	tests := []struct {
		name         string
		unreadable   bool
		threadGone   bool
		regenerate   bool
		wantAnswered bool
	}{
		{name: "follow-up while the thread is readable", wantAnswered: true},
		{name: "follow-up after losing access to the thread", unreadable: true},
		{name: "follow-up after the thread was deleted", threadGone: true},
		{name: "regenerate after losing access to the thread", unreadable: true, regenerate: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := setupPolicyEnv(t, []llm.BotConfig{policyBotConfig()}, dmMakeTextStream("follow-up answer"))

			if !tc.threadGone {
				env.mmClient.posts[threadPostID] = &model.Post{Id: threadPostID, ChannelId: policySecretChannel, UserId: autoReplyOtherUserID, Message: "secret plans"}
			}
			if tc.unreadable {
				env.mmClient.unreadableChannels = map[string]bool{policySecretChannel: true}
			}

			analysisPost := &model.Post{Id: model.NewId(), UserId: autoReplyBotUserID, ChannelId: env.dm.Id, Message: "summary of secret plans"}
			analysisPost.AddProp(conversations.ThreadIDProp, threadPostID)
			analysisPost.AddProp(conversations.AnalysisTypeProp, "summarize_thread")
			channelID := env.dm.Id
			analysisID := analysisPost.Id
			conv := &store.Conversation{
				ID:           model.NewId(),
				UserID:       autoReplyUserID,
				BotID:        autoReplyBotUserID,
				ChannelID:    &channelID,
				RootPostID:   &analysisID,
				Operation:    llm.OperationThreadAnalysis,
				SystemPrompt: "system",
			}
			require.NoError(t, env.convStore.CreateConversation(conv))
			analysisPost.AddProp(streaming.ConversationIDProp, conv.ID)
			env.mmClient.posts[analysisPost.Id] = analysisPost
			userContent, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: "summarize"}})
			require.NoError(t, err)
			summaryContent, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: analysisPost.Message}})
			require.NoError(t, err)
			require.NoError(t, env.convStore.CreateTurn(&store.Turn{ID: model.NewId(), ConversationID: conv.ID, Role: "user", Content: userContent, Sequence: 1}))
			require.NoError(t, env.convStore.CreateTurn(&store.Turn{ID: model.NewId(), ConversationID: conv.ID, PostID: &analysisID, Role: "assistant", Content: summaryContent, Sequence: 2}))

			if tc.regenerate {
				err = env.conversations.HandleRegenerate(context.Background(), autoReplyUserID, analysisPost, env.dm)
				require.Error(t, err)
				assert.Empty(t, env.llmRequests())
				return
			}

			reply := &model.Post{Id: model.NewId(), UserId: autoReplyUserID, ChannelId: env.dm.Id, RootId: analysisPost.Id, Message: "what should we do next?"}
			env.conversations.MessageHasBeenPosted(nil, reply)

			if tc.wantAnswered {
				env.waitForStream(t)
				assert.Len(t, env.llmRequests(), 1)
				return
			}
			assert.Empty(t, env.llmRequests(), "the summarized thread must not be replayed to the LLM")
			require.NotEmpty(t, env.mmClient.updatedPosts, "the placeholder must explain why there is no answer")
			placeholder := env.mmClient.updatedPosts[len(env.mmClient.updatedPosts)-1]
			assert.Equal(t, threadSourceDeniedEN, placeholder.Message)
		})
	}
}

const policyTimeToolName = "get_time"

// turnEntryPoint starts or resumes a turn the way one production entry point
// does. Every run asks about the weather, so the LLM's canned responses apply
// to each.
type turnEntryPoint struct {
	name string
	dm   bool
	// returnsErr is set for entry points that report a denial to their
	// caller; message hooks only decline.
	returnsErr bool
	start      func(t *testing.T, env *policyEnv) error
}

func policyBot(env *policyEnv) *bots.Bot {
	return env.botService.GetBotByID(autoReplyBotUserID)
}

func postMessage(env *policyEnv, post *model.Post) error {
	env.conversations.MessageHasBeenPosted(nil, post)
	return nil
}

func dmMessage(env *policyEnv, message string) *model.Post {
	post := &model.Post{Id: model.NewId(), UserId: autoReplyUserID, ChannelId: env.dm.Id, Message: message}
	env.setThread(post.Id, post)
	return post
}

func pendingTimeBlock() []conversation.ContentBlock {
	return []conversation.ContentBlock{{
		Type:         conversation.BlockTypeToolUse,
		ID:           "tc-time",
		Name:         llm.NamespaceMCPToolName("mattermost", policyTimeToolName),
		ServerOrigin: mcp.EmbeddedClientKey,
		Input:        json.RawMessage(`{}`),
		Status:       conversation.StatusPending,
		Shared:       new(false),
	}}
}

func pendingQuestionBlock() []conversation.ContentBlock {
	return []conversation.ContentBlock{{
		Type:            conversation.BlockTypeToolUse,
		ID:              "q-1",
		Name:            "AskUserQuestion",
		Input:           json.RawMessage(`{"question":"Which city?","options":[{"label":"NYC"},{"label":"Paris"}]}`),
		Status:          conversation.StatusPending,
		UserInteraction: llm.UserInteractionSelect,
		Shared:          new(false),
	}}
}

var turnEntryPoints = []turnEntryPoint{
	{
		name: "DM message",
		dm:   true,
		start: func(_ *testing.T, env *policyEnv) error {
			return postMessage(env, dmMessage(env, "what is the weather"))
		},
	},
	{
		name: "@agent in its own DM",
		dm:   true,
		start: func(_ *testing.T, env *policyEnv) error {
			return postMessage(env, dmMessage(env, "@"+autoReplyBotUsername+" what is the weather"))
		},
	},
	{
		name: "channel mention",
		start: func(_ *testing.T, env *policyEnv) error {
			return postMessage(env, env.rootPost(autoReplyUserID, "@"+autoReplyBotUsername+" what is the weather"))
		},
	},
	{
		name: "channel auto-reply",
		start: func(_ *testing.T, env *policyEnv) error {
			env.settings.set(autoreply.Setting{ChannelID: autoReplyChannelID, BotID: autoReplyBotUserID, Mode: autoreply.ModeRootPosts})
			return postMessage(env, env.rootPost(autoReplyUserID, "what is the weather"))
		},
	},
	{
		name:       "loop-in from the mention reminder",
		returnsErr: true,
		start: func(_ *testing.T, env *policyEnv) error {
			reply := env.threadReply(autoReplyUserID, "what is the weather", true)
			return env.conversations.HandleLoopInAgent(context.Background(), autoReplyUserID, policyBot(env), reply, env.channel)
		},
	},
	{
		name: "bot activate_ai mention",
		start: func(_ *testing.T, env *policyEnv) error {
			post := env.rootPost(autoReplyForeignBotID, "@"+autoReplyBotUsername+" what is the weather")
			post.AddProp(conversations.ActivateAIProp, true)
			return postMessage(env, post)
		},
	},
	{
		name:       "regenerate in a DM",
		dm:         true,
		returnsErr: true,
		start: func(t *testing.T, env *policyEnv) error {
			botPost := env.seedRun(t, env.dm, nil, humanPost("what is the weather"), []conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: "earlier answer"}}, nil)
			return env.conversations.HandleRegenerate(context.Background(), autoReplyUserID, botPost, env.dm)
		},
	},
	{
		name:       "regenerate in a channel",
		returnsErr: true,
		start: func(t *testing.T, env *policyEnv) error {
			botPost := env.seedRun(t, env.channel, nil, humanPost("what is the weather"), []conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: "earlier answer"}}, nil)
			return env.conversations.HandleRegenerate(context.Background(), autoReplyUserID, botPost, env.channel)
		},
	},
	{
		name:       "tool approve",
		dm:         true,
		returnsErr: true,
		start: func(t *testing.T, env *policyEnv) error {
			botPost := env.seedRun(t, env.dm, nil, humanPost("what time is it"), pendingTimeBlock(), nil)
			return env.conversations.HandleToolCall(context.Background(), autoReplyUserID, botPost, env.dm, []string{"tc-time"}, nil)
		},
	},
	{
		name:       "ask-user answer",
		returnsErr: true,
		start: func(t *testing.T, env *policyEnv) error {
			botPost := env.seedRun(t, env.channel, nil, humanPost("what is the weather"), pendingQuestionBlock(), nil)
			answers := map[string]mmtools.UserInteractionAnswer{"q-1": {Selected: []string{"NYC"}}}
			return env.conversations.HandleToolCall(context.Background(), autoReplyUserID, botPost, env.channel, []string{"q-1"}, answers)
		},
	},
	{
		name:       "WouldAutoExecute auto-resume",
		returnsErr: true,
		start: func(t *testing.T, env *policyEnv) error {
			marked := pendingWeatherBlock()
			marked[0].WouldAutoExecute = true
			botPost := env.seedRun(t, env.channel, nil, humanPost("what is the weather"), append(marked, pendingQuestionBlock()...), nil)
			answers := map[string]mmtools.UserInteractionAnswer{"q-1": {Selected: []string{"NYC"}}}
			return env.conversations.HandleToolCall(context.Background(), autoReplyUserID, botPost, env.channel, []string{"q-1"}, answers)
		},
	},
	{
		name:       "share follow-up",
		returnsErr: true,
		start: func(t *testing.T, env *policyEnv) error {
			toolUse, toolResults := executedWeatherBlocks()
			botPost := env.seedRun(t, env.channel, nil, humanPost("what is the weather"), toolUse, toolResults)
			return env.conversations.HandleToolResult(context.Background(), autoReplyUserID, botPost, env.channel, []string{policyWeatherCall.ID})
		},
	},
	{
		name:       "search",
		dm:         true,
		returnsErr: true,
		start: func(_ *testing.T, env *policyEnv) error {
			_, err := env.conversations.HandleSearch(context.Background(), policyBot(env), env.mmClient.users[autoReplyUserID], conversations.SearchRequest{Query: "what is the weather"})
			return err
		},
	},
}

// setupInvariantEnv is a policy environment with channel tool calling on, an
// embedded time tool and search tool beside the remote weather tool, and DMs
// for search created in the agent's DM.
func setupInvariantEnv(t *testing.T, cfg llm.BotConfig, llmResponses ...*llm.TextStreamResult) *policyEnv {
	t.Helper()
	env := setupPolicyEnv(t, []llm.BotConfig{cfg}, llmResponses...)
	env.config.enableChannelMentionToolCalling = true
	env.mcpMgr.tools = append(env.mcpMgr.tools,
		embeddedTestTool(policyTimeToolName, "noon"),
		embeddedTestTool("search_posts", "no results"),
	)
	env.checker.setPolicy(mcp.EmbeddedClientKey, policyTimeToolName, mcp.ToolPolicyAsk)
	env.mmClient.dmChannelID = env.dm.Id
	return env
}

// TestTurnEntryPointPolicyInvariants pins that every entry point that starts
// or resumes a turn applies the same turn policy: the requester's access to
// the agent, the multiplayer license for channel turns, and DM-only auto-run
// tool policy.
func TestTurnEntryPointPolicyInvariants(t *testing.T) {
	for _, entry := range turnEntryPoints {
		t.Run(entry.name, func(t *testing.T) {
			t.Run("an agent that blocks the requester runs nothing", func(t *testing.T) {
				cfg := policyBotConfig()
				cfg.UserAccessLevel = llm.UserAccessLevelBlock
				cfg.UserIDs = []string{autoReplyUserID, autoReplyForeignBotID}
				env := setupInvariantEnv(t, cfg, dmMakeTextStream("answer"))

				err := entry.start(t, env)

				if entry.returnsErr {
					require.ErrorIs(t, err, bots.ErrUsageRestriction)
				}
				assert.Empty(t, env.llmRequests())
				assert.Zero(t, env.executions.Load())
			})

			t.Run("only DM turns run without the multiplayer license", func(t *testing.T) {
				env := setupInvariantEnv(t, policyBotConfig(), dmMakeTextStream("answer"))
				env.overrideLicense(nil)

				err := entry.start(t, env)

				if !entry.dm {
					if entry.returnsErr {
						var licErr *enterprise.LicenseError
						require.ErrorAs(t, err, &licErr)
						assert.Equal(t, enterprise.CapMultiplayerChannels, licErr.Capability)
					}
					assert.Empty(t, env.llmRequests())
					return
				}
				require.NoError(t, err)
				env.waitForStream(t)
				assert.NotEmpty(t, env.llmRequests())
			})

			t.Run("a DM-only auto-run tool auto-runs only in DM turns", func(t *testing.T) {
				env := setupInvariantEnv(t, policyBotConfig(),
					dmMakeToolCallStream([]llm.ToolCall{policyWeatherCall}),
					dmMakeTextStream("It's sunny"),
				)
				env.checker.setPolicy(policyToolOrigin, policyToolName, mcp.ToolPolicyAutoRunInDM)

				require.NoError(t, entry.start(t, env))
				env.waitForStream(t)

				require.NotEmpty(t, env.llmRequests())
				want := int32(0)
				if entry.dm {
					want = 1
				}
				assert.Equal(t, want, env.executions.Load())
			})
		})
	}
}
