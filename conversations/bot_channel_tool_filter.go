// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mcp"
)

// applyBotChannelAutoEverywhereToolFilter keeps only MCP tools whose policy is
// auto_run_everywhere and enabled. Built-in tools (empty ServerOrigin) are removed
// except for internal MCP meta-tools and auto-execute built-ins.
// Removed tools are recorded in DisabledToolsInfo for the model.
// When no policy checker is configured, fail closed for business tools so replayed
// posts cannot expose MCP tools without policy validation.
func (c *Conversations) applyBotChannelAutoEverywhereToolFilter(llmContext *llm.Context) {
	if llmContext == nil || llmContext.Tools == nil {
		return
	}
	if c.toolPolicyChecker == nil {
		// Meta-tools and auto-execute built-ins never require policy
		// approval, so they survive even without a checker.
		keep := func(tool llm.Tool) bool {
			return mcp.IsMCPMetaTool(tool.Name) || isAutoExecuteBuiltIn(tool)
		}
		removed := make([]llm.ToolInfo, 0)
		for _, tool := range llmContext.Tools.GetTools() {
			if keep(tool) {
				continue
			}
			removed = append(removed, llm.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
			})
		}
		llmContext.Tools.KeepToolsIf(keep)
		if len(removed) > 0 {
			// Replace any prior DisabledToolsInfo from applyToolAvailability with the tools removed here.
			llmContext.DisabledToolsInfo = removed
		}
		return
	}

	removed := make([]llm.ToolInfo, 0)
	for _, t := range llmContext.Tools.GetTools() {
		if botChannelAutoEverywhereKeepTool(c.toolPolicyChecker, t) {
			continue
		}
		removed = append(removed, llm.ToolInfo{
			Name:        t.Name,
			Description: t.Description,
		})
	}

	llmContext.Tools.KeepToolsIf(func(tool llm.Tool) bool {
		return botChannelAutoEverywhereKeepTool(c.toolPolicyChecker, tool)
	})

	if len(removed) > 0 {
		llmContext.DisabledToolsInfo = append(llmContext.DisabledToolsInfo, removed...)
	}
}

// applyToolAvailability decides whether tools should be disabled for a context based on
// whether the conversation is a DM or a channel mention with tools allowed. It returns
// true when tools are disabled. When disabled, it populates DisabledToolsInfo so the
// model can still reference the tool descriptions.
func applyToolAvailability(context *llm.Context, isDM bool, allowToolsInChannel bool) bool {
	toolsDisabled := !isDM && !allowToolsInChannel
	if context != nil {
		if toolsDisabled && context.Tools != nil {
			context.DisabledToolsInfo = context.Tools.GetToolsInfo()
		} else {
			context.DisabledToolsInfo = nil
		}
	}
	return toolsDisabled
}

// toolsDisabledLLMOptions returns the language-model options for a run whose
// tools are disabled: tools off, plus native web search when both the config
// and the bot allow it in channels. Nil when tools are enabled.
func (c *Conversations) toolsDisabledLLMOptions(bot *bots.Bot, toolsDisabled bool) []llm.LanguageModelOption {
	if !toolsDisabled {
		return nil
	}
	opts := []llm.LanguageModelOption{llm.WithToolsDisabled()}
	if c.configProvider != nil && c.configProvider.AllowNativeWebSearchInChannels() && bot.HasNativeWebSearchEnabled() && c.licenseChecker.Allows(enterprise.CapProviderWebSearch) {
		opts = append(opts, llm.WithNativeWebSearchAllowed())
	}
	return opts
}

func botChannelAutoEverywhereKeepTool(checker mcp.ToolPolicyChecker, tool llm.Tool) bool {
	if mcp.IsMCPMetaTool(tool.Name) {
		return true
	}
	// Auto-execute built-ins (e.g. CreateFile) run without approval like the
	// meta-tools, so they stay available in bot-channel follow-ups.
	if isAutoExecuteBuiltIn(tool) {
		return true
	}
	if checker == nil {
		return false
	}
	if tool.ServerOrigin == "" {
		return false
	}
	policy, enabled := checker.GetToolPolicy(tool.ServerOrigin, llm.BareMCPToolName(tool.Name))
	return mcp.IsToolPolicyAutoRunEverywhere(policy) && enabled
}
