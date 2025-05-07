// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"errors"

	"github.com/mattermost/mattermost-plugin-ai/agents/channels"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

func (p *AgentsService) ChannelInterval(userID string, bot *Bot, channel *model.Channel, startTime, endTime int64, presetPrompt, prompt string) (map[string]string, error) {
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return nil, err
	}

	context := p.contextBuilder.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
		p.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
	)

	promptPreset := ""
	promptTitle := ""
	switch presetPrompt {
	case "summarize_unreads":
		promptPreset = llm.PromptSummarizeChannelSinceSystem
		promptTitle = "Summarize Unreads"
	case "summarize_range":
		promptPreset = llm.PromptSummarizeChannelRangeSystem
		promptTitle = "Summarize Channel"
	case "action_items":
		promptPreset = llm.PromptFindActionItemsSystem
		promptTitle = "Find Action Items"
	case "open_questions":
		promptPreset = llm.PromptFindOpenQuestionsSystem
		promptTitle = "Find Open Questions"
	default:
		return nil, errors.New("invalid preset prompt")
	}

	resultStream, err := channels.New(p.GetLLM(bot.cfg), p.prompts, p.mmClient).Interval(context, channel.Id, startTime, endTime, promptPreset)
	if err != nil {
		return nil, err
	}

	post := &model.Post{}
	post.AddProp(NoRegen, "true")
	// Here we don't have a specific post we're responding to, so pass empty string
	if err := p.streamResultToNewDM(bot.mmBot.UserId, resultStream, user.Id, post, ""); err != nil {
		return nil, err
	}

	p.saveTitleAsync(post.Id, promptTitle)

	return map[string]string{
		"postID":    post.Id,
		"channelId": post.ChannelId,
	}, nil
}
