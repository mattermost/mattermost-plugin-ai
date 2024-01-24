package main

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/subtitles"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
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

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := p.selectEmoji(post, p.MakeConversationContext(user, channel, nil)); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

func (p *Plugin) handleSummarize(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	createdPost, err := p.startNewSummaryThread(post.Id, p.MakeConversationContext(user, channel, nil))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "Unable to produce summary"))
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

	createdPost, err := p.newCallRecordingThread(user, post, channel, fileID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := p.saveTitle(createdPost.Id, "Meeting Summary"); err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to save title"))
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

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "unable to get user"))
		return
	}

	targetPostUser, err := p.pluginAPI.User.Get(post.UserId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "unable to get calls user"))
		return
	}
	if !targetPostUser.IsBot || targetPostUser.Username != CallsBotUsername {
		c.AbortWithError(http.StatusBadRequest, errors.New("not a calls bot post"))
		return
	}

	createdPost, err := p.newCallTranscriptionSummaryThread(user, post, channel)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "unable to summarize transcription"))
		return
	}

	if err := p.saveTitle(createdPost.Id, "Meeting Summary"); err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to save title"))
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

func (p *Plugin) handleStop(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	p.streamingContextsMutex.Lock()
	defer p.streamingContextsMutex.Unlock()

	post := c.MustGet(ContextPostKey).(*model.Post)

	if post.UserId != p.botid {
		c.AbortWithError(http.StatusBadRequest, errors.New("not a bot post"))
		return
	}

	if post.GetProp(LLMRequesterUserID) != userID {
		c.AbortWithError(http.StatusForbidden, errors.New("only the original poster can stop the stream"))
		return
	}

	cancel, ok := p.streamingContexts[post.Id]
	if !ok {
		c.AbortWithError(http.StatusBadRequest, errors.New("context already canceled"))
		return
	}

	cancel()
}

func (p *Plugin) handleRegenerate(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	post := c.MustGet(ContextPostKey).(*model.Post)
	channel := c.MustGet(ContextChannelKey).(*model.Channel)

	if post.UserId != p.botid {
		c.AbortWithError(http.StatusBadRequest, errors.New("Not a AI bot post"))
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

	summaryPostIDProp := post.GetProp(ThreadIDProp)
	refrencedRecordingFileIDProp := post.GetProp(ReferencedRecordingFileID)
	referencedTranscriptPostProp := post.GetProp(ReferencedTranscriptPostID)
	var result *ai.TextStreamResult
	switch {
	case summaryPostIDProp != nil:
		summaryPostID := summaryPostIDProp.(string)
		siteURL := p.API.GetConfig().ServiceSettings.SiteURL
		post.Message = summaryPostMessage(summaryPostID, *siteURL)

		result, err = p.summarizePost(summaryPostID, p.MakeConversationContext(user, channel, nil))
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not summarize post on regen"))
			return
		}
	case refrencedRecordingFileIDProp != nil:
		post.Message = ""
		refrencedRecordingFileID := refrencedRecordingFileIDProp.(string)

		fileInfo, err := p.pluginAPI.File.GetInfo(refrencedRecordingFileID)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not get transcription file on regen"))
			return
		}

		reader, err := p.pluginAPI.File.Get(post.FileIds[0])
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not get transcription file on regen"))
			return
		}
		transcription, err := subtitles.NewSubtitlesFromVTT(reader)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not parse transcription file on regen"))
			return
		}

		if transcription.IsEmpty() {
			c.AbortWithError(http.StatusInternalServerError, errors.New("transcription is empty on regen"))
			return
		}

		originalFileChannel, err := p.pluginAPI.Channel.Get(fileInfo.ChannelId)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not get channel of original recording on regen"))
			return
		}

		context := p.MakeConversationContext(user, originalFileChannel, nil)
		result, err = p.summarizeTranscription(transcription, context)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not summarize transcription on regen"))
		}
	case referencedTranscriptPostProp != nil:
		post.Message = ""
		refrencedTranscriptionPostID := referencedTranscriptPostProp.(string)
		referencedTranscriptionPost, err := p.pluginAPI.Post.GetPost(refrencedTranscriptionPostID)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not get transcription post on regen"))
			return
		}

		transcriptionFileID, err := getCaptionsFileIDFromProps(referencedTranscriptionPost)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "unable to get transcription file id"))
			return
		}
		transcriptionFileReader, err := p.pluginAPI.File.Get(transcriptionFileID)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "unable to read calls file"))
			return
		}

		transcription, err := subtitles.NewSubtitlesFromVTT(transcriptionFileReader)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "unable to parse transcription file"))
			return
		}

		context := p.MakeConversationContext(user, channel, nil)
		result, err = p.summarizeTranscription(transcription, context)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "unable to summarize transcription"))
			return
		}

	default:
		post.Message = ""

		threadData, err := p.getThreadAndMeta(post.Id)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		respondingToPostID, ok := post.GetProp(RespondingToProp).(string)
		if !ok {
			threadData.cutoffBeforePostID(post.Id)
		} else {
			threadData.cutoffAtPostID(respondingToPostID)
		}
		postToRegenerate := threadData.latestPost()
		context := p.MakeConversationContext(user, channel, postToRegenerate)

		if result, err = p.continueConversation(threadData, context); err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not continue conversation on regen"))
			return
		}
	}

	p.streamResultToPost(result, post)
}
