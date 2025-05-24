// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/agents/threads"
	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	ThreadIDProp     = "referenced_thread"
	AnalysisTypeProp = "prompt_type"
	JobStatusError   = "error"
)

func (p *AgentsService) ThreadAnalysis(userID string, bot *bots.Bot, post *model.Post, channel *model.Channel, analysisType string) (*model.Post, error) {
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return nil, fmt.Errorf("unable to get user: %w", err)
	}

	context := p.contextBuilder.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
		p.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.GetMMBot().UserId, channel)),
	)

	analyzer := threads.New(p.GetLLM(bot.GetConfig()), p.prompts, p.mmClient)
	var analysisStream *llm.TextStreamResult
	var title string
	switch analysisType {
	case "summarize_thread":
		title = "Thread Summary"
		analysisStream, err = analyzer.Summarize(post.Id, context)
	case "action_items":
		title = "Action Items"
		analysisStream, err = analyzer.FindActionItems(post.Id, context)
	case "open_questions":
		title = "Open Questions"
		analysisStream, err = analyzer.FindOpenQuestions(post.Id, context)
	default:
		return nil, fmt.Errorf("invalid analysis type: %s", analysisType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to analyze thread: %w", err)
	}

	analysisPost := p.makeAnalysisPost(context.RequestingUser.Locale, post.Id, analysisType)
	if err := p.streamResultToNewDM(bot.GetMMBot().UserId, analysisStream, context.RequestingUser.Id, analysisPost, post.Id); err != nil {
		return nil, err
	}

	p.saveTitleAsync(post.Id, title)

	return analysisPost, nil
}

func (p *AgentsService) makeAnalysisPost(locale string, postIDToAnalyze string, analysisType string) *model.Post {
	siteURL := p.pluginAPI.Configuration.GetConfig().ServiceSettings.SiteURL
	post := &model.Post{
		Message: p.analysisPostMessage(locale, postIDToAnalyze, analysisType, *siteURL),
	}
	post.AddProp(ThreadIDProp, postIDToAnalyze)
	post.AddProp(AnalysisTypeProp, analysisType)

	return post
}

func (p *AgentsService) analysisPostMessage(locale string, postIDToAnalyze string, analysisType string, siteURL string) string {
	T := i18n.LocalizerFunc(p.i18n, locale)
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
