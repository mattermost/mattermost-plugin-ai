// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/streaming"
	"github.com/mattermost/mattermost-plugin-ai/subtitles"
	"github.com/mattermost/mattermost-plugin-ai/threads"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	ReferencedRecordingFileID  = "referenced_recording_file_id"
	ReferencedTranscriptPostID = "referenced_transcript_post_id"
)

// HandleRegenerate handles post regeneration requests
func (c *Conversations) HandleRegenerate(userID string, post *model.Post, channel *model.Channel) error {
	bot := c.bots.GetBotByID(post.UserId)
	if bot == nil {
		return fmt.Errorf("unable to get bot")
	}

	if post.GetProp(LLMRequesterUserID) != userID {
		return errors.New("only the original poster can regenerate")
	}

	if post.GetProp(NoRegen) != nil {
		return errors.New("tagged no regen")
	}

	user, err := c.pluginAPI.User.Get(userID)
	if err != nil {
		return fmt.Errorf("unable to get user to regen post: %w", err)
	}

	ctx, err := c.streamingService.GetStreamingContext(context.Background(), post.Id)
	if err != nil {
		return fmt.Errorf("unable to get post streaming context: %w", err)
	}
	defer c.streamingService.FinishStreaming(post.Id)

	threadIDProp := post.GetProp(ThreadIDProp)
	analysisTypeProp := post.GetProp(AnalysisTypeProp)
	referenceRecordingFileIDProp := post.GetProp(ReferencedRecordingFileID)
	referencedTranscriptPostProp := post.GetProp(ReferencedTranscriptPostID)
	post.DelProp(streaming.ToolCallProp)
	var result *llm.TextStreamResult
	switch {
	case threadIDProp != nil:
		threadID := threadIDProp.(string)
		analysisType := analysisTypeProp.(string)
		// 		config := c.pluginAPI.Configuration.GetConfig()
		// 		siteURL := config.ServiceSettings.SiteURL
		// TODO: Move analysisPostMessage to conversations package
		post.Message = "" // c.analysisPostMessage(user.Locale, threadID, analysisType, *siteURL)

		llmContext := c.contextBuilder.BuildLLMContextUserRequest(
			bot,
			user,
			channel,
			c.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.GetMMBot().UserId, channel)),
		)

		var err error
		result, err = threads.New(bot.LLM(), c.prompts, c.mmClient).Analyze(threadID, llmContext, analysisType)
		if err != nil {
			return fmt.Errorf("could not summarize post on regen: %w", err)
		}
	case referenceRecordingFileIDProp != nil:
		post.Message = ""
		referencedRecordingFileID := referenceRecordingFileIDProp.(string)

		fileInfo, getErr := c.pluginAPI.File.GetInfo(referencedRecordingFileID)
		if getErr != nil {
			return fmt.Errorf("could not get transcription file on regen: %w", getErr)
		}

		reader, getErr := c.pluginAPI.File.Get(post.FileIds[0])
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

		originalFileChannel, channelErr := c.pluginAPI.Channel.Get(fileInfo.ChannelId)
		if channelErr != nil {
			return fmt.Errorf("could not get channel of original recording on regen: %w", channelErr)
		}

		// 		_context := c.contextBuilder.BuildLLMContextUserRequest(
		// 			bot,
		// 			user,
		// 			originalFileChannel,
		// 			c.contextBuilder.WithLLMContextDefaultTools(bot, originalFileChannel.Type == model.ChannelTypeDirect),
		// 		)
		var summaryErr error
		// TODO: Move summarizeTranscription to conversations package
		_ = transcription
		_ = originalFileChannel
		result = nil
		summaryErr = fmt.Errorf("summarizeTranscription not implemented yet")
		if summaryErr != nil {
			return fmt.Errorf("could not summarize transcription on regen: %w", summaryErr)
		}
	case referencedTranscriptPostProp != nil:
		post.Message = ""
		referencedTranscriptionPostID := referencedTranscriptPostProp.(string)
		referencedTranscriptionPost, postErr := c.pluginAPI.Post.GetPost(referencedTranscriptionPostID)
		if postErr != nil {
			return fmt.Errorf("could not get transcription post on regen: %w", postErr)
		}

		// TODO: Move getCaptionsFileIDFromProps to conversations package
		_ = referencedTranscriptionPost
		transcriptionFileID := ""
		fileIDErr := fmt.Errorf("getCaptionsFileIDFromProps not implemented yet")
		if fileIDErr != nil {
			return fmt.Errorf("unable to get transcription file id: %w", fileIDErr)
		}
		transcriptionFileReader, fileErr := c.pluginAPI.File.Get(transcriptionFileID)
		if fileErr != nil {
			return fmt.Errorf("unable to read calls file: %w", fileErr)
		}

		transcription, parseErr := subtitles.NewSubtitlesFromVTT(transcriptionFileReader)
		if parseErr != nil {
			return fmt.Errorf("unable to parse transcription file: %w", parseErr)
		}

		// 		_context := c.contextBuilder.BuildLLMContextUserRequest(
		// 			bot,
		// 			user,
		// 			channel,
		// 			c.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.GetMMBot().UserId, channel)),
		// 		)
		var summaryErr error
		// TODO: Move summarizeTranscription to conversations package
		_ = transcription
		_ = channel
		result = nil
		summaryErr = fmt.Errorf("summarizeTranscription not implemented yet")
		if summaryErr != nil {
			return fmt.Errorf("unable to summarize transcription: %w", summaryErr)
		}

	default:
		post.Message = ""

		respondingToPostID, ok := post.GetProp(RespondingToProp).(string)
		if !ok {
			return errors.New("post missing responding to prop")
		}
		respondingToPost, getErr := c.pluginAPI.Post.GetPost(respondingToPostID)
		if getErr != nil {
			return fmt.Errorf("could not get post being responded to: %w", getErr)
		}

		// Create a context with the tool call callback already set
		contextWithCallback := c.contextBuilder.BuildLLMContextUserRequest(
			bot,
			user,
			channel,
			c.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.GetMMBot().UserId, channel)),
		)

		// Process the user request with the context that has the callback
		var processErr error
		result, processErr = c.ProcessUserRequestWithContext(bot, user, channel, respondingToPost, contextWithCallback)
		if processErr != nil {
			return fmt.Errorf("could not continue conversation on regen: %w", processErr)
		}
	}

	if mmapi.IsDMWith(bot.GetMMBot().UserId, channel) {
		if channel.Name == bot.GetMMBot().UserId+"__"+user.Id || channel.Name == user.Id+"__"+bot.GetMMBot().UserId {
			c.streamingService.StreamToPost(ctx, result, post, user.Locale)
			return nil
		}
	}

	config := c.pluginAPI.Configuration.GetConfig()
	c.streamingService.StreamToPost(ctx, result, post, *config.LocalizationSettings.DefaultServerLocale)

	return nil
}
