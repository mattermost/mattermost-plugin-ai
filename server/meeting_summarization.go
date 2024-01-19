package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	sq "github.com/Masterminds/squirrel"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/subtitles"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

const ReferencedRecordingPostID = "referenced_recording_post_id"
const ReferencedTranscriptPostID = "referenced_transcript_post_id"
const NoRegen = "no_regen"

func getCaptionsFileIDFromProps(post *model.Post) (fileID string, err error) {
	if post == nil {
		return "", errors.New("post is nil")
	}

	defer func() {
		if r := recover(); r != nil {
			err = errors.New("unable to parse captions on post")
		}
	}()

	captions, ok := post.GetProp("captions").([]interface{})
	if !ok || len(captions) == 0 {
		return "", errors.New("no captions on post")
	}

	// Calls will only ever have one for now.
	return captions[0].(map[string]interface{})["file_id"].(string), nil
}

func (p *Plugin) createTranscription(recordingFileID string) (*subtitles.Subtitles, error) {
	if p.ffmpegPath == "" {
		return nil, errors.New("ffmpeg not installed")
	}

	recordingFileInfo, err := p.pluginAPI.File.GetInfo(recordingFileID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get calls file info")
	}

	fileReader, err := p.pluginAPI.File.Get(recordingFileID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read calls file")
	}

	var cmd *exec.Cmd
	if recordingFileInfo.Size > WhisperAPILimit {
		cmd = exec.Command(p.ffmpegPath, "-i", "pipe:0", "-ac", "1", "-map", "0:a:0", "-b:a", "32k", "-ar", "16000", "-f", "mp3", "pipe:1")
	} else {
		cmd = exec.Command(p.ffmpegPath, "-i", "pipe:0", "-f", "mp3", "pipe:1")
	}

	cmd.Stdin = fileReader

	audioReader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create stdout pipe")
	}

	errorReader, err := cmd.StderrPipe()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create stderr pipe")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "couldn't run ffmpeg")
	}

	transcriber := p.getTranscribe()
	// Limit reader should probably error out instead of just silently failing
	transcription, err := transcriber.Transcribe(io.LimitReader(audioReader, WhisperAPILimit))
	if err != nil {
		return nil, errors.Wrap(err, "unable to transcribe")
	}

	errout, err := io.ReadAll(errorReader)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read stderr from ffmpeg")
	}

	if err := cmd.Wait(); err != nil {
		p.pluginAPI.Log.Debug("ffmpeg stderr: " + string(errout))
		return nil, errors.Wrap(err, "error while waiting for ffmpeg")
	}

	return transcription, nil
}

func (p *Plugin) newCallRecordingThread(requestingUser *model.User, recordingPost *model.Post, channel *model.Channel) (*model.Post, error) {
	if len(recordingPost.FileIds) != 1 {
		return nil, errors.New("Unexpected number of files in calls post")
	}

	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	surePost := &model.Post{
		Message: fmt.Sprintf("Sure, I will summarize this recording: %s/_redirect/pl/%s\n", *siteURL, recordingPost.Id),
	}
	surePost.AddProp(NoRegen, "true")
	if err := p.botDM(requestingUser.Id, surePost); err != nil {
		return nil, err
	}

	if err := p.summarizeCallRecording(surePost.Id, requestingUser, recordingPost, channel); err != nil {
		return nil, err
	}

	return surePost, nil
}

func (p *Plugin) newCallTranscriptionSummaryThread(requestingUser *model.User, transcriptionPost *model.Post, channel *model.Channel) (*model.Post, error) {
	if len(transcriptionPost.FileIds) != 1 {
		return nil, errors.New("Unexpected number of files in calls post")
	}

	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	surePost := &model.Post{
		Message: fmt.Sprintf("Sure, I will summarize this transcription: %s/_redirect/pl/%s\n", *siteURL, transcriptionPost.Id),
	}
	surePost.AddProp(NoRegen, "true")
	if err := p.botDM(requestingUser.Id, surePost); err != nil {
		return nil, err
	}

	transcriptionFileID, err := getCaptionsFileIDFromProps(transcriptionPost)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get transcription file id")
	}
	transcriptionFileReader, err := p.pluginAPI.File.Get(transcriptionFileID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read calls file")
	}

	transcription, err := subtitles.NewSubtitlesFromVTT(transcriptionFileReader)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse transcription file")
	}

	context := p.MakeConversationContext(requestingUser, channel, nil)
	summaryStream, err := p.summarizeTranscription(transcription, context)
	if err != nil {
		return nil, errors.Wrap(err, "unable to summarize transcription")
	}

	summaryPost := &model.Post{
		RootId:    surePost.Id,
		ChannelId: surePost.ChannelId,
		Message:   "",
	}
	summaryPost.AddProp(ReferencedTranscriptPostID, transcriptionPost.Id)
	if err := p.streamResultToNewPost(requestingUser.Id, summaryStream, summaryPost); err != nil {
		return nil, errors.Wrap(err, "unable to stream result to post")
	}

	return surePost, nil
}

func (p *Plugin) summarizeCallRecording(rootID string, requestingUser *model.User, recordingPost *model.Post, channel *model.Channel) error {
	transcriptPost := &model.Post{
		RootId:  rootID,
		Message: "Processing audio into transcription. This will take some time...",
	}
	transcriptPost.AddProp(ReferencedRecordingPostID, recordingPost.Id)
	if err := p.botDM(requestingUser.Id, transcriptPost); err != nil {
		return err
	}

	go func() (reterr error) {
		// Update to an error if we return one.
		defer func() {
			if reterr != nil {
				transcriptPost.Message = "Sorry! Somthing went wrong. Check the server logs for details."
				if err := p.pluginAPI.Post.UpdatePost(transcriptPost); err != nil {
					p.API.LogError("Failed to update post in error handling handleCallRecordingPost", "error", err)
				}
				p.API.LogError("Error in call recording post", "error", reterr)
			}
		}()

		transcription, err := p.createTranscription(recordingPost.FileIds[0])
		if err != nil {
			return errors.Wrap(err, "failed to create transcription")
		}

		transcriptFileInfo, err := p.pluginAPI.File.Upload(strings.NewReader(transcription.FormatVTT()), "transcript.txt", channel.Id)
		if err != nil {
			return errors.Wrap(err, "unable to upload transcript")
		}

		context := p.MakeConversationContext(requestingUser, channel, nil)
		summaryStream, err := p.summarizeTranscription(transcription, context)
		if err != nil {
			return errors.Wrap(err, "unable to summarize transcription")
		}

		if err := p.updatePostWithFile(transcriptPost, transcriptFileInfo); err != nil {
			return errors.Wrap(err, "unable to update transcript post")
		}

		if err := p.streamResultToPost(summaryStream, transcriptPost); err != nil {
			return errors.Wrap(err, "unable to stream result to post")
		}

		return nil
	}()

	return nil
}

func (p *Plugin) summarizeTranscription(transcription *subtitles.Subtitles, context ai.ConversationContext) (*ai.TextStreamResult, error) {
	llmFormattedTranscription := transcription.FormatForLLM()
	tokens := p.getLLM().CountTokens(llmFormattedTranscription)
	tokenLimitWithMargin := int(float64(p.getLLM().TokenLimit())*0.75) - ContextTokenMargin
	if tokenLimitWithMargin < 0 {
		tokenLimitWithMargin = ContextTokenMargin / 2
	}
	isChunked := false
	if tokens > tokenLimitWithMargin {
		p.pluginAPI.Log.Debug("Transcription too long, summarizing in chunks.", "tokens", tokens, "limit", tokenLimitWithMargin)
		chunks := splitPlaintextOnSentences(llmFormattedTranscription, tokenLimitWithMargin*4)
		summarizedChunks := make([]string, 0, len(chunks))
		p.pluginAPI.Log.Debug("Split into chunks", "chunks", len(chunks))
		for _, chunk := range chunks {
			context.PromptParameters = map[string]string{"TranscriptionChunk": chunk}
			summarizeChunkPrompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeChunk, context)
			if err != nil {
				return nil, errors.Wrap(err, "unable to get summarize chunk prompt")
			}

			summarizedChunk, err := p.getLLM().ChatCompletionNoStream(summarizeChunkPrompt)
			if err != nil {
				return nil, errors.Wrap(err, "unable to get summarized chunk")
			}

			summarizedChunks = append(summarizedChunks, summarizedChunk)
		}

		llmFormattedTranscription = strings.Join(summarizedChunks, "\n\n")
		isChunked = true
		p.pluginAPI.Log.Debug("Completed chunk summarization", "chunks", len(summarizedChunks), "tokens", p.getLLM().CountTokens(llmFormattedTranscription))
	}

	context.PromptParameters = map[string]string{"Transcription": llmFormattedTranscription, "IsChunked": fmt.Sprintf("%t", isChunked)}
	summaryPrompt, err := p.prompts.ChatCompletion(ai.PromptMeetingSummary, context)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get meeting summary prompt")
	}

	summaryStream, err := p.getLLM().ChatCompletion(summaryPrompt)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get meeting summary")
	}

	return summaryStream, nil
}

func (p *Plugin) updatePostWithFile(post *model.Post, fileinfo *model.FileInfo) error {
	if _, err := p.execBuilder(p.builder.
		Update("FileInfo").
		Set("PostId", post.Id).
		Set("ChannelId", post.ChannelId).
		Where(sq.And{
			sq.Eq{"Id": fileinfo.Id},
			sq.Eq{"PostId": ""},
		})); err != nil {
		return errors.Wrap(err, "unable to update file info")
	}

	post.FileIds = []string{fileinfo.Id}
	post.Message = ""
	if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
		return errors.Wrap(err, "unable to update post")
	}

	return nil
}
