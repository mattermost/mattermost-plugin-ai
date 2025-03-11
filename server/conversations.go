// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost-plugin-ai/server/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

const RespondingToProp = "responding_to"

func (p *Plugin) processUserRequestToBot(bot *Bot, postingUser *model.User, channel *model.Channel, post *model.Post) (*llm.TextStreamResult, error) {
	context := p.BuildLLMContextUserRequest(
		bot,
		postingUser,
		channel,
		p.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
	)

	prompt, err := p.prompts.Format(llm.PromptDirectMessageQuestionSystem, context)
	if err != nil {
		return nil, fmt.Errorf("failed to format prompt: %w", err)
	}

	posts := []llm.Post{
		{
			Role:    llm.PostRoleSystem,
			Message: prompt,
		},
	}

	// If we are continuing an existing conversation
	if post.RootId != "" {
		previousConversation, errThread := p.getThreadAndMeta(post.Id)
		if errThread != nil {
			return nil, fmt.Errorf("failed to get previous conversation: %w", errThread)
		}
		previousConversation.cutoffBeforePostID(post.Id)

		posts = append(posts, p.ThreadToLLMPosts(bot, previousConversation.Posts)...)
	}

	posts = append(posts, llm.Post{
		Role:    llm.PostRoleUser,
		Message: post.Message,
	})

	completionRequest := llm.CompletionRequest{
		Posts:   posts,
		Context: context,
	}
	result, err := p.getLLM(bot.cfg).ChatCompletion(completionRequest)
	if err != nil {
		return nil, err
	}

	go func() {
		request := "Write a short title for the following request. Include only the title and nothing else, no quotations. Request:\n" + post.Message
		if err := p.generateTitle(bot, request, post.Id, context); err != nil {
			p.API.LogError("Failed to generate title", "error", err.Error())
			return
		}
	}()

	return result, nil
}

func (p *Plugin) generateTitle(bot *Bot, request string, postID string, context *llm.Context) error {
	titleRequest := llm.CompletionRequest{
		Posts:   []llm.Post{{Role: llm.PostRoleUser, Message: request}},
		Context: context,
	}

	conversationTitle, err := p.getLLM(bot.cfg).ChatCompletionNoStream(titleRequest, llm.WithMaxGeneratedTokens(25))
	if err != nil {
		return fmt.Errorf("failed to get title: %w", err)
	}

	conversationTitle = strings.Trim(conversationTitle, "\n \"'")

	if err := p.saveTitle(postID, conversationTitle); err != nil {
		return fmt.Errorf("failed to save title: %w", err)
	}

	return nil
}

/*func (p *Plugin) continueConversation(bot *Bot, threadData *ThreadData, context llm.ConversationContext) (*llm.TextStreamResult, error) {
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

	context.PromptParameters = map[string]any{"Thread": originalThread}
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
}*/
