// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api

import (
	stdcontext "context"
	"fmt"
	"net/http"

	"errors"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/conversations"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/react"
	"github.com/mattermost/mattermost-plugin-ai/threads"
	"github.com/mattermost/mattermost/server/public/model"
)

func (a *API) postAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	postID := c.Param("postid")

	post, err := a.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Set(ContextPostKey, post)

	channel, err := a.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Set(ContextChannelKey, channel)

	if !a.pluginAPI.User.HasPermissionToChannel(userID, channel.Id, model.PermissionReadChannel) {
		c.AbortWithError(http.StatusForbidden, errors.New("user doesn't have permission to read channel post in in"))
		return
	}

	bot := c.MustGet(ContextBotKey).(*bots.Bot)
	if err := a.bots.CheckUsageRestrictions(userID, bot, channel); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}
}

func (a *API) handleReact(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*bots.Bot)

	requestingUser, err := a.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	context := a.contextBuilder.BuildLLMContextUserRequest(
		bot,
		requestingUser,
		channel,
	)

	emojiName, err := react.New(
		bot.LLM(),
		a.prompts,
	).Resolve(post.Message, context)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Add reaction to the post
	if err := a.pluginAPI.Post.AddReaction(&model.Reaction{
		EmojiName: emojiName,
		UserId:    bot.GetMMBot().UserId,
		PostId:    post.Id,
	}); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to add reaction: %w", err))
	}

	c.Status(http.StatusOK)
}

func (a *API) handleThreadAnalysis(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*bots.Bot)

	if !a.conversationsService.IsBasicsLicensed() {
		c.AbortWithError(http.StatusForbidden, errors.New("feature not licensed"))
		return
	}

	var data struct {
		AnalysisType string `json:"analysis_type" binding:"required"`
	}
	if bindErr := c.ShouldBindJSON(&data); bindErr != nil {
		c.AbortWithError(http.StatusBadRequest, bindErr)
		return
	}

	switch data.AnalysisType {
	case "summarize_thread":
		// Valid analysis type for thread summarization
	case "action_items":
		// Valid analysis type for finding action items
	case "open_questions":
		// Valid analysis type for finding open questions
	default:
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid analysis type: %s", data.AnalysisType))
		return
	}

	// Get the user to build context
	user, err := a.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get user: %w", err))
		return
	}

	// Build LLM context
	llmContext := a.contextBuilder.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
		a.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.GetMMBot().UserId, channel)),
	)

	// Create thread analyzer
	analyzer := threads.New(bot.LLM(), a.prompts, a.mmClient)
	var analysisStream *llm.TextStreamResult
	var title string
	switch data.AnalysisType {
	case "summarize_thread":
		title = "Thread Summary"
		analysisStream, err = analyzer.Summarize(post.Id, llmContext)
	case "action_items":
		title = "Action Items"
		analysisStream, err = analyzer.FindActionItems(post.Id, llmContext)
	case "open_questions":
		title = "Open Questions"
		analysisStream, err = analyzer.FindOpenQuestions(post.Id, llmContext)
	}
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to analyze thread: %w", err))
		return
	}

	// Create analysis post
	siteURL := a.pluginAPI.Configuration.GetConfig().ServiceSettings.SiteURL
	analysisPost := a.makeAnalysisPost(user.Locale, post.Id, data.AnalysisType, *siteURL)
	if err := a.conversationsService.StreamToNewDM(stdcontext.Background(), bot.GetMMBot().UserId, analysisStream, user.Id, analysisPost, post.Id); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	a.conversationsService.SaveTitleAsync(post.Id, title)

	c.JSON(http.StatusOK, map[string]string{
		"postid":    analysisPost.Id,
		"channelid": analysisPost.ChannelId,
	})
}

func (a *API) handleTranscribeFile(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	fileID := c.Param("fileid")
	bot := c.MustGet(ContextBotKey).(*bots.Bot)

	result, err := a.meetingsService.HandleTranscribeFile(userID, bot, post, channel, fileID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Render(http.StatusOK, render.JSON{Data: result})
}

func (a *API) handleSummarizeTranscription(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*bots.Bot)

	result, err := a.meetingsService.HandleSummarizeTranscription(userID, bot, post, channel)
	if err != nil {
		if err.Error() == "not a calls or zoom bot post" {
			c.AbortWithError(http.StatusBadRequest, errors.New("not a calls or zoom bot post"))
			return
		}
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to summarize transcription: %w", err))
		return
	}

	c.Render(http.StatusOK, render.JSON{Data: result})
}

func (a *API) handleStop(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)

	botID := post.UserId
	if !a.bots.IsAnyBot(botID) {
		c.AbortWithError(http.StatusBadRequest, errors.New("not a bot post"))
		return
	}

	if post.GetProp(conversations.LLMRequesterUserID) != userID {
		c.AbortWithError(http.StatusForbidden, errors.New("only the original poster can stop the stream"))
		return
	}

	a.conversationsService.StopPostStreaming(post.Id)
	c.Status(http.StatusOK)
}

func (a *API) handleRegenerate(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)

	err := a.conversationsService.HandleRegenerate(userID, post, channel)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to regenerate post: %w", err))
		return
	}

	c.Status(http.StatusOK)
}

func (a *API) handleToolCall(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)

	if !a.conversationsService.IsBasicsLicensed() {
		c.AbortWithError(http.StatusForbidden, errors.New("feature not licensed"))
		return
	}

	// Only the original requester can approve/reject tool calls
	if post.GetProp(conversations.LLMRequesterUserID) != userID {
		c.AbortWithError(http.StatusForbidden, errors.New("only the original requester can approve/reject tool calls"))
		return
	}

	var data struct {
		AcceptedToolIDs []string `json:"accepted_tool_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&data); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	err := a.conversationsService.HandleToolCall(userID, post, channel, data.AcceptedToolIDs)
	if err != nil {
		if err.Error() == "post missing pending tool calls" || err.Error() == "post pending tool calls not valid JSON" {
			c.AbortWithError(http.StatusBadRequest, err)
		} else {
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		return
	}

	c.Status(http.StatusOK)
}

func (a *API) handlePostbackSummary(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)

	result, err := a.meetingsService.HandlePostbackSummary(userID, post)
	if err != nil {
		if err.Error() == "post missing reference to transcription post ID" {
			c.AbortWithError(http.StatusBadRequest, err)
		} else {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to post back summary: %w", err))
		}
		return
	}

	c.Render(http.StatusOK, render.JSON{Data: result})
}

// makeAnalysisPost creates a post for thread analysis results
func (a *API) makeAnalysisPost(locale string, postIDToAnalyze string, analysisType string, siteURL string) *model.Post {
	post := &model.Post{
		Message: a.analysisPostMessage(locale, postIDToAnalyze, analysisType, siteURL),
	}
	post.AddProp(conversations.ThreadIDProp, postIDToAnalyze)
	post.AddProp(conversations.AnalysisTypeProp, analysisType)

	return post
}

func (a *API) analysisPostMessage(locale string, postIDToAnalyze string, analysisType string, siteURL string) string {
	T := i18n.LocalizerFunc(a.conversationsService.GetI18nBundle(), locale)
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
