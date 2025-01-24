// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

const RespondingToProp = "responding_to"

func (p *Plugin) processUserRequestToBot(bot *Bot, context llm.ConversationContext) error {
	if context.Post.RootId == "" {
		return p.newConversation(bot, context)
	}

	threadData, err := p.getThreadAndMeta(context.Post.RootId)
	if err != nil {
		return err
	}

	// Cutoff the thread at the post we are responding to avoid races.
	threadData.cutoffAtPostID(context.Post.Id)

	result, err := p.continueConversation(bot, threadData, context)
	if err != nil {
		return err
	}

	responsePost := &model.Post{
		ChannelId: context.Channel.Id,
		RootId:    context.Post.RootId,
	}
	responsePost.AddProp(RespondingToProp, context.Post.Id)
	if err := p.streamResultToNewPost(bot.mmBot.UserId, context.RequestingUser.Id, result, responsePost); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) newConversation(bot *Bot, context llm.ConversationContext) error {
	conversation, err := p.prompts.ChatCompletion(llm.PromptDirectMessageQuestion, context, p.getDefaultToolsStore(bot, context.IsDMWithBot()))
	if err != nil {
		return err
	}
	conversation.AddPost(p.PostToAIPost(bot, context.Post))

	result, err := p.getLLM(bot.cfg).ChatCompletion(conversation)
	if err != nil {
		return err
	}

	responsePost := &model.Post{
		ChannelId: context.Channel.Id,
		RootId:    context.Post.Id,
	}
	if err := p.streamResultToNewPost(bot.mmBot.UserId, context.RequestingUser.Id, result, responsePost); err != nil {
		return err
	}

	go func() {
		request := "Write a short title for the following request. Include only the title and nothing else, no quotations. Request:\n" + context.Post.Message
		if err := p.generateTitle(bot, request, context); err != nil {
			p.API.LogError("Failed to generate title", "error", err.Error())
			return
		}
	}()

	return nil
}

func (p *Plugin) generateTitle(bot *Bot, request string, context llm.ConversationContext) error {
	titleRequest := llm.BotConversation{
		Posts:   []llm.Post{{Role: llm.PostRoleUser, Message: request}},
		Context: context,
	}
	conversationTitle, err := p.getLLM(bot.cfg).ChatCompletionNoStream(titleRequest, llm.WithMaxGeneratedTokens(25))
	if err != nil {
		return fmt.Errorf("failed to get title: %w", err)
	}

	conversationTitle = strings.Trim(conversationTitle, "\n \"'")

	if err := p.saveTitle(context.Post.Id, conversationTitle); err != nil {
		return fmt.Errorf("failed to save title: %w", err)
	}

	return nil
}

func (p *Plugin) continueConversation(bot *Bot, threadData *ThreadData, context llm.ConversationContext) (*llm.TextStreamResult, error) {
	// Special handing for threads started by the bot in response to a summarization request.
	var result *llm.TextStreamResult
	originalThreadID, ok := threadData.Posts[0].GetProp(ThreadIDProp).(string)
	if ok && originalThreadID != "" && threadData.Posts[0].UserId == bot.mmBot.UserId {
		threadPost, err := p.pluginAPI.Post.GetPost(originalThreadID)
		if err != nil {
			return nil, err
		}
		threadChannel, err := p.pluginAPI.Channel.Get(threadPost.ChannelId)
		if err != nil {
			return nil, err
		}

		if !p.pluginAPI.User.HasPermissionToChannel(context.Post.UserId, threadChannel.Id, model.PermissionReadChannel) ||
			p.checkUsageRestrictions(context.Post.UserId, bot, threadChannel) != nil {
			T := i18nLocalizerFunc(p.i18n, context.RequestingUser.Locale)
			responsePost := &model.Post{
				ChannelId: context.Channel.Id,
				RootId:    context.Post.RootId,
				Message:   T("copilot.no_longer_access_error", "Sorry, you no longer have access to the original thread."),
			}
			if err = p.botCreatePost(bot.mmBot.UserId, context.RequestingUser.Id, responsePost); err != nil {
				return nil, err
			}
			return nil, errors.New("user no longer has access to original thread")
		}

		result, err = p.continueThreadConversation(bot, threadData, originalThreadID, context)
		if err != nil {
			return nil, err
		}
	} else {
		prompt, err := p.prompts.ChatCompletion(llm.PromptDirectMessageQuestion, context, p.getDefaultToolsStore(bot, context.IsDMWithBot()))
		if err != nil {
			return nil, err
		}
		prompt.AppendConversation(p.ThreadToBotConversation(bot, threadData.Posts))

		result, err = p.getLLM(bot.cfg).ChatCompletion(prompt)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (p *Plugin) continueThreadConversation(bot *Bot, questionThreadData *ThreadData, originalThreadID string, context llm.ConversationContext) (*llm.TextStreamResult, error) {
	originalThreadData, err := p.getThreadAndMeta(originalThreadID)
	if err != nil {
		return nil, err
	}
	originalThread := formatThread(originalThreadData)

	context.PromptParameters = map[string]string{"Thread": originalThread}
	prompt, err := p.prompts.ChatCompletion(llm.PromptSummarizeThread, context, p.getDefaultToolsStore(bot, context.IsDMWithBot()))
	if err != nil {
		return nil, err
	}
	prompt.AppendConversation(p.ThreadToBotConversation(bot, questionThreadData.Posts))

	result, err := p.getLLM(bot.cfg).ChatCompletion(prompt)
	if err != nil {
		return nil, err
	}

	return result, nil
}
