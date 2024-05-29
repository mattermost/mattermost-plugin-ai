package main

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"errors"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/subtitles"
	"github.com/mattermost/mattermost-plugin-ai/server/enterprise"
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

	if err := p.checkUsageRestrictions(userID, channel); err != nil {
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

	context := p.MakeConversationContext(bot, user, channel, post)
	context.PromptParameters = map[string]string{"Message": post.Message}
	prompt, err := p.prompts.ChatCompletion(ai.PromptEmojiSelect, context)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	emojiName, err := p.getLLM(bot.cfg.Service).ChatCompletionNoStream(prompt, ai.WithMaxGeneratedTokens(25))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Do some emoji post processing to hopefully make this an actual emoji.
	emojiName = strings.Trim(strings.TrimSpace(emojiName), ":")

	if _, found := model.GetSystemEmojiId(emojiName); !found {
		p.pluginAPI.Post.AddReaction(&model.Reaction{
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

func (p *Plugin) handleSummarize(c *gin.Context) {
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

	createdPost, err := p.startNewSummaryThread(bot, post.Id, p.MakeConversationContext(bot, user, channel, nil))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("unable to produce summary: %w", err))
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
	if !targetPostUser.IsBot || targetPostUser.Username != CallsBotUsername {
		c.AbortWithError(http.StatusBadRequest, errors.New("not a calls bot post"))
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
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := p.regeneratePost(bot, post, user, channel); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}

func (p *Plugin) regeneratePost(bot *Bot, post *model.Post, user *model.User, channel *model.Channel) error {
	ctx, err := p.getPostStreamingContext(context.Background(), post.Id)
	if err != nil {
		return err
	}
	defer p.finishPostStreaming(post.Id)

	summaryPostIDProp := post.GetProp(ThreadIDProp)
	refrencedRecordingFileIDProp := post.GetProp(ReferencedRecordingFileID)
	referencedTranscriptPostProp := post.GetProp(ReferencedTranscriptPostID)
	var result *ai.TextStreamResult
	switch {
	case summaryPostIDProp != nil:
		summaryPostID := summaryPostIDProp.(string)
		siteURL := p.API.GetConfig().ServiceSettings.SiteURL
		post.Message = p.summaryPostMessage(user.Locale, summaryPostID, *siteURL)

		var err error
		result, err = p.summarizePost(bot, summaryPostID, p.MakeConversationContext(bot, user, channel, nil))
		if err != nil {
			return fmt.Errorf("could not summarize post on regen: %w", err)
		}
	case refrencedRecordingFileIDProp != nil:
		post.Message = ""
		refrencedRecordingFileID := refrencedRecordingFileIDProp.(string)

		fileInfo, err := p.pluginAPI.File.GetInfo(refrencedRecordingFileID)
		if err != nil {
			return fmt.Errorf("could not get transcription file on regen: %w", err)
		}

		reader, err := p.pluginAPI.File.Get(post.FileIds[0])
		if err != nil {
			return fmt.Errorf("could not get transcription file on regen: %w", err)
		}
		transcription, err := subtitles.NewSubtitlesFromVTT(reader)
		if err != nil {
			return fmt.Errorf("could not parse transcription file on regen: %w", err)
		}

		if transcription.IsEmpty() {
			return errors.New("transcription is empty on regen")
		}

		originalFileChannel, err := p.pluginAPI.Channel.Get(fileInfo.ChannelId)
		if err != nil {
			return fmt.Errorf("could not get channel of original recording on regen: %w", err)
		}

		context := p.MakeConversationContext(bot, user, originalFileChannel, nil)
		result, err = p.summarizeTranscription(bot, transcription, context)
		if err != nil {
			return fmt.Errorf("could not summarize transcription on regen: %w", err)
		}
	case referencedTranscriptPostProp != nil:
		post.Message = ""
		refrencedTranscriptionPostID := referencedTranscriptPostProp.(string)
		referencedTranscriptionPost, err := p.pluginAPI.Post.GetPost(refrencedTranscriptionPostID)
		if err != nil {
			return fmt.Errorf("could not get transcription post on regen: %w", err)
		}

		transcriptionFileID, err := getCaptionsFileIDFromProps(referencedTranscriptionPost)
		if err != nil {
			return fmt.Errorf("unable to get transcription file id: %w", err)
		}
		transcriptionFileReader, err := p.pluginAPI.File.Get(transcriptionFileID)
		if err != nil {
			return fmt.Errorf("unable to read calls file: %w", err)
		}

		transcription, err := subtitles.NewSubtitlesFromVTT(transcriptionFileReader)
		if err != nil {
			return fmt.Errorf("unable to parse transcription file: %w", err)
		}

		context := p.MakeConversationContext(bot, user, channel, nil)
		result, err = p.summarizeTranscription(bot, transcription, context)
		if err != nil {
			return fmt.Errorf("unable to summarize transcription: %w", err)
		}

	default:
		post.Message = ""

		threadData, err := p.getThreadAndMeta(post.Id)
		if err != nil {
			return err
		}
		respondingToPostID, ok := post.GetProp(RespondingToProp).(string)
		if !ok {
			threadData.cutoffBeforePostID(post.Id)
		} else {
			threadData.cutoffAtPostID(respondingToPostID)
		}
		postToRegenerate := threadData.latestPost()
		context := p.MakeConversationContext(bot, user, channel, postToRegenerate)

		if result, err = p.continueConversation(bot, threadData, context); err != nil {
			return fmt.Errorf("could not continue conversation on regen: %w", err)
		}
	}

	p.streamResultToPost(ctx, result, post)

	return nil
}
