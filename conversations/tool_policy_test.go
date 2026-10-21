// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise/enterprisetest"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mcp"
	"github.com/mattermost/mattermost-plugin-agents/v2/toolrunner"
	"github.com/stretchr/testify/assert"
)

// policyTestConversations is a Conversations licensed for remote MCP, so the
// policy decisions below are not masked by the license gate.
func policyTestConversations(checker mcp.ToolPolicyChecker) *Conversations {
	return &Conversations{toolPolicyChecker: checker, licenseChecker: enterprisetest.CheckerAt(enterprise.LevelEnterprise)}
}

// testTurnPolicy is the policy of a turn with tools enabled in a DM or a
// channel.
func testTurnPolicy(c *Conversations, isDM bool) *turnPolicy {
	return &turnPolicy{c: c, isDM: isDM, toolsEnabled: true}
}

// testDMTurnPolicy is the policy resolveTurnPolicy gives a DM turn with bot,
// for tests that exercise the tool loop rather than policy resolution.
func testDMTurnPolicy(c *Conversations, bot *bots.Bot) *turnPolicy {
	return &turnPolicy{
		c:            c,
		turnRequest:  turnRequest{bot: bot},
		isDM:         true,
		toolsEnabled: true,
		interactive:  true,
		maxRounds:    bot.GetConfig().EffectiveMaxToolTurns(),
	}
}

// TestAutoExecute captures the channel-vs-DM policy matrix. In
// DMs, both auto_run and auto_run_everywhere bypass approval. In channels,
// only auto_run_everywhere bypasses approval — auto_run tools fall through
// to the manual approve → share flow so the channel-visible follow-up
// cannot reveal unshared tool output.
func TestAutoExecute(t *testing.T) {
	const origin = "https://mcp.example.com/mcp"
	const toolName = "example_tool"

	cases := []struct {
		name    string
		isDM    bool
		policy  string
		enabled bool
		want    bool
	}{
		{name: "DM + auto_run_in_dm enabled -> auto-execute", isDM: true, policy: mcp.ToolPolicyAutoRunInDM, enabled: true, want: true},
		{name: "DM + auto_run_everywhere enabled -> auto-execute", isDM: true, policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true, want: true},
		{name: "DM + ask -> approve", isDM: true, policy: mcp.ToolPolicyAsk, enabled: true, want: false},
		{name: "DM + disabled -> approve", isDM: true, policy: mcp.ToolPolicyAutoRunInDM, enabled: false, want: false},

		{name: "channel + auto_run_in_dm enabled -> approve (DM-only policy)", isDM: false, policy: mcp.ToolPolicyAutoRunInDM, enabled: true, want: false},
		{name: "channel + auto_run_everywhere enabled -> auto-execute", isDM: false, policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true, want: true},
		{name: "channel + ask -> approve", isDM: false, policy: mcp.ToolPolicyAsk, enabled: true, want: false},
		{name: "channel + auto_run_everywhere disabled -> approve", isDM: false, policy: mcp.ToolPolicyAutoRunEverywhere, enabled: false, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := policyTestConversations(mapPolicyChecker{
				origin: {
					toolName: {policy: tc.policy, enabled: tc.enabled},
				},
			})
			llmCtx := &llm.Context{Tools: llm.NewToolStore()}
			llmCtx.Tools.AddTools([]llm.Tool{{Name: toolName, ServerOrigin: origin}})
			callback := testTurnPolicy(c, tc.isDM).autoExecute(llmCtx)
			got := callback(llm.ToolCall{Name: toolName, ServerOrigin: origin})
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestAutoExecute_NilChecker covers the fail-closed branch — with
// no policy checker wired up, no tool should ever auto-execute.
func TestAutoExecute_NilChecker(t *testing.T) {
	c := policyTestConversations(nil)
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: "x", ServerOrigin: "y"}})
	for _, isDM := range []bool{true, false} {
		got := testTurnPolicy(c, isDM).autoExecute(llmCtx)(llm.ToolCall{Name: "x", ServerOrigin: "y"})
		assert.False(t, got, "isDM=%v", isDM)
	}
}

func TestAutoExecute_MetaToolsAutoExecute(t *testing.T) {
	c := policyTestConversations(nil)
	store := llm.NewToolStore()
	store.AddTools(mcp.NewMetaTools(nil))
	llmCtx := &llm.Context{Tools: store}

	for _, isDM := range []bool{true, false} {
		got := testTurnPolicy(c, isDM).autoExecute(llmCtx)(llm.ToolCall{Name: mcp.LoadToolName})
		assert.True(t, got, "isDM=%v", isDM)
	}

	got := testTurnPolicy(c, true).autoExecute(llmCtx)(llm.ToolCall{Name: mcp.LoadToolName, ServerOrigin: "https://mcp.example.com/mcp"})
	assert.False(t, got)
}

func TestAutoExecute_NamespacedToolUsesBarePolicy(t *testing.T) {
	const origin = "https://mcp.example.com/mcp"

	c := policyTestConversations(mapPolicyChecker{
		origin: {
			"example_tool": {policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true},
		},
	})
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: "example__example_tool", ServerOrigin: origin}})

	got := testTurnPolicy(c, false).autoExecute(llmCtx)(llm.ToolCall{Name: "example__example_tool"})

	assert.True(t, got)
}

// TestAutoExecute_ConfigPolicyChecker runs the production
// config-backed checker against runtime tool names as the MCP client builds
// them. Plugin tools carry a "{pluginID}__" prefix from pluginmcp, so their
// configured names contain the namespace separator themselves.
func TestAutoExecute_ConfigPolicyChecker(t *testing.T) {
	const (
		pluginOrigin = "plugin://com.example.demo"
		remoteOrigin = "https://mcp.example.com/mcp"
	)

	cases := []struct {
		name           string
		origin         string
		serverSlug     string
		configuredName string
		policy         string
		unlicensed     bool
		wantDM         bool
		wantChannel    bool
	}{
		{name: "plugin tool auto_run_everywhere without policy license falls back to ask", origin: pluginOrigin, serverSlug: "demo_plugin", configuredName: "com_example_demo__add", policy: mcp.ToolPolicyAutoRunEverywhere, unlicensed: true, wantDM: false, wantChannel: false},
		{name: "plugin tool auto_run_everywhere", origin: pluginOrigin, serverSlug: "demo_plugin", configuredName: "com_example_demo__add", policy: mcp.ToolPolicyAutoRunEverywhere, wantDM: true, wantChannel: true},
		{name: "plugin tool auto_run_in_dm", origin: pluginOrigin, serverSlug: "demo_plugin", configuredName: "com_example_demo__add", policy: mcp.ToolPolicyAutoRunInDM, wantDM: true, wantChannel: false},
		{name: "plugin tool ask", origin: pluginOrigin, serverSlug: "demo_plugin", configuredName: "com_example_demo__add", policy: mcp.ToolPolicyAsk, wantDM: false, wantChannel: false},
		{name: "remote tool name containing separator", origin: remoteOrigin, serverSlug: "remote", configuredName: "issues__create", policy: mcp.ToolPolicyAutoRunEverywhere, wantDM: true, wantChannel: true},
		{name: "remote tool plain name", origin: remoteOrigin, serverSlug: "remote", configuredName: "get_issue", policy: mcp.ToolPolicyAutoRunEverywhere, wantDM: true, wantChannel: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toolConfigs := []mcp.ToolConfig{{Name: tc.configuredName, Policy: tc.policy, Enabled: true}}
			cfg := mcp.Config{}
			if tc.origin == pluginOrigin {
				cfg.PluginServers = []mcp.PluginServerConfig{{PluginID: "com.example.demo", Name: "Demo Plugin", Enabled: true, ToolConfigs: toolConfigs}}
			} else {
				cfg.Servers = []mcp.ServerConfig{{Name: "Remote", Enabled: true, BaseURL: remoteOrigin, ToolConfigs: toolConfigs}}
			}

			checker := mcp.NewConfigToolPolicyChecker(func() mcp.Config { return cfg }, func() bool { return !tc.unlicensed })
			c := policyTestConversations(checker)
			runtimeName := llm.NamespaceMCPToolName(tc.serverSlug, tc.configuredName)
			llmCtx := &llm.Context{Tools: llm.NewToolStore()}
			llmCtx.Tools.AddTools([]llm.Tool{{Name: runtimeName, ServerOrigin: tc.origin}})
			call := llm.ToolCall{Name: runtimeName, ServerOrigin: tc.origin}

			assert.Equal(t, tc.wantDM, testTurnPolicy(c, true).autoExecute(llmCtx)(call), "DM")
			assert.Equal(t, tc.wantChannel, testTurnPolicy(c, false).autoExecute(llmCtx)(call), "channel")
		})
	}
}

func TestAutoExecuteMetaToolsBypassPolicy(t *testing.T) {
	c := policyTestConversations(nil)

	assert.True(t, testTurnPolicy(c, true).autoExecute(nil)(llm.ToolCall{Name: mcp.SearchToolsName}))
	assert.True(t, testTurnPolicy(c, false).autoExecute(nil)(llm.ToolCall{Name: mcp.LoadToolName}))
}

func TestAutoExecuteMetaToolDoesNotAuthorizeBusinessTool(t *testing.T) {
	c := policyTestConversations(nil)

	assert.False(t, testTurnPolicy(c, true).autoExecute(nil)(llm.ToolCall{Name: "jira__get_issue"}))
}

// TestAutoExecute_AutoExecuteBuiltIn pins that auto-execute
// built-ins (e.g. CreateFile) bypass approval in both DMs and channels — even
// with no policy checker wired up — while an MCP tool carrying the flag must
// still go through policy and therefore fails closed here.
func TestAutoExecute_AutoExecuteBuiltIn(t *testing.T) {
	const mcpOrigin = "https://mcp.example.com/mcp"

	cases := []struct {
		name   string
		isDM   bool
		origin string
		want   bool
	}{
		{name: "DM built-in auto-executes", isDM: true, want: true},
		{name: "channel built-in auto-executes", isDM: false, want: true},
		{name: "DM MCP tool with flag follows policy", isDM: true, origin: mcpOrigin, want: false},
		{name: "channel MCP tool with flag follows policy", isDM: false, origin: mcpOrigin, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := policyTestConversations(nil)
			llmCtx := &llm.Context{Tools: llm.NewToolStore()}
			llmCtx.Tools.AddTools([]llm.Tool{{Name: "CreateFile", ServerOrigin: tc.origin, AutoExecute: true}})

			got := testTurnPolicy(c, tc.isDM).autoExecute(llmCtx)(llm.ToolCall{Name: "CreateFile", ServerOrigin: tc.origin})

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAutoExecute_AutoExecuteBuiltInSkipsPolicyLookup(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAsk, enabled: true}
	c := policyTestConversations(checker)
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: "CreateFile", AutoExecute: true}})

	got := testTurnPolicy(c, false).autoExecute(llmCtx)(llm.ToolCall{Name: "CreateFile"})

	assert.True(t, got)
	assert.Zero(t, checker.calls)
}

// TestAutoExecute_UserInteractionNeverAutoExecutes pins the contract
// that tools answered by the user (e.g. AskUserQuestion) never auto-execute,
// even if a policy claims auto_run — auto-running one would skip the question.
func TestAutoExecute_UserInteractionNeverAutoExecutes(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: "AskUserQuestion", UserInteraction: llm.UserInteractionSelect}})

	for _, isDM := range []bool{true, false} {
		got := testTurnPolicy(c, isDM).autoExecute(llmCtx)(llm.ToolCall{Name: "AskUserQuestion"})
		assert.False(t, got, "isDM=%v", isDM)
	}
}

type countingPolicyChecker struct {
	calls        int
	lastOrigin   string
	lastToolName string
	policy       string
	enabled      bool
}

func (c *countingPolicyChecker) GetToolPolicy(origin, toolName string) (string, bool) {
	c.calls++
	c.lastOrigin = origin
	c.lastToolName = toolName
	return c.policy, c.enabled
}

func TestAutoExecute_UnknownToolSkipsPolicyLookup(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}

	got := testTurnPolicy(c, true).autoExecute(llmCtx)(llm.ToolCall{Name: "unknown_tool", ServerOrigin: "https://mcp.example.com"})

	assert.False(t, got)
	assert.Zero(t, checker.calls)
}

func TestAutoExecute_KnownToolUsesPolicy(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunInDM, enabled: true}
	c := policyTestConversations(checker)
	const origin = "https://mcp.example.com"
	const toolName = "known_tool"
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: toolName, ServerOrigin: origin}})

	got := testTurnPolicy(c, true).autoExecute(llmCtx)(llm.ToolCall{Name: toolName})

	assert.True(t, got)
	assert.Equal(t, 1, checker.calls)
}

func TestAutoExecuteDenormalizesNamespacedTool(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	const origin = "https://mcp.atlassian.com"
	const runtimeToolName = "jira__get_issue"
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: runtimeToolName, ServerOrigin: origin}})

	got := testTurnPolicy(c, false).autoExecute(llmCtx)(llm.ToolCall{Name: runtimeToolName})

	assert.True(t, got)
	assert.Equal(t, 1, checker.calls)
	assert.Equal(t, "get_issue", checker.lastToolName)
}

func TestAutoExecuteFailsClosedOnAmbiguousBareName(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{
		{Name: "jira__get_issue", ServerOrigin: "https://jira.example.com"},
		{Name: "github__get_issue", ServerOrigin: "https://github.example.com"},
	})

	got := testTurnPolicy(c, false).autoExecute(llmCtx)(llm.ToolCall{Name: "get_issue"})

	assert.False(t, got)
	assert.Zero(t, checker.calls)
}

func TestAutoExecuteUsesServerOriginToDisambiguateBareName(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	const origin = "https://github.example.com"
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{
		{Name: "jira__get_issue", ServerOrigin: "https://jira.example.com"},
		{Name: "github__get_issue", ServerOrigin: origin},
	})

	got := testTurnPolicy(c, false).autoExecute(llmCtx)(llm.ToolCall{Name: "get_issue", ServerOrigin: origin})

	assert.True(t, got)
	assert.Equal(t, 1, checker.calls)
	assert.Equal(t, origin, checker.lastOrigin)
	assert.Equal(t, "get_issue", checker.lastToolName)
}

// TestSharedForTurns_RespectsEnabledFlag pins the result-sharing
// contract: a disabled tool must never drive results to shared=true, even if
// its policy is auto_run_everywhere. The enabled flag is authoritative —
// matching autoExecute, which also refuses to auto-execute a
// disabled tool.
func TestSharedForTurns_RespectsEnabledFlag(t *testing.T) {
	const origin = "https://mcp.example.com/mcp"
	const toolName = "example_tool"

	c := policyTestConversations(mapPolicyChecker{
		origin: {
			toolName: {policy: mcp.ToolPolicyAutoRunEverywhere, enabled: false},
		},
	})
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: toolName, ServerOrigin: origin}})

	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{{Name: toolName, ServerOrigin: origin}},
	}}

	assert.False(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx),
		"a disabled tool must not auto-share results even when the policy is auto_run_everywhere")
}

func TestSharedForTurns_NamespacedToolUsesBarePolicy(t *testing.T) {
	const origin = "https://mcp.example.com/mcp"

	c := policyTestConversations(mapPolicyChecker{
		origin: {
			"example_tool": {policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true},
		},
	})
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: "example__example_tool", ServerOrigin: origin}})

	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{{Name: "example__example_tool", ServerOrigin: origin}},
	}}

	assert.True(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
}

func TestSharedForTurns_AllowsMetaTools(t *testing.T) {
	const origin = "https://mcp.example.com/mcp"
	const toolName = "example_tool"

	c := policyTestConversations(mapPolicyChecker{
		origin: {
			toolName: {policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true},
		},
	})
	store := llm.NewToolStore()
	store.AddTools(mcp.NewMetaTools(nil))
	store.AddTools([]llm.Tool{{Name: toolName, ServerOrigin: origin}})
	llmCtx := &llm.Context{Tools: store}

	turns := []toolrunner.ToolTurn{
		{AssistantToolCalls: []llm.ToolCall{{Name: mcp.SearchToolsName}}},
		{AssistantToolCalls: []llm.ToolCall{{Name: toolName, ServerOrigin: origin}}},
	}

	assert.True(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
}

func TestSharedForTurns_UnknownToolReturnsFalse(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{{Name: "unknown_tool", ServerOrigin: "https://mcp.example.com"}},
	}}

	assert.False(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
	assert.Zero(t, checker.calls)
}

func TestSharedForTurnsMetaOnlyBypassesPolicy(t *testing.T) {
	c := policyTestConversations(nil)
	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{
			{Name: mcp.SearchToolsName},
			{Name: mcp.LoadToolName},
		},
	}}

	assert.True(t, testTurnPolicy(c, false).sharedForTurns(turns, nil))
}

// TestSharedForTurns_AutoExecuteBuiltIn pins that a round made up
// only of auto-execute built-ins (e.g. CreateFile) is written shared=true even
// with no policy checker, while an MCP tool carrying the flag still requires
// an auto_run_everywhere policy and so fails closed here.
func TestSharedForTurns_AutoExecuteBuiltIn(t *testing.T) {
	const mcpOrigin = "https://mcp.example.com/mcp"

	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "built-in only round is shared", origin: "", want: true},
		{name: "MCP tool with flag still requires policy", origin: mcpOrigin, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := policyTestConversations(nil)
			llmCtx := &llm.Context{Tools: llm.NewToolStore()}
			llmCtx.Tools.AddTools([]llm.Tool{{Name: "CreateFile", ServerOrigin: tc.origin, AutoExecute: true}})
			turns := []toolrunner.ToolTurn{{
				AssistantToolCalls: []llm.ToolCall{{Name: "CreateFile", ServerOrigin: tc.origin}},
			}}

			assert.Equal(t, tc.want, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
		})
	}
}

func TestSharedForTurnsMixedMetaAndAutoRunBusinessTool(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	const origin = "https://mcp.atlassian.com"
	const runtimeToolName = "jira__get_issue"
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: runtimeToolName, ServerOrigin: origin}})
	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{
			{Name: mcp.SearchToolsName},
			{Name: runtimeToolName},
			{Name: mcp.LoadToolName},
		},
	}}

	assert.True(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
	assert.Equal(t, 1, checker.calls)
	assert.Equal(t, "get_issue", checker.lastToolName)
}

func TestSharedForTurnsMixedMetaAndNonAutoBusinessTool(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAsk, enabled: true}
	c := policyTestConversations(checker)
	const origin = "https://mcp.atlassian.com"
	const runtimeToolName = "jira__get_issue"
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: runtimeToolName, ServerOrigin: origin}})
	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{
			{Name: mcp.SearchToolsName},
			{Name: runtimeToolName},
		},
	}}

	assert.False(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
	assert.Equal(t, 1, checker.calls)
	assert.Equal(t, "get_issue", checker.lastToolName)
}

func TestSharedForTurnsDenormalizesNamespacedTool(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	const origin = "https://mcp.atlassian.com"
	const runtimeToolName = "jira__get_issue"
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{{Name: runtimeToolName, ServerOrigin: origin}})
	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{{Name: runtimeToolName}},
	}}

	assert.True(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
	assert.Equal(t, 1, checker.calls)
	assert.Equal(t, "get_issue", checker.lastToolName)
}

func TestSharedForTurnsFailsClosedOnAmbiguousBareName(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{
		{Name: "jira__get_issue", ServerOrigin: "https://jira.example.com"},
		{Name: "github__get_issue", ServerOrigin: "https://github.example.com"},
	})
	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{{Name: "get_issue"}},
	}}

	assert.False(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
	assert.Zero(t, checker.calls)
}

func TestSharedForTurnsUsesServerOriginToDisambiguateBareName(t *testing.T) {
	checker := &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true}
	c := policyTestConversations(checker)
	const origin = "https://github.example.com"
	llmCtx := &llm.Context{Tools: llm.NewToolStore()}
	llmCtx.Tools.AddTools([]llm.Tool{
		{Name: "jira__get_issue", ServerOrigin: "https://jira.example.com"},
		{Name: "github__get_issue", ServerOrigin: origin},
	})
	turns := []toolrunner.ToolTurn{{
		AssistantToolCalls: []llm.ToolCall{{Name: "get_issue", ServerOrigin: origin}},
	}}

	assert.True(t, testTurnPolicy(c, false).sharedForTurns(turns, llmCtx))
	assert.Equal(t, 1, checker.calls)
	assert.Equal(t, origin, checker.lastOrigin)
	assert.Equal(t, "get_issue", checker.lastToolName)
}

// TestAutoExecuteRequiresRemoteMCPLicense pins that a tool from a remote or
// plugin MCP server never runs without approval below Enterprise, whatever
// its policy, while built-in and embedded Mattermost tools still follow
// policy.
func TestAutoExecuteRequiresRemoteMCPLicense(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		level  enterprise.Level
		want   bool
	}{
		{name: "remote MCP below Enterprise", origin: "https://mcp.example.com", level: enterprise.LevelProfessional, want: false},
		{name: "plugin MCP below Enterprise", origin: "plugin://com.example.demo", level: enterprise.LevelProfessional, want: false},
		{name: "embedded MCP below Enterprise", origin: mcp.EmbeddedClientKey, level: enterprise.LevelUnlicensed, want: true},
		{name: "remote MCP at Enterprise", origin: "https://mcp.example.com", level: enterprise.LevelEnterprise, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Conversations{
				toolPolicyChecker: &countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true},
				licenseChecker:    enterprisetest.CheckerAt(tc.level),
			}
			llmCtx := &llm.Context{Tools: llm.NewToolStore()}
			llmCtx.Tools.AddTools([]llm.Tool{{Name: "get_issue", ServerOrigin: tc.origin}})

			for _, isDM := range []bool{true, false} {
				got := testTurnPolicy(c, isDM).autoExecute(llmCtx)(llm.ToolCall{Name: "get_issue", ServerOrigin: tc.origin})
				assert.Equal(t, tc.want, got, "isDM=%v", isDM)
			}
		})
	}
}

// TestAutoExecuteNeverRunsWhenToolsAreDisabled pins that a turn without tools
// runs nothing, even tools that never need approval.
func TestAutoExecuteNeverRunsWhenToolsAreDisabled(t *testing.T) {
	c := policyTestConversations(&countingPolicyChecker{policy: mcp.ToolPolicyAutoRunEverywhere, enabled: true})
	store := llm.NewToolStore()
	store.AddTools(mcp.NewMetaTools(nil))
	store.AddTools([]llm.Tool{{Name: "CreateFile", AutoExecute: true}, {Name: "get_issue", ServerOrigin: "https://mcp.example.com"}})
	llmCtx := &llm.Context{Tools: store}
	p := &turnPolicy{c: c}

	for _, call := range []llm.ToolCall{{Name: mcp.LoadToolName}, {Name: "CreateFile"}, {Name: "get_issue", ServerOrigin: "https://mcp.example.com"}} {
		assert.False(t, p.autoExecute(llmCtx)(call), call.Name)
	}
}
