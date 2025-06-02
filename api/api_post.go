// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api

import (
	"fmt"
	"net/http"

	"errors"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost-plugin-ai/agents"
	"github.com/mattermost/mattermost-plugin-ai/agents/react"
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

	bot := c.MustGet(ContextBotKey).(*agents.Bot)
	if err := a.agents.CheckUsageRestrictions(userID, bot, channel); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}
}

func (a *API) handleReact(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*agents.Bot)

	if err := a.enforceEmptyBody(c); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	requestingUser, err := a.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	context := a.agents.GetContextBuilder().BuildLLMContextUserRequest(
		bot,
		requestingUser,
		channel,
	)

	emojiName, err := react.New(
		a.agents.GetLLM(bot.GetConfig()),
		a.agents.GetPrompts(),
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
	bot := c.MustGet(ContextBotKey).(*agents.Bot)

	if !a.agents.IsBasicsLicensed() {
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

	createdPost, err := a.agents.ThreadAnalysis(userID, bot, post, channel, data.AnalysisType)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to perform analysis: %w", err))
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"postid":    createdPost.Id,
		"channelid": createdPost.ChannelId,
	})
}

func (a *API) handleTranscribeFile(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	fileID := c.Param("fileid")
	bot := c.MustGet(ContextBotKey).(*agents.Bot)

	if err := a.enforceEmptyBody(c); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	result, err := a.agents.HandleTranscribeFile(userID, bot, post, channel, fileID)
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
	bot := c.MustGet(ContextBotKey).(*agents.Bot)

	if err := a.enforceEmptyBody(c); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	result, err := a.agents.HandleSummarizeTranscription(userID, bot, post, channel)
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

	if err := a.enforceEmptyBody(c); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	botID := post.UserId
	if !a.agents.IsAnyBot(botID) {
		c.AbortWithError(http.StatusBadRequest, errors.New("not a bot post"))
		return
	}

	if post.GetProp(agents.LLMRequesterUserID) != userID {
		c.AbortWithError(http.StatusForbidden, errors.New("only the original poster can stop the stream"))
		return
	}

	a.agents.StopPostStreaming(post.Id)
	c.Status(http.StatusOK)
}

func (a *API) handleRegenerate(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)

	if err := a.enforceEmptyBody(c); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	err := a.agents.HandleRegenerate(userID, post, channel)
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

	if !a.agents.IsBasicsLicensed() {
		c.AbortWithError(http.StatusForbidden, errors.New("feature not licensed"))
		return
	}

	// Only the original requester can approve/reject tool calls
	if post.GetProp(agents.LLMRequesterUserID) != userID {
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

	err := a.agents.HandleToolCall(userID, post, channel, data.AcceptedToolIDs)
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

	if err := a.enforceEmptyBody(c); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	result, err := a.agents.HandlePostbackSummary(userID, post)
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
