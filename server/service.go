package main

import (
	"fmt"
	"strings"

	"errors"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	WhisperAPILimit    = 25 * 1000 * 1000 // 25 MB
	ContextTokenMargin = 1000
	RespondingToProp   = "responding_to"
)

func (p *Plugin) processUserRequestToBot(bot *Bot, context ai.ConversationContext) error {
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

func (p *Plugin) newConversation(bot *Bot, context ai.ConversationContext) error {
	_, err := p.newConversationWithPost(bot, context)
	return err
}

func (p *Plugin) newConversationWithPost(bot *Bot, context ai.ConversationContext) (*model.Post, error) {
	conversation, err := p.prompts.ChatCompletion(ai.PromptDirectMessageQuestion, context)
	if err != nil {
		return nil, err
	}
	conversation.AddPost(p.PostToAIPost(bot, context.Post))

	result, err := p.getLLM(bot.cfg).ChatCompletion(conversation)
	if err != nil {
		return nil, err
	}

	responsePost := &model.Post{
		ChannelId: context.Channel.Id,
		RootId:    context.Post.Id,
	}
	if err := p.streamResultToNewPost(bot.mmBot.UserId, context.RequestingUser.Id, result, responsePost); err != nil {
		return nil, err
	}

	go func() {
		request := "Write a short title for the following request. Include only the title and nothing else, no quotations. Request:\n" + context.Post.Message
		if err := p.generateTitle(bot, request, context.Post.Id); err != nil {
			p.API.LogError("Failed to generate title", "error", err.Error())
			return
		}
	}()

	return responsePost, nil
}

func (p *Plugin) generateTitle(bot *Bot, request string, threadRootID string) error {
	titleRequest := ai.BotConversation{
		Posts: []ai.Post{{Role: ai.PostRoleUser, Message: request}},
	}
	conversationTitle, err := p.getLLM(bot.cfg).ChatCompletionNoStream(titleRequest, ai.WithMaxGeneratedTokens(25))
	if err != nil {
		return fmt.Errorf("failed to get title: %w", err)
	}

	conversationTitle = strings.Trim(conversationTitle, "\n \"'")

	if err := p.saveTitle(threadRootID, conversationTitle); err != nil {
		return fmt.Errorf("failed to save title: %w", err)
	}

	return nil
}

func (p *Plugin) continueConversation(bot *Bot, threadData *ThreadData, context ai.ConversationContext) (*ai.TextStreamResult, error) {
	// Special handing for threads started by the bot in response to a summarization request.
	var result *ai.TextStreamResult
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
			p.checkUsageRestrictions(context.Post.UserId, threadChannel) != nil {
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
		prompt, err := p.prompts.ChatCompletion(ai.PromptDirectMessageQuestion, context)
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

func (p *Plugin) continueThreadConversation(bot *Bot, questionThreadData *ThreadData, originalThreadID string, context ai.ConversationContext) (*ai.TextStreamResult, error) {
	originalThreadData, err := p.getThreadAndMeta(originalThreadID)
	if err != nil {
		return nil, err
	}
	originalThread := formatThread(originalThreadData)

	context.PromptParameters = map[string]string{"Thread": originalThread}
	prompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeThread, context)
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

const ThreadIDProp = "referenced_thread"

// DM the user with a standard message. Run the inferance
func (p *Plugin) summarizePost(bot *Bot, postIDToSummarize string, context ai.ConversationContext) (*ai.TextStreamResult, error) {
	threadData, err := p.getThreadAndMeta(postIDToSummarize)
	if err != nil {
		return nil, err
	}

	formattedThread := formatThread(threadData)

	context.PromptParameters = map[string]string{"Thread": formattedThread}
	prompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeThread, context)
	if err != nil {
		return nil, err
	}
	summaryStream, err := p.getLLM(bot.cfg).ChatCompletion(prompt)
	if err != nil {
		return nil, err
	}

	return summaryStream, nil
}

func (p *Plugin) summaryPostMessage(locale string, postIDToSummarize string, siteURL string) string {
	T := i18nLocalizerFunc(p.i18n, locale)
	return T("copilot.summarize_thread", "Sure, I will summarize this thread: %s/_redirect/pl/%s\n", siteURL, postIDToSummarize)
}

func (p *Plugin) makeSummaryPost(locale string, postIDToSummarize string) *model.Post {
	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	post := &model.Post{
		Message: p.summaryPostMessage(locale, postIDToSummarize, *siteURL),
	}
	post.AddProp(ThreadIDProp, postIDToSummarize)

	return post
}

func (p *Plugin) startNewSummaryThread(bot *Bot, postIDToSummarize string, context ai.ConversationContext) (*model.Post, error) {
	summaryStream, err := p.summarizePost(bot, postIDToSummarize, context)
	if err != nil {
		return nil, err
	}

	post := p.makeSummaryPost(context.RequestingUser.Locale, postIDToSummarize)
	if err := p.streamResultToNewDM(bot.mmBot.UserId, summaryStream, context.RequestingUser.Id, post); err != nil {
		return nil, err
	}

	p.saveTitleAsync(post.Id, "Thread Summary")

	return post, nil
}
