// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

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
