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

// processUserRequestWithContext is an internal helper that uses an existing context to process a message
func (p *Plugin) processUserRequestWithContext(bot *Bot, postingUser *model.User, channel *model.Channel, post *model.Post, context *llm.Context) (*llm.TextStreamResult, error) {
	var posts []llm.Post
	if post.RootId == "" {
		// A new conversation
		prompt, err := p.prompts.Format(llm.PromptDirectMessageQuestionSystem, context)
		if err != nil {
			return nil, fmt.Errorf("failed to format prompt: %w", err)
		}
		posts = []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: prompt,
			},
		}
	} else {
		// Continuing an existing conversation
		previousConversation, errThread := p.getThreadAndMeta(post.Id)
		if errThread != nil {
			return nil, fmt.Errorf("failed to get previous conversation: %w", errThread)
		}
		previousConversation.cutoffBeforePostID(post.Id)

		var err error
		posts, err = p.existingConversationToLLMPosts(bot, previousConversation, context)
		if err != nil {
			return nil, fmt.Errorf("failed to convert existing conversation to LLM posts: %w", err)
		}
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

func (p *Plugin) processUserRequestToBot(bot *Bot, postingUser *model.User, channel *model.Channel, post *model.Post) (*llm.TextStreamResult, error) {
	// Create a context with default tools
	context := p.BuildLLMContextUserRequest(
		bot,
		postingUser,
		channel,
		p.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
	)

	return p.processUserRequestWithContext(bot, postingUser, channel, post, context)
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

func (p *Plugin) existingConversationToLLMPosts(bot *Bot, conversation *ThreadData, context *llm.Context) ([]llm.Post, error) {
	// Handle thread summarization requests
	originalThreadID, ok := conversation.Posts[0].GetProp(ThreadIDProp).(string)
	if ok && originalThreadID != "" && conversation.Posts[0].UserId == bot.mmBot.UserId {
		threadPost, err := p.pluginAPI.Post.GetPost(originalThreadID)
		if err != nil {
			return nil, err
		}
		threadChannel, err := p.pluginAPI.Channel.Get(threadPost.ChannelId)
		if err != nil {
			return nil, err
		}

		if !p.pluginAPI.User.HasPermissionToChannel(context.RequestingUser.Id, threadChannel.Id, model.PermissionReadChannel) ||
			p.checkUsageRestrictions(context.RequestingUser.Id, bot, threadChannel) != nil {
			T := i18nLocalizerFunc(p.i18n, context.RequestingUser.Locale)
			responsePost := &model.Post{
				ChannelId: context.Channel.Id,
				RootId:    originalThreadID,
				Message:   T("copilot.no_longer_access_error", "Sorry, you no longer have access to the original thread."),
			}
			if err = p.botCreateNonResponsePost(bot.mmBot.UserId, context.RequestingUser.Id, responsePost); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("user no longer has access to original thread")
		}

		analysisType, ok := conversation.Posts[0].GetProp(AnalysisTypeProp).(string)
		if !ok {
			return nil, fmt.Errorf("missing analysis type")
		}

		posts, err := p.getAnalyzeThreadPosts(originalThreadID, context, analysisType)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p.ThreadToLLMPosts(bot, conversation.Posts)...)
		return posts, nil
	}

	// Plain DM conversation
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
	posts = append(posts, p.ThreadToLLMPosts(bot, conversation.Posts)...)

	return posts, nil
}
