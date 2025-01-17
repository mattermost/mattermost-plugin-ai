package main

import (
	"fmt"
	"strings"

	"errors"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	WhisperAPILimit    = 25 * 1000 * 1000 // 25 MB
	ContextTokenMargin = 1000
	RespondingToProp   = "responding_to"
)

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

const ThreadIDProp = "referenced_thread"
const AnalysisTypeProp = "prompt_type"

// DM the user with a standard message. Run the inferance
func (p *Plugin) analyzeThread(bot *Bot, postIDToAnalyze string, analysisType string, context llm.ConversationContext) (*llm.TextStreamResult, error) {
	threadData, err := p.getThreadAndMeta(postIDToAnalyze)
	if err != nil {
		return nil, err
	}

	formattedThread := formatThread(threadData)

	context.PromptParameters = map[string]string{"Thread": formattedThread}
	var promptType string
	switch analysisType {
	case "summarize_thread":
		promptType = llm.PromptSummarizeThread
	case "action_items":
		promptType = llm.PromptFindActionItems
	case "open_questions":
		promptType = llm.PromptFindOpenQuestions
	default:
		return nil, fmt.Errorf("invalid analysis type: %s", analysisType)
	}

	prompt, err := p.prompts.ChatCompletion(promptType, context, p.getDefaultToolsStore(bot, context.IsDMWithBot()))
	if err != nil {
		return nil, err
	}
	analysisStream, err := p.getLLM(bot.cfg).ChatCompletion(prompt)
	if err != nil {
		return nil, err
	}

	return analysisStream, nil
}

func (p *Plugin) makeAnalysisPost(locale string, postIDToAnalyze string, analysisType string) *model.Post {
	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	post := &model.Post{
		Message: p.analysisPostMessage(locale, postIDToAnalyze, analysisType, *siteURL),
	}
	post.AddProp(ThreadIDProp, postIDToAnalyze)
	post.AddProp(AnalysisTypeProp, analysisType)

	return post
}

func (p *Plugin) analysisPostMessage(locale string, postIDToAnalyze string, analysisType string, siteURL string) string {
	T := i18nLocalizerFunc(p.i18n, locale)
	switch analysisType {
	case "summarize_thread":
		return T("copilot.summarize_thread", "Sure, I will summarize this thread: %s/_redirect/pl/%s\n", siteURL, postIDToAnalyze)
	case "action_items":
		return T("copilot.find_action_items", "Sure, I will find action items in this thread: %s/_redirect/pl/%s\n", siteURL, postIDToAnalyze)
	case "open_questions":
		return T("copilot.find_open_questions", "Sure, I will find open questions in this thread: %s/_redirect/pl/%s\n", siteURL, postIDToAnalyze)
	default:
		return T("copilot.analyze_thread", "Sure, I will analyze this thread: %s/_redirect/pl/%s\n", siteURL, postIDToAnalyze)
	}
}

func (p *Plugin) startNewAnalysisThread(bot *Bot, postIDToAnalyze string, analysisType string, context llm.ConversationContext) (*model.Post, error) {
	analysisStream, err := p.analyzeThread(bot, postIDToAnalyze, analysisType, context)
	if err != nil {
		return nil, err
	}

	post := p.makeAnalysisPost(context.RequestingUser.Locale, postIDToAnalyze, analysisType)
	if err := p.streamResultToNewDM(bot.mmBot.UserId, analysisStream, context.RequestingUser.Id, post); err != nil {
		return nil, err
	}

	var title string
	switch analysisType {
	case "summarize":
		title = "Thread Summary"
	case "action_items":
		title = "Action Items"
	case "open_questions":
		title = "Open Questions"
	default:
		title = "Thread Analysis"
	}
	p.saveTitleAsync(post.Id, title)

	return post, nil
}
