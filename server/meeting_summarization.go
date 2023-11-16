package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

func (p *Plugin) handleCallRecordingPost(recordingPost *model.Post, channel *model.Channel) (reterr error) {
	if len(recordingPost.FileIds) != 1 {
		return errors.New("Unexpected number of files in calls post")
	}

	if p.ffmpegPath == "" {
		return errors.New("ffmpeg not installed")
	}

	rootId := recordingPost.Id
	if recordingPost.RootId != "" {
		rootId = recordingPost.RootId
	}

	botPost := &model.Post{
		ChannelId: recordingPost.ChannelId,
		RootId:    rootId,
		Message:   "Transcribing meeting...",
	}
	if err := p.botCreatePost("", botPost); err != nil {
		return err
	}

	// Update to an error if we return one.
	defer func() {
		if reterr != nil {
			botPost.Message = "Sorry! Somthing went wrong. Check the server logs for details."
			if err := p.pluginAPI.Post.UpdatePost(botPost); err != nil {
				p.API.LogError("Failed to update post in error handling handleCallRecordingPost", "error", err)
			}
		}
	}()

	recordingFileID := recordingPost.FileIds[0]

	recordingFileInfo, err := p.pluginAPI.File.GetInfo(recordingFileID)
	if err != nil {
		return errors.Wrap(err, "unable to get calls file info")
	}

	fileReader, err := p.pluginAPI.File.Get(recordingFileID)
	if err != nil {
		return errors.Wrap(err, "unable to read calls file")
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
		return errors.Wrap(err, "couldn't create stdout pipe")
	}

	errorReader, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "couldn't create stderr pipe")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "couldn't run ffmpeg")
	}

	transcriber := p.getTranscribe()
	// Limit reader should probably error out instead of just silently failing
	transcription, err := transcriber.Transcribe(io.LimitReader(audioReader, WhisperAPILimit))
	if err != nil {
		return err
	}
	llmFormattedTranscription := transcription.FormatForLLM()

	errout, err := io.ReadAll(errorReader)
	if err != nil {
		return errors.Wrap(err, "unable to read stderr from ffmpeg")
	}

	if err := cmd.Wait(); err != nil {
		p.pluginAPI.Log.Debug("ffmpeg stderr: " + string(errout))
		return errors.Wrap(err, "error while waiting for ffmpeg")
	}

	transcriptFileInfo, err := p.pluginAPI.File.Upload(strings.NewReader(transcription.FormatTextOnly()), "transcript.txt", channel.Id)
	if err != nil {
		return errors.Wrap(err, "unable to upload transcript")
	}

	// Can not update a post to include file attachments. So we have to delete and re-create.
	if err := p.pluginAPI.Post.DeletePost(botPost.Id); err != nil {
		return errors.Wrap(err, "unable to delete bot post")
	}

	botPost.Id = ""
	botPost.CreateAt = 0
	botPost.UpdateAt = 0
	botPost.EditAt = 0
	botPost.Message += "\nRefining transcription..."
	botPost.FileIds = []string{transcriptFileInfo.Id}
	if err := p.botCreatePost("", botPost); err != nil {
		return err
	}

	tokens := p.getLLM().CountTokens(llmFormattedTranscription)
	isChunked := false
	if tokens > p.getLLM().TokenLimit()-ContextTokenMargin {
		p.pluginAPI.Log.Debug("Transcription too long, summarizing in chunks.", "tokens", tokens, "limit", p.getLLM().TokenLimit()-ContextTokenMargin)
		chunks := splitPlaintextOnSentences(llmFormattedTranscription, (p.getLLM().TokenLimit()-ContextTokenMargin)*4)
		summarizedChunks := make([]string, 0, len(chunks))
		p.pluginAPI.Log.Debug("Split into chunks", "chunks", len(chunks))
		for _, chunk := range chunks {
			context := p.MakeConversationContext(nil, channel, nil)
			context.PromptParameters = map[string]string{"TranscriptionChunk": chunk}
			summarizeChunkPrompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeChunk, context)
			if err != nil {
				return err
			}

			summarizedChunk, err := p.getLLM().ChatCompletionNoStream(summarizeChunkPrompt)
			if err != nil {
				return err
			}

			summarizedChunks = append(summarizedChunks, summarizedChunk)
		}

		llmFormattedTranscription = strings.Join(summarizedChunks, "\n\n")
		isChunked = true
	}

	context := p.MakeConversationContext(nil, channel, nil)
	context.PromptParameters = map[string]string{"Transcription": llmFormattedTranscription, "IsChunked": fmt.Sprintf("%t", isChunked)}
	summaryPrompt, err := p.prompts.ChatCompletion(ai.PromptMeetingSummaryOnly, context)
	if err != nil {
		return err
	}

	keyPointsPrompt, err := p.prompts.ChatCompletion(ai.PromptMeetingKeyPoints, context)
	if err != nil {
		return err
	}

	summaryStream, err := p.getLLM().ChatCompletion(summaryPrompt)
	if err != nil {
		return err
	}

	keyPointsStream, err := p.getLLM().ChatCompletion(keyPointsPrompt)
	if err != nil {
		return err
	}

	botPost.Message = ""
	template := []string{
		"# Meeting Summary\n",
		"",
		"\n## Key Discussion Points\n",
		"",
		"\n\n_Summary generated using AI, and may contain inaccuracies. Do not take this summary as absolute truth._",
	}
	if err := p.pluginAPI.Post.UpdatePost(botPost); err != nil {
		return err
	}

	if err := p.multiStreamResultToPost(botPost, template, summaryStream, keyPointsStream); err != nil {
		return err
	}

	return nil
}
