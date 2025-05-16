// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"context"
	"errors"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/llm/subtitles"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

// HandleRegenerate handles post regeneration requests
func (p *AgentsService) HandleRegenerate(userID string, post *model.Post, channel *model.Channel) error {
	bot := p.GetBotByID(post.UserId)
	if bot == nil {
		return fmt.Errorf("unable to get bot")
	}

	if post.GetProp(LLMRequesterUserID) != userID {
		return errors.New("only the original poster can regenerate")
	}

	if post.GetProp(NoRegen) != nil {
		return errors.New("tagged no regen")
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return fmt.Errorf("unable to get user to regen post: %w", err)
	}

	ctx, err := p.getPostStreamingContext(context.Background(), post.Id)
	if err != nil {
		return fmt.Errorf("unable to get post streaming context: %w", err)
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
		config := p.pluginAPI.Configuration.GetConfig()
		siteURL := config.ServiceSettings.SiteURL
		post.Message = p.analysisPostMessage(user.Locale, threadID, analysisType, *siteURL)

		var err error
		result, err = p.analyzeThread(bot, threadID, analysisType, p.contextBuilder.BuildLLMContextUserRequest(
			bot,
			user,
			channel,
			p.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
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

		context := p.contextBuilder.BuildLLMContextUserRequest(
			bot,
			user,
			originalFileChannel,
			p.contextBuilder.WithLLMContextDefaultTools(bot, originalFileChannel.Type == model.ChannelTypeDirect),
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

		context := p.contextBuilder.BuildLLMContextUserRequest(
			bot,
			user,
			channel,
			p.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
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
		contextWithCallback := p.contextBuilder.BuildLLMContextUserRequest(
			bot,
			user,
			channel,
			p.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
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

	config := p.pluginAPI.Configuration.GetConfig()
	p.streamResultToPost(ctx, result, post, *config.LocalizationSettings.DefaultServerLocale)

	return nil
}
