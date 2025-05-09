// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"errors"
	"fmt"
	"slices"
	"time"

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

// HandleReindexPosts starts a post reindexing job
func (p *AgentsService) HandleReindexPosts() (JobStatus, error) {
	// Check if search is initialized
	if p.search == nil {
		return JobStatus{}, fmt.Errorf("search functionality is not configured")
	}

	// Check if a job is already running
	var jobStatus JobStatus
	err := p.pluginAPI.KV.Get(ReindexJobKey, &jobStatus)
	if err != nil && err.Error() != "not found" {
		return JobStatus{}, fmt.Errorf("failed to check job status: %w", err)
	}

	// If we have a valid job status and it's running, return conflict
	if jobStatus.Status == JobStatusRunning {
		return jobStatus, fmt.Errorf("job already running")
	}

	// Get an estimate of total posts for progress tracking
	var count int64
	dbErr := p.db.Get(&count, `SELECT COUNT(*) FROM Posts WHERE DeleteAt = 0 AND Message != '' AND Type = ''`)
	if dbErr != nil {
		p.pluginAPI.Log.Warn("Failed to get post count for progress tracking", "error", dbErr)
		count = 0 // Continue with zero estimate
	}

	// Create initial job status
	newJobStatus := JobStatus{
		Status:    JobStatusRunning,
		StartedAt: time.Now(),
		TotalRows: count,
	}

	// Save initial job status
	_, err = p.pluginAPI.KV.Set(ReindexJobKey, newJobStatus)
	if err != nil {
		return JobStatus{}, fmt.Errorf("failed to save job status: %w", err)
	}

	// Start the reindexing job in background
	go p.runReindexJob(&newJobStatus)

	return newJobStatus, nil
}

// GetJobStatus gets the status of the reindex job
func (p *AgentsService) GetJobStatus() (JobStatus, error) {
	var jobStatus JobStatus
	err := p.pluginAPI.KV.Get(ReindexJobKey, &jobStatus)
	if err != nil {
		return JobStatus{}, err
	}
	return jobStatus, nil
}

// CancelJob cancels a running reindex job
func (p *AgentsService) CancelJob() (JobStatus, error) {
	var jobStatus JobStatus
	err := p.pluginAPI.KV.Get(ReindexJobKey, &jobStatus)
	if err != nil {
		return JobStatus{}, err
	}

	if jobStatus.Status != JobStatusRunning {
		return JobStatus{}, fmt.Errorf("not running")
	}

	// Update status to canceled
	jobStatus.Status = JobStatusCanceled
	jobStatus.CompletedAt = time.Now()

	// Save updated status
	_, err = p.pluginAPI.KV.Set(ReindexJobKey, jobStatus)
	if err != nil {
		return JobStatus{}, fmt.Errorf("failed to save job status: %w", err)
	}

	return jobStatus, nil
}

// HandleThreadAnalysis handles thread analysis requests
func (p *AgentsService) HandleThreadAnalysis(userID string, bot *Bot, post *model.Post, channel *model.Channel, analysisType string) (map[string]string, error) {
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return nil, fmt.Errorf("unable to get user: %w", err)
	}

	context := p.contextBuilder.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
	)
	createdPost, err := p.startNewAnalysisThread(bot, post.Id, analysisType, context)
	if err != nil {
		return nil, fmt.Errorf("unable to perform analysis: %w", err)
	}

	return map[string]string{
		"postid":    createdPost.Id,
		"channelid": createdPost.ChannelId,
	}, nil
}

const (
	postsPerPage = 60
	maxPosts     = 200
)

func (p *AgentsService) getPostsByChannelBetween(channelID string, startTime, endTime int64) (*model.PostList, error) {
	// Find the ID of first post in our time range
	firstPostID, err := p.getFirstPostBeforeTimeRangeID(channelID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// Initialize result list
	result := &model.PostList{
		Posts: make(map[string]*model.Post),
		Order: []string{},
	}

	// Keep fetching previous pages until we either:
	// 1. Reach the endTime
	// 2. Hit the maxPosts limit
	// 3. Run out of posts
	totalPosts := 0
	page := 0

	for totalPosts < maxPosts {
		morePosts, err := p.pluginAPI.Post.GetPostsBefore(channelID, firstPostID, page, postsPerPage)
		if err != nil {
			return nil, err
		}

		if len(morePosts.Posts) == 0 {
			break // No more posts
		}

		// Add posts that fall within our time range
		for _, post := range morePosts.Posts {
			if post.CreateAt >= startTime && post.CreateAt <= endTime {
				result.Posts[post.Id] = post
				result.Order = append([]string{post.Id}, result.Order...)
				totalPosts++
				if totalPosts >= maxPosts {
					break
				}
			}
			if post.CreateAt < startTime {
				break // We've gone too far back
			}
		}

		page++
	}

	return result, nil
}

// HandleIntervalRequest handles interval analysis requests
func (p *AgentsService) HandleIntervalRequest(userID string, bot *Bot, channel *model.Channel, startTime, endTime int64, presetPrompt, prompt string) (map[string]string, error) {
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return nil, err
	}

	var posts *model.PostList
	if endTime == 0 {
		posts, err = p.pluginAPI.Post.GetPostsSince(channel.Id, startTime)
	} else {
		posts, err = p.getPostsByChannelBetween(channel.Id, startTime, endTime)
	}
	if err != nil {
		return nil, err
	}

	threadData, err := p.getMetadataForPosts(posts)
	if err != nil {
		return nil, err
	}

	// Remove deleted posts
	threadData.Posts = slices.DeleteFunc(threadData.Posts, func(post *model.Post) bool {
		return post.DeleteAt != 0
	})

	formattedThread := formatThread(threadData)

	context := p.contextBuilder.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
		p.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
	)
	context.Parameters = map[string]any{
		"Thread": formattedThread,
	}

	promptPreset := ""
	switch presetPrompt {
	case "summarize_unreads":
		promptPreset = llm.PromptSummarizeChannelSinceSystem
	case "summarize_range":
		promptPreset = llm.PromptSummarizeChannelRangeSystem
	case "action_items":
		promptPreset = llm.PromptFindActionItemsSystem
	case "open_questions":
		promptPreset = llm.PromptFindOpenQuestionsSystem
	default:
		return nil, errors.New("invalid preset prompt")
	}

	systemPrompt, err := p.prompts.Format(promptPreset, context)
	if err != nil {
		return nil, err
	}

	userPrompt, err := p.prompts.Format(llm.PromptThreadUser, context)
	if err != nil {
		return nil, err
	}

	completionRequest := llm.CompletionRequest{
		Posts: []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: systemPrompt,
			},
			{
				Role:    llm.PostRoleUser,
				Message: userPrompt,
			},
		},
		Context: context,
	}

	resultStream, err := p.GetLLM(bot.cfg).ChatCompletion(completionRequest)
	if err != nil {
		return nil, err
	}

	post := &model.Post{}
	post.AddProp(NoRegen, "true")
	// Here we don't have a specific post we're responding to, so pass empty string
	if err := p.streamResultToNewDM(bot.mmBot.UserId, resultStream, user.Id, post, ""); err != nil {
		return nil, err
	}

	promptTitle := ""
	switch presetPrompt {
	case "summarize_unreads":
		promptTitle = "Summarize Unreads"
	case "summarize_range":
		promptTitle = "Summarize Channel"
	case "action_items":
		promptTitle = "Find Action Items"
	case "open_questions":
		promptTitle = "Find Open Questions"
	}

	p.saveTitleAsync(post.Id, promptTitle)

	return map[string]string{
		"postID":    post.Id,
		"channelId": post.ChannelId,
	}, nil
}

// HandleInterPluginSimpleCompletion handles simple completion requests from other plugins
func (p *AgentsService) HandleInterPluginSimpleCompletion(systemPrompt, userPrompt, botUsername, userID string, parameters map[string]any) (string, error) {
	// If bot username is not provided, use the default bot
	if botUsername == "" {
		botUsername = p.getConfiguration().DefaultBotName
	}

	// Get the bot by username or use the first available bot
	bot := p.GetBotByUsernameOrFirst(botUsername)
	if bot == nil {
		return "", fmt.Errorf("failed to get bot: %s", botUsername)
	}

	// Get user information
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %v", err)
	}

	// Create a proper context for the LLM
	context := p.contextBuilder.BuildLLMContextUserRequest(
		bot,
		user,
		nil, // No channel for inter-plugin requests
		p.contextBuilder.WithLLMContextParameters(parameters),
	)

	// Add tools if not disabled
	if !bot.cfg.DisableTools {
		context.Tools = p.contextBuilder.GetToolsStoreForUser(bot, true, userID)
	}

	// Format system prompt using template
	formattedSystemPrompt, err := p.prompts.FormatString(systemPrompt, context)
	if err != nil {
		return "", fmt.Errorf("failed to format system prompt: %v", err)
	}

	formattedUserPrompt, err := p.prompts.FormatString(userPrompt, context)
	if err != nil {
		return "", fmt.Errorf("failed to format user prompt: %v", err)
	}

	// Create a completion request with system prompt and user prompt
	completionRequest := llm.CompletionRequest{
		Posts: []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: formattedSystemPrompt,
			},
			{
				Role:    llm.PostRoleUser,
				Message: formattedUserPrompt,
			},
		},
		Context: context,
	}

	// Execute the completion
	response, err := p.GetLLM(bot.cfg).ChatCompletionNoStream(completionRequest)
	if err != nil {
		return "", fmt.Errorf("completion failed: %v", err)
	}

	return response, nil
}

// DM the user with a standard message. Run the inferance
func (p *AgentsService) analyzeThread(bot *Bot, postIDToAnalyze string, analysisType string, context *llm.Context) (*llm.TextStreamResult, error) {
	posts, err := p.getAnalyzeThreadPosts(postIDToAnalyze, context, analysisType)
	if err != nil {
		return nil, err
	}

	completionReqest := llm.CompletionRequest{
		Posts:   posts,
		Context: context,
	}
	analysisStream, err := p.GetLLM(bot.cfg).ChatCompletion(completionReqest)
	if err != nil {
		return nil, err
	}

	return analysisStream, nil
}

func (p *AgentsService) getAnalyzeThreadPosts(postIDToAnalyze string, context *llm.Context, analysisType string) ([]llm.Post, error) {
	threadData, err := p.getThreadAndMeta(postIDToAnalyze)
	if err != nil {
		return nil, err
	}

	formattedThread := formatThread(threadData)

	context.Parameters = map[string]any{"Thread": formattedThread}
	var promptType string
	switch analysisType {
	case "summarize_thread":
		promptType = llm.PromptSummarizeThreadSystem
	case "action_items":
		promptType = llm.PromptFindActionItemsSystem
	case "open_questions":
		promptType = llm.PromptFindOpenQuestionsSystem
	default:
		return nil, fmt.Errorf("invalid analysis type: %s", analysisType)
	}

	systemPrompt, err := p.prompts.Format(promptType, context)
	if err != nil {
		return nil, fmt.Errorf("failed to format system prompt: %w", err)
	}

	userPrompt, err := p.prompts.Format(llm.PromptThreadUser, context)
	if err != nil {
		return nil, fmt.Errorf("failed to format user prompt: %w", err)
	}

	posts := []llm.Post{
		{
			Role:    llm.PostRoleSystem,
			Message: systemPrompt,
		},
		{
			Role:    llm.PostRoleUser,
			Message: userPrompt,
		},
	}
	return posts, nil
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

func (p *AgentsService) startNewAnalysisThread(bot *Bot, postIDToAnalyze string, analysisType string, context *llm.Context) (*model.Post, error) {
	analysisStream, err := p.analyzeThread(bot, postIDToAnalyze, analysisType, context)
	if err != nil {
		return nil, err
	}

	post := p.makeAnalysisPost(context.RequestingUser.Locale, postIDToAnalyze, analysisType)
	if err := p.streamResultToNewDM(bot.mmBot.UserId, analysisStream, context.RequestingUser.Id, post, postIDToAnalyze); err != nil {
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
