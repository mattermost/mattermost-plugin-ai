// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"errors"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost-plugin-ai/server/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost-plugin-ai/server/llm/subtitles"
	"github.com/mattermost/mattermost-plugin-ai/server/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) postAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	postID := c.Param("postid")

	post, err := p.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Set(ContextPostKey, post)

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Set(ContextChannelKey, channel)

	if !p.pluginAPI.User.HasPermissionToChannel(userID, channel.Id, model.PermissionReadChannel) {
		c.AbortWithError(http.StatusForbidden, errors.New("user doesn't have permission to read channel post in in"))
		return
	}

	bot := c.MustGet(ContextBotKey).(*Bot)
	if err := p.checkUsageRestrictions(userID, bot, channel); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}
}

func (p *Plugin) handleReact(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*Bot)

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	context := p.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
	)
	context.Parameters = map[string]any{"Message": post.Message}
	prompt, err := p.prompts.Format(llm.PromptEmojiSelectSystem, context)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	completionRequest := llm.CompletionRequest{
		Posts: []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: prompt,
			},
			{
				Role:    llm.PostRoleUser,
				Message: post.Message,
			},
		},
		Context: context,
	}
	emojiName, err := p.getLLM(bot.cfg).ChatCompletionNoStream(completionRequest, llm.WithMaxGeneratedTokens(25))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Do some emoji post-processing to hopefully make this an actual emoji.
	emojiName = strings.Trim(strings.TrimSpace(emojiName), ":")

	if _, found := model.GetSystemEmojiId(emojiName); !found {
		_ = p.pluginAPI.Post.AddReaction(&model.Reaction{
			EmojiName: "large_red_square",
			UserId:    bot.mmBot.UserId,
			PostId:    post.Id,
		})

		c.AbortWithError(http.StatusInternalServerError, errors.New("LLM returned somthing other than emoji: "+emojiName))
		return
	}

	if err := p.pluginAPI.Post.AddReaction(&model.Reaction{
		EmojiName: emojiName,
		UserId:    bot.mmBot.UserId,
		PostId:    post.Id,
	}); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

func (p *Plugin) handleThreadAnalysis(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*Bot)

	if !p.licenseChecker.IsBasicsLicensed() {
		c.AbortWithError(http.StatusForbidden, enterprise.ErrNotLicensed)
		return
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
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
	case "action_items":
	case "open_questions":
		break
	default:
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid analysis type: %s", data.AnalysisType))
		return
	}

	context := p.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
	)
	createdPost, err := p.startNewAnalysisThread(bot, post.Id, data.AnalysisType, context)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to perform analysis: %w", err))
		return
	}

	result := struct {
		PostID    string `json:"postid"`
		ChannelID string `json:"channelid"`
	}{
		PostID:    createdPost.Id,
		ChannelID: createdPost.ChannelId,
	}
	c.JSON(http.StatusOK, result)
}

func (p *Plugin) handleTranscribeFile(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	fileID := c.Param("fileid")
	bot := c.MustGet(ContextBotKey).(*Bot)

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	recordingFileInfo, err := p.pluginAPI.File.GetInfo(fileID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if recordingFileInfo.ChannelId != channel.Id || !slices.Contains(post.FileIds, fileID) {
		c.AbortWithError(http.StatusBadRequest, errors.New("file not attached to specified post"))
		return
	}

	createdPost, err := p.newCallRecordingThread(bot, user, post, channel, fileID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := p.saveTitle(createdPost.Id, "Meeting Summary"); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to save title: %w", err))
		return
	}

	data := struct {
		PostID    string `json:"postid"`
		ChannelID string `json:"channelid"`
	}{
		PostID:    createdPost.Id,
		ChannelID: createdPost.ChannelId,
	}
	c.Render(http.StatusOK, render.JSON{Data: data})
}

func (p *Plugin) handleSummarizeTranscription(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*Bot)

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get user: %w", err))
		return
	}

	targetPostUser, err := p.pluginAPI.User.Get(post.UserId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get calls user: %w", err))
		return
	}
	if !targetPostUser.IsBot || (targetPostUser.Username != CallsBotUsername && targetPostUser.Username != ZoomBotUsername) {
		c.AbortWithError(http.StatusBadRequest, errors.New("not a calls or zoom bot post"))
		return
	}

	createdPost, err := p.newCallTranscriptionSummaryThread(bot, user, post, channel)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to summarize transcription: %w", err))
		return
	}

	p.saveTitleAsync(createdPost.Id, "Meeting Summary")

	data := struct {
		PostID    string `json:"postid"`
		ChannelID string `json:"channelid"`
	}{
		PostID:    createdPost.Id,
		ChannelID: createdPost.ChannelId,
	}
	c.Render(http.StatusOK, render.JSON{Data: data})
}

func (p *Plugin) handleStop(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)

	if p.GetBotByID(post.UserId) == nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("not a bot post"))
		return
	}

	if post.GetProp(LLMRequesterUserID) != userID {
		c.AbortWithError(http.StatusForbidden, errors.New("only the original poster can stop the stream"))
		return
	}

	p.stopPostStreaming(post.Id)
}

func (p *Plugin) handleRegenerate(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)

	bot := p.GetBotByID(post.UserId)
	if bot == nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get bot"))
		return
	}

	if post.GetProp(LLMRequesterUserID) != userID {
		c.AbortWithError(http.StatusForbidden, errors.New("only the original poster can regenerate"))
		return
	}

	if post.GetProp(NoRegen) != nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("taged no regen"))
		return
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get user to regen post: %w", err))
		return
	}

	if err := p.regeneratePost(bot, post, user, channel); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to regenerate post: %w", err))
		return
	}
}

func (p *Plugin) regeneratePost(bot *Bot, post *model.Post, user *model.User, channel *model.Channel) error {
	ctx, err := p.getPostStreamingContext(context.Background(), post.Id)
	if err != nil {
		return err
	}
	defer p.finishPostStreaming(post.Id)

	threadIDProp := post.GetProp(ThreadIDProp)
	analysisTypeProp := post.GetProp(AnalysisTypeProp)
	referenceRecordingFileIDProp := post.GetProp(ReferencedRecordingFileID)
	referencedTranscriptPostProp := post.GetProp(ReferencedTranscriptPostID)
	post.DelProp(ToolCallProp)
	var result *llm.TextStreamResult
	switch {
	case threadIDProp != nil:
		threadID := threadIDProp.(string)
		analysisType := analysisTypeProp.(string)
		siteURL := p.API.GetConfig().ServiceSettings.SiteURL
		post.Message = p.analysisPostMessage(user.Locale, threadID, analysisType, *siteURL)

		var err error
		result, err = p.analyzeThread(bot, threadID, analysisType, p.BuildLLMContextUserRequest(
			bot,
			user,
			channel,
			p.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
			p.WithLLMContextToolCallCallback(post.Id),
		))
		if err != nil {
			return fmt.Errorf("could not summarize post on regen: %w", err)
		}
	case referenceRecordingFileIDProp != nil:
		post.Message = ""
		referencedRecordingFileID := referenceRecordingFileIDProp.(string)

		fileInfo, getErr := p.pluginAPI.File.GetInfo(referencedRecordingFileID)
		if getErr != nil {
			return fmt.Errorf("could not get transcription file on regen: %w", getErr)
		}

		reader, getErr := p.pluginAPI.File.Get(post.FileIds[0])
		if getErr != nil {
			return fmt.Errorf("could not get transcription file on regen: %w", getErr)
		}
		transcription, parseErr := subtitles.NewSubtitlesFromVTT(reader)
		if parseErr != nil {
			return fmt.Errorf("could not parse transcription file on regen: %w", parseErr)
		}

		if transcription.IsEmpty() {
			return errors.New("transcription is empty on regen")
		}

		originalFileChannel, channelErr := p.pluginAPI.Channel.Get(fileInfo.ChannelId)
		if channelErr != nil {
			return fmt.Errorf("could not get channel of original recording on regen: %w", channelErr)
		}

		context := p.BuildLLMContextUserRequest(
			bot,
			user,
			originalFileChannel,
			p.WithLLMContextDefaultTools(bot, originalFileChannel.Type == model.ChannelTypeDirect),
			p.WithLLMContextToolCallCallback(post.Id),
		)
		var summaryErr error
		result, summaryErr = p.summarizeTranscription(bot, transcription, context)
		if summaryErr != nil {
			return fmt.Errorf("could not summarize transcription on regen: %w", summaryErr)
		}
	case referencedTranscriptPostProp != nil:
		post.Message = ""
		referencedTranscriptionPostID := referencedTranscriptPostProp.(string)
		referencedTranscriptionPost, postErr := p.pluginAPI.Post.GetPost(referencedTranscriptionPostID)
		if postErr != nil {
			return fmt.Errorf("could not get transcription post on regen: %w", postErr)
		}

		transcriptionFileID, fileIDErr := getCaptionsFileIDFromProps(referencedTranscriptionPost)
		if fileIDErr != nil {
			return fmt.Errorf("unable to get transcription file id: %w", fileIDErr)
		}
		transcriptionFileReader, fileErr := p.pluginAPI.File.Get(transcriptionFileID)
		if fileErr != nil {
			return fmt.Errorf("unable to read calls file: %w", fileErr)
		}

		transcription, parseErr := subtitles.NewSubtitlesFromVTT(transcriptionFileReader)
		if parseErr != nil {
			return fmt.Errorf("unable to parse transcription file: %w", parseErr)
		}

		context := p.BuildLLMContextUserRequest(
			bot,
			user,
			channel,
			p.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
			p.WithLLMContextToolCallCallback(post.Id),
		)
		var summaryErr error
		result, summaryErr = p.summarizeTranscription(bot, transcription, context)
		if summaryErr != nil {
			return fmt.Errorf("unable to summarize transcription: %w", summaryErr)
		}

	default:
		post.Message = ""

		respondingToPostID, ok := post.GetProp(RespondingToProp).(string)
		if !ok {
			return errors.New("post missing responding to prop")
		}
		respondingToPost, getErr := p.pluginAPI.Post.GetPost(respondingToPostID)
		if getErr != nil {
			return fmt.Errorf("could not get post being responded to: %w", getErr)
		}

		// Create a context with the tool call callback already set
		contextWithCallback := p.BuildLLMContextUserRequest(
			bot,
			user,
			channel,
			p.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
			p.WithLLMContextToolCallCallback(post.Id),
		)

		// Process the user request with the context that has the callback
		var processErr error
		result, processErr = p.processUserRequestWithContext(bot, user, channel, respondingToPost, contextWithCallback)
		if processErr != nil {
			return fmt.Errorf("could not continue conversation on regen: %w", processErr)
		}
	}

	if mmapi.IsDMWith(bot.mmBot.UserId, channel) {
		if channel.Name == bot.mmBot.UserId+"__"+user.Id || channel.Name == user.Id+"__"+bot.mmBot.UserId {
			p.streamResultToPost(ctx, result, post, user.Locale)
			return nil
		}
	}

	p.streamResultToPost(ctx, result, post, *p.API.GetConfig().LocalizationSettings.DefaultServerLocale)

	return nil
}

func (p *Plugin) handleToolCall(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := p.GetBotByID(post.UserId)

	if !p.licenseChecker.IsBasicsLicensed() {
		c.AbortWithError(http.StatusForbidden, enterprise.ErrNotLicensed)
		return
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Only the original requester can approve/reject tool calls
	if post.GetProp(LLMRequesterUserID) != userID {
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

	toolsJSON := post.GetProp(ToolCallProp)
	if toolsJSON == nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("post missing pending tool calls"))
		return
	}

	var tools []llm.ToolCall
	if err := json.Unmarshal([]byte(toolsJSON.(string)), &tools); err != nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("post pending tool calls not valid JSON"))
		return
	}

	context := p.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
		p.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
	)

	for i := range tools {
		if slices.Contains(data.AcceptedToolIDs, tools[i].ID) {
			result, err := context.Tools.ResolveTool(tools[i].Name, func(args any) error {
				return json.Unmarshal(tools[i].Arguments, args)
			}, context)
			if err != nil {
				// Maybe in the future we can return this to the user and have a retry. For now just tell the LLM it failed.
				tools[i].Result = "Tool call failed"
				tools[i].Status = llm.ToolCallStatusError
				continue
			}
			tools[i].Result = result
			tools[i].Status = llm.ToolCallStatusSuccess
		} else {
			tools[i].Result = "Tool call rejected by user"
			tools[i].Status = llm.ToolCallStatusRejected
		}
	}

	responseRootID := post.Id
	if post.RootId != "" {
		responseRootID = post.RootId
	}

	// Update post with the tool call results
	resolvedToolsJSON, err := json.Marshal(tools)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to marshal tool call results: %w", err))
		return
	}
	post.AddProp(ToolCallProp, string(resolvedToolsJSON))

	if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to update post with tool call results: %w", err))
		return
	}

	// Only continue if at lest one tool call was successful
	if !slices.ContainsFunc(tools, func(tc llm.ToolCall) bool {
		return tc.Status == llm.ToolCallStatusSuccess
	}) {
		c.Status(http.StatusOK)
		return
	}

	previousConversation, err := p.getThreadAndMeta(responseRootID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get previous conversation: %w", err))
		return
	}
	previousConversation.cutoffBeforePostID(post.Id)
	previousConversation.Posts[len(previousConversation.Posts)-1] = post

	posts, err := p.existingConversationToLLMPosts(bot, previousConversation, context)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to convert existing conversation to LLM posts: %w", err))
		return
	}

	completionRequest := llm.CompletionRequest{
		Posts:   posts,
		Context: context,
	}
	result, err := p.getLLM(bot.cfg).ChatCompletion(completionRequest)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get chat completion: %w", err))
		return
	}

	responsePost := &model.Post{
		ChannelId: channel.Id,
		RootId:    responseRootID,
	}
	if err := p.streamResultToNewPost(bot.mmBot.UserId, user.Id, result, responsePost, post.Id); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to stream result to new post: %w", err))
		return
	}

	c.Status(http.StatusOK)
}

func (p *Plugin) handlePostbackSummary(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)

	bot := p.GetBotByID(post.UserId)
	if bot == nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get bot"))
		return
	}

	if post.GetProp(LLMRequesterUserID) != userID {
		c.AbortWithError(http.StatusForbidden, errors.New("only the original requester can post back"))
		return
	}

	transcriptThreadRootPost, err := p.pluginAPI.Post.GetPost(post.RootId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get transcript thread root post: %w", err))
		return
	}

	originalTranscriptPostID, ok := transcriptThreadRootPost.GetProp(ReferencedTranscriptPostID).(string)
	if !ok || originalTranscriptPostID == "" {
		c.AbortWithError(http.StatusBadRequest, errors.New("post missing reference to transcription post ID"))
		return
	}

	transcriptionPost, err := p.pluginAPI.Post.GetPost(originalTranscriptPostID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to get transcription post: %w", err))
		return
	}

	if !p.pluginAPI.User.HasPermissionToChannel(userID, transcriptionPost.ChannelId, model.PermissionCreatePost) {
		c.AbortWithError(http.StatusForbidden, errors.New("user doesn't have permission to create a post in the transcript channel"))
		return
	}

	postedSummary := &model.Post{
		UserId:    bot.mmBot.UserId,
		ChannelId: transcriptionPost.ChannelId,
		RootId:    transcriptionPost.RootId,
		Message:   post.Message,
		Type:      "custom_llm_postback",
	}
	postedSummary.AddProp("userid", userID)
	if err := p.pluginAPI.Post.CreatePost(postedSummary); err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to post back summary: %w", err))
		return
	}

	data := struct {
		PostID    string `json:"rootid"`
		ChannelID string `json:"channelid"`
	}{
		PostID:    postedSummary.RootId,
		ChannelID: postedSummary.ChannelId,
	}
	c.Render(http.StatusOK, render.JSON{Data: data})
}
