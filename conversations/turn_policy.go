// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/llmcontext"
	"github.com/mattermost/mattermost-plugin-agents/v2/mcp"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmapi"
	"github.com/mattermost/mattermost-plugin-agents/v2/store"
	"github.com/mattermost/mattermost-plugin-agents/v2/toolrunner"
	"github.com/mattermost/mattermost/server/public/model"
)

// ErrThreadSourceUnavailable is returned when a thread analysis conversation
// is continued by a user who can no longer read the analyzed thread, or when
// that thread cannot be found.
var ErrThreadSourceUnavailable = errors.New("the analyzed thread is no longer available to the requester")

// turnRequest identifies a turn that is about to run: a new turn started by
// trigger, or a resume (regenerate, tool decision) of the run that produced
// post.
type turnRequest struct {
	bot     *bots.Bot
	user    *model.User
	channel *model.Channel
	// conv is the conversation the turn continues. New DM thread replies
	// leave it nil; the resolver looks it up.
	conv    *store.Conversation
	trigger *model.Post
	post    *model.Post
}

// turnPolicy is the resolved behavior of one turn. Every entry point that
// starts or resumes a turn gets it from resolveTurnPolicy, so gates, tool
// availability, and the auto-execute decision cannot drift between them.
type turnPolicy struct {
	c *Conversations
	turnRequest

	isDM         bool
	toolsEnabled bool
	// interactive means the requester is present to answer user-interaction
	// tools and connection requests.
	interactive bool
	// autoEverywhereOnly limits the catalog to tools that may run unattended
	// in a channel (auto_run_everywhere), for runs nobody is present to
	// approve.
	autoEverywhereOnly bool
	convOpts           []llm.ContextOption
	maxRounds          int
}

// resolveTurnPolicy checks that req may run and derives how it runs. Gates
// fail closed: the requester's access to the agent, the channel license for
// every non-DM turn, and, for thread analysis conversations, the requester's
// access to the analyzed thread.
func (c *Conversations) resolveTurnPolicy(ctx context.Context, req turnRequest) (*turnPolicy, error) {
	if req.bot == nil || req.user == nil || req.channel == nil {
		return nil, errors.New("turn is missing its agent, requester, or channel")
	}
	isDM := mmapi.IsDMWith(req.bot.GetMMBot().UserId, req.channel)

	if isDM {
		if err := c.bots.CheckUsageRestrictionsForUser(ctx, req.bot, req.user.Id); err != nil {
			return nil, err
		}
	} else {
		if err := c.bots.CheckUsageRestrictions(ctx, req.user.Id, req.bot, req.channel); err != nil {
			return nil, err
		}
		if err := c.licenseChecker.Check(enterprise.CapMultiplayerChannels); err != nil {
			return nil, err
		}
	}

	if req.conv == nil && isDM && req.trigger != nil && req.trigger.RootId != "" {
		if c.convService == nil {
			return nil, errors.New("conversation service not configured")
		}
		conv, err := c.convService.GetConversationByThread(req.trigger.RootId, req.bot.GetMMBot().UserId, req.user.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to look up thread conversation: %w", err)
		}
		req.conv = conv
	}

	if req.conv != nil && req.conv.Operation == llm.OperationThreadAnalysis {
		if err := c.checkThreadAnalysisConversationSource(req.user.Id, req.conv, req.post); err != nil {
			return nil, err
		}
	}

	p := &turnPolicy{
		c:           c,
		turnRequest: req,
		isDM:        isDM,
		convOpts:    c.conversationToolOptions(req.bot, req.conv),
		maxRounds:   req.bot.GetConfig().EffectiveMaxToolTurns(),
	}
	switch {
	case isDM:
		p.toolsEnabled = true
		p.interactive = true
	case req.trigger != nil:
		p.setChannelRunTools(req.trigger, req.user)
	case !c.channelMentionToolCallingEnabled():
		// Nothing can run, so the initiator does not matter; the requester
		// clicked a control and is present.
		p.interactive = true
	default:
		initiator, initiatingUser := c.runInitiator(req.conv, req.post)
		p.setChannelRunTools(initiator, initiatingUser)
	}
	return p, nil
}

// setChannelRunTools derives tool availability for a channel run started by
// initiator, posted by initiatingUser. An unknown initiator is treated as
// unattended.
func (p *turnPolicy) setChannelRunTools(initiator *model.Post, initiatingUser *model.User) {
	configEnabled := p.c.channelMentionToolCallingEnabled()
	switch {
	case initiator == nil || initiatingUser == nil || isBotActivateAI(initiator, initiatingUser):
		// Auto-approval needs a policy checker; without one an unattended
		// run would strand pending approvals. The catalog stays limited
		// either way, and the filter keeps no business tools without one.
		p.toolsEnabled = configEnabled && p.c.toolPolicyChecker != nil
		p.autoEverywhereOnly = true
	case isAutomatedInvoker(initiator, initiatingUser):
	default:
		p.toolsEnabled = configEnabled
		p.interactive = true
	}
}

// runInitiator returns the post that started the run which produced post,
// and its author. Either is nil when it cannot be found.
func (c *Conversations) runInitiator(conv *store.Conversation, post *model.Post) (*model.Post, *model.User) {
	if conv == nil || post == nil || c.convService == nil {
		return nil, nil
	}
	turn, err := c.convService.GetInitiatingUserTurn(conv.ID, post.Id)
	if err != nil || turn == nil || turn.PostID == nil {
		return nil, nil
	}
	initiator, err := c.mmClient.GetPost(*turn.PostID)
	if err != nil || initiator == nil {
		return nil, nil
	}
	initiatingUser, err := c.mmClient.GetUser(initiator.UserId)
	if err != nil || initiatingUser == nil {
		return nil, nil
	}
	return initiator, initiatingUser
}

// checkThreadAnalysisConversationSource re-checks that userID can read the
// thread a thread analysis conversation summarized. post is the agent post
// being resumed, if any; it saves a lookup when it is the analysis post.
func (c *Conversations) checkThreadAnalysisConversationSource(userID string, conv *store.Conversation, post *model.Post) error {
	if conv.RootPostID == nil || *conv.RootPostID == "" {
		return fmt.Errorf("thread analysis conversation has no analysis post: %w", ErrThreadSourceUnavailable)
	}
	analysisPost := post
	if analysisPost == nil || analysisPost.Id != *conv.RootPostID {
		var err error
		analysisPost, err = c.mmClient.GetPost(*conv.RootPostID)
		if err != nil {
			return fmt.Errorf("failed to get analysis post: %w: %w", ErrThreadSourceUnavailable, err)
		}
	}
	_, err := c.threadAnalysisSource(userID, analysisPost)
	return err
}

// threadAnalysisSource returns the thread analysisPost summarized, provided
// userID can still read it.
func (c *Conversations) threadAnalysisSource(userID string, analysisPost *model.Post) (*model.Post, error) {
	threadID, _ := analysisPost.GetProp(ThreadIDProp).(string)
	if threadID == "" {
		return nil, fmt.Errorf("analysis post has no analyzed thread: %w", ErrThreadSourceUnavailable)
	}
	threadPost, err := c.mmClient.GetPost(threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get analyzed thread: %w: %w", ErrThreadSourceUnavailable, err)
	}
	if !c.mmClient.HasPermissionToChannel(userID, threadPost.ChannelId, model.PermissionReadChannel) {
		return nil, ErrThreadSourceUnavailable
	}
	return threadPost, nil
}

// buildContext builds the turn's LLM context with the tools the policy
// allows. extraOpts are applied after the policy's own options.
func (p *turnPolicy) buildContext(ctx context.Context, prefsLogMessage string, extraOpts ...llm.ContextOption) *llm.Context {
	builder := p.c.contextBuilder
	opts := []llm.ContextOption{builder.WithLLMContextResponseFiles()}
	if p.interactive {
		opts = append(opts, builder.WithLLMContextInteractive())
	}
	if p.autoEverywhereOnly {
		opts = append(opts, builder.WithLLMContextMCPToolFilter(func(tool llm.Tool) bool {
			return botChannelAutoEverywhereKeepTool(p.c.toolPolicyChecker, tool)
		}))
	}
	opts = append(opts, p.convOpts...)
	opts = append(opts, extraOpts...)

	llmContext := p.c.buildConversationContextWithTools(ctx, p.bot, p.user, p.channel, prefsLogMessage, opts...)
	applyToolAvailability(llmContext, p.isDM, p.toolsEnabled)
	if p.autoEverywhereOnly {
		p.c.applyBotChannelAutoEverywhereToolFilter(llmContext)
	}
	return llmContext
}

// connectionCheck returns the context an accepted connection request is
// judged against, and the filter the run applies to the server's tools. A
// restricted run's own context cannot tell a server that is still not
// connected from one whose tools the run may not use, so it is judged
// against the requester's full catalog. That context only answers the
// request; it never executes tools.
func (p *turnPolicy) connectionCheck(ctx context.Context, runCtx *llm.Context) (*llm.Context, func(llm.Tool) bool) {
	if p.interactive && !p.autoEverywhereOnly {
		return runCtx, nil
	}
	full := *p
	full.toolsEnabled, full.interactive, full.autoEverywhereOnly = true, true, false
	fullCtx := full.buildContext(ctx, "Failed to load user tool preferences for connection request")
	if p.autoEverywhereOnly {
		return fullCtx, func(tool llm.Tool) bool {
			return botChannelAutoEverywhereKeepTool(p.c.toolPolicyChecker, tool)
		}
	}
	return fullCtx, func(llm.Tool) bool { return false }
}

// autoExecute decides, per tool call, whether the call runs without the
// requester's approval.
func (p *turnPolicy) autoExecute(llmCtx *llm.Context) func(llm.ToolCall) bool {
	return func(tc llm.ToolCall) bool {
		if !p.toolsEnabled {
			return false
		}
		if isMCPMetaToolCall(tc, llmCtx) {
			return true
		}
		var lookup llm.ToolLookup
		var found bool
		if llmCtx != nil {
			lookup, found = llmCtx.Tools.LookupTool(tc.Name, tc.ServerOrigin)
		}
		if !found {
			return false
		}
		// Interaction tools are answered by the user; auto-executing one
		// would bypass the question entirely.
		if lookup.Tool.UserInteraction != "" {
			return false
		}
		if p.remoteMCPDecisionAllowed(lookup.ServerOrigin) != nil {
			return false
		}
		// Only honored for built-ins: an MCP tool carrying the flag must
		// still go through policy.
		if isAutoExecuteBuiltIn(lookup.Tool) {
			return true
		}
		if p.c.toolPolicyChecker == nil {
			return false
		}
		policy, enabled := p.c.toolPolicyChecker.GetToolPolicy(lookup.ServerOrigin, lookup.BareName)
		if !enabled {
			return false
		}
		// The legacy auto_run policy is DM-only so a channel-visible reply
		// cannot reveal unshared tool output without an explicit Share.
		if p.isDM {
			return mcp.IsToolPolicyAutoRunInDM(policy)
		}
		return mcp.IsToolPolicyAutoRunEverywhere(policy)
	}
}

// remoteMCPDecisionAllowed returns ErrRemoteMCPNotLicensed when executing, or
// sharing the output of, a tool from serverOrigin needs the remote MCP
// license and the server does not have it. A nil license checker fails
// closed.
func (p *turnPolicy) remoteMCPDecisionAllowed(serverOrigin string) error {
	if mcp.IsRemoteServerOrigin(serverOrigin) && !p.c.licenseChecker.Allows(enterprise.CapRemoteMCP) {
		return ErrRemoteMCPNotLicensed
	}
	return nil
}

// sharedForTurns reports whether tool rounds the turn ran automatically may
// be written as shared, skipping the result-approval step. DM results are
// always shared; in a channel every call must be one that needs no approval
// anywhere.
func (p *turnPolicy) sharedForTurns(turns []toolrunner.ToolTurn, llmCtx *llm.Context) bool {
	if p.isDM {
		return true
	}
	sawToolCall := false
	for _, turn := range turns {
		for _, tc := range turn.AssistantToolCalls {
			sawToolCall = true
			if isMCPMetaToolCall(tc, llmCtx) {
				continue
			}
			var lookup llm.ToolLookup
			var found bool
			if llmCtx != nil {
				lookup, found = llmCtx.Tools.LookupTool(tc.Name, tc.ServerOrigin)
			}
			if !found {
				return false
			}
			if isAutoExecuteBuiltIn(lookup.Tool) {
				continue
			}
			if p.c.toolPolicyChecker == nil {
				return false
			}
			policy, enabled := p.c.toolPolicyChecker.GetToolPolicy(lookup.ServerOrigin, lookup.BareName)
			if !enabled || !mcp.IsToolPolicyAutoRunEverywhere(policy) {
				return false
			}
		}
	}
	return sawToolCall
}

// runToolLoop runs the ToolRunner over req with the turn's round cap and
// auto-execute decision, persisting each intermediate tool round to convID as
// it completes. writeFailMsg (plus writeFailArgs) is logged when persisting a
// round fails.
func (p *turnPolicy) runToolLoop(ctx context.Context, llmCtx *llm.Context, convID string, req llm.CompletionRequest, writeFailMsg string, writeFailArgs ...any) (*toolrunner.ToolRunResult, error) {
	opts := p.c.withProviderWebSearchLicense(p.c.toolsDisabledLLMOptions(p.bot, !p.toolsEnabled))
	runner := toolrunner.New(p.bot.LLM(), toolrunner.WithMaxRounds(p.maxRounds), toolrunner.WithToolErrorResult(llmcontext.MCPToolErrorResult))
	return runner.Run(ctx, req, p.autoExecute(llmCtx), func(turns []toolrunner.ToolTurn) {
		if writeErr := p.c.convService.WriteToolTurns(convID, turns, p.sharedForTurns(turns, llmCtx)); writeErr != nil {
			p.c.mmClient.LogError(writeFailMsg, append([]any{"error", writeErr}, writeFailArgs...)...)
		}
	}, opts...)
}
