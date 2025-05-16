// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/mattermost/mattermost-plugin-ai/llm/subtitles"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	ReferencedRecordingFileID  = "referenced_recording_file_id"
	ReferencedTranscriptPostID = "referenced_transcript_post_id"
)

// Transcriber interface needs to be defined here as it's used by plugin.go
type Transcriber interface {
	Transcribe(file io.Reader) (*subtitles.Subtitles, error)
}

// HandleTranscribeFile handles file transcription requests
func (p *AgentsService) HandleTranscribeFile(userID string, bot *Bot, post *model.Post, channel *model.Channel, fileID string) (map[string]string, error) {
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return nil, err
	}

	recordingFileInfo, err := p.pluginAPI.File.GetInfo(fileID)
	if err != nil {
		return nil, err
	}

	if recordingFileInfo.ChannelId != channel.Id || !slices.Contains(post.FileIds, fileID) {
		return nil, errors.New("file not attached to specified post")
	}

	createdPost, err := p.newCallRecordingThread(bot, user, post, channel, fileID)
	if err != nil {
		return nil, err
	}

	if err := p.saveTitle(createdPost.Id, "Meeting Summary"); err != nil {
		return nil, fmt.Errorf("failed to save title: %w", err)
	}

	return map[string]string{
		"postid":    createdPost.Id,
		"channelid": createdPost.ChannelId,
	}, nil
}

// HandleSummarizeTranscription handles transcription summarization requests
func (p *AgentsService) HandleSummarizeTranscription(userID string, bot *Bot, post *model.Post, channel *model.Channel) (map[string]string, error) {
	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return nil, fmt.Errorf("unable to get user: %w", err)
	}

	targetPostUser, err := p.pluginAPI.User.Get(post.UserId)
	if err != nil {
		return nil, fmt.Errorf("unable to get calls user: %w", err)
	}

	if !targetPostUser.IsBot || (targetPostUser.Username != CallsBotUsername && targetPostUser.Username != ZoomBotUsername) {
		return nil, errors.New("not a calls or zoom bot post")
	}

	createdPost, err := p.newCallTranscriptionSummaryThread(bot, user, post, channel)
	if err != nil {
		return nil, fmt.Errorf("unable to summarize transcription: %w", err)
	}

	p.saveTitleAsync(createdPost.Id, "Meeting Summary")

	return map[string]string{
		"postid":    createdPost.Id,
		"channelid": createdPost.ChannelId,
	}, nil
}

// HandlePostbackSummary handles posting back a summary to the original channel
func (p *AgentsService) HandlePostbackSummary(userID string, post *model.Post) (map[string]string, error) {
	bot := p.GetBotByID(post.UserId)
	if bot == nil {
		return nil, fmt.Errorf("unable to get bot")
	}

	if post.GetProp(LLMRequesterUserID) != userID {
		return nil, errors.New("only the original requester can post back")
	}

	transcriptThreadRootPost, err := p.pluginAPI.Post.GetPost(post.RootId)
	if err != nil {
		return nil, fmt.Errorf("unable to get transcript thread root post: %w", err)
	}

	originalTranscriptPostID, ok := transcriptThreadRootPost.GetProp(ReferencedTranscriptPostID).(string)
	if !ok || originalTranscriptPostID == "" {
		return nil, errors.New("post missing reference to transcription post ID")
	}

	transcriptionPost, err := p.pluginAPI.Post.GetPost(originalTranscriptPostID)
	if err != nil {
		return nil, fmt.Errorf("unable to get transcription post: %w", err)
	}

	if !p.pluginAPI.User.HasPermissionToChannel(userID, transcriptionPost.ChannelId, model.PermissionCreatePost) {
		return nil, errors.New("user doesn't have permission to create a post in the transcript channel")
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
		return nil, fmt.Errorf("unable to post back summary: %w", err)
	}

	return map[string]string{
		"rootid":    postedSummary.RootId,
		"channelid": postedSummary.ChannelId,
	}, nil
}
