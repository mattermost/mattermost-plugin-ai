package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
)

func (p *Plugin) processUserRequestToBot(post *model.Post, channel *model.Channel) error {
	if post.RootId == "" {
		return p.newConversation(post)
	}

	return p.continueConversation(post)
}

func (p *Plugin) newConversation(post *model.Post) error {
	conversation, err := p.prompts.ChatCompletion(ai.PromptDirectMessageQuestion, nil)
	if err != nil {
		return err
	}
	conversation.AddUserPost(post)

	result, err := p.getLLM().ChatCompletion(conversation)
	if err != nil {
		return err
	}

	responsePost := &model.Post{
		ChannelId: post.ChannelId,
		RootId:    post.Id,
	}
	if err := p.streamResultToNewPost(result, responsePost); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) continueConversation(post *model.Post) error {
	threadData, err := p.getThreadAndMeta(post.RootId)
	if err != nil {
		return err
	}

	// Special handing for threads started by the bot in responce to a summarization request.
	var result *ai.TextStreamResult
	originalThreadID, ok := threadData.Posts[0].GetProp(ThreadIDProp).(string)
	if ok && originalThreadID != "" {
		threadPost, err := p.pluginAPI.Post.GetPost(originalThreadID)
		if err != nil {
			return err
		}
		threadChannel, err := p.pluginAPI.Channel.Get(threadPost.ChannelId)
		if err != nil {
			return err
		}

		if !p.pluginAPI.User.HasPermissionToChannel(post.UserId, threadChannel.Id, model.PermissionReadChannel) ||
			p.checkUsageRestrictions(post.UserId, threadChannel) != nil {
			responsePost := &model.Post{
				ChannelId: post.ChannelId,
				RootId:    post.RootId,
				Message:   "Sorry, you no longer have access to the original thread.",
			}
			if err := p.botCreatePost(responsePost); err != nil {
				return err
			}
			return nil
		}

		result, err = p.continueThreadConversation(threadData, originalThreadID)
		if err != nil {
			return err
		}
	} else {
		prompt, err := p.prompts.ChatCompletion(ai.PromptDirectMessageQuestion, nil)
		if err != nil {
			return err
		}
		prompt.AppendConversation(ai.ThreadToBotConversation(p.botid, threadData.Posts))

		result, err = p.getLLM().ChatCompletion(prompt)
		if err != nil {
			return err
		}
	}

	responsePost := &model.Post{
		ChannelId: post.ChannelId,
		RootId:    post.RootId,
	}
	if err := p.streamResultToNewPost(result, responsePost); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) continueThreadConversation(questionThreadData *ThreadData, originalThreadID string) (*ai.TextStreamResult, error) {
	originalThreadData, err := p.getThreadAndMeta(originalThreadID)
	if err != nil {
		return nil, err
	}
	originalThread := formatThread(originalThreadData)

	prompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeThread, map[string]string{"Thread": originalThread})
	if err != nil {
		return nil, err
	}
	prompt.AppendConversation(ai.ThreadToBotConversation(p.botid, questionThreadData.Posts))

	result, err := p.getLLM().ChatCompletion(prompt)
	if err != nil {
		return nil, err
	}

	return result, nil
}

const ThreadIDProp = "referenced_thread"

// DM the user with a standard message. Run the inferance
func (p *Plugin) startNewSummaryThread(postID string, userID string) (string, error) {
	threadData, err := p.getThreadAndMeta(postID)
	if err != nil {
		return "", err
	}

	formattedThread := formatThread(threadData)

	prompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeThread, map[string]string{"Thread": formattedThread})
	if err != nil {
		return "", err
	}
	summaryStream, err := p.getLLM().ChatCompletion(prompt)
	if err != nil {
		return "", err
	}

	post := &model.Post{
		Message: fmt.Sprintf("A summary of [this thread](/_redirect/pl/%s):\n", postID),
	}
	post.AddProp(ThreadIDProp, postID)

	if err := p.streamResultToNewDM(summaryStream, userID, post); err != nil {
		return "", err
	}

	return post.Id, nil
}

func (p *Plugin) selectEmoji(post *model.Post) error {
	prompt, err := p.prompts.ChatCompletion(ai.PromptEmojiSelect, map[string]string{"Message": post.Message})
	if err != nil {
		return err
	}

	emojiName, err := p.getLLM().ChatCompletionNoStream(prompt, ai.WithmaxTokens(25))
	if err != nil {
		return err
	}

	// Do some emoji post processing to hopfully make this an actual emoji.
	emojiName = strings.Trim(strings.TrimSpace(emojiName), ":")

	if _, found := model.GetSystemEmojiId(emojiName); !found {
		p.pluginAPI.Post.AddReaction(&model.Reaction{
			EmojiName: "large_red_square",
			UserId:    p.botid,
			PostId:    post.Id,
		})
		return errors.New("LLM returned somthing other than emoji: " + emojiName)
	}

	if err := p.pluginAPI.Post.AddReaction(&model.Reaction{
		EmojiName: emojiName,
		UserId:    p.botid,
		PostId:    post.Id,
	}); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) handleCallRecordingPost(recordingPost *model.Post) (err error) {
	if len(recordingPost.FileIds) != 1 {
		return errors.New("Unexpected number of files in calls post")
	}

	if !p.haveFFMpeg {
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
	if err := p.botCreatePost(botPost); err != nil {
		return err
	}

	// Update to an error if we return one.
	defer func() {
		if err != nil {
			botPost.Message = "Sorry! Somthing went wrong. Check the server logs for details."
			if err := p.pluginAPI.Post.UpdatePost(botPost); err != nil {
				p.API.LogError("Failed to update post in error handling handleCallRecordingPost", "error", err)
			}
		}
	}()

	fileID := recordingPost.FileIds[0]
	fileReader, err := p.pluginAPI.File.Get(fileID)
	if err != nil {
		return errors.Wrap(err, "unable to read calls file")
	}

	cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "mp3", "pipe:1")
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
	transcription, err := transcriber.Transcribe(audioReader)
	if err != nil {
		return err
	}

	errout, err := io.ReadAll(errorReader)
	if err != nil {
		return errors.Wrap(err, "unable to read stderr from ffmpeg")
	}

	if err := cmd.Wait(); err != nil {
		p.pluginAPI.Log.Debug("ffmpeg stderr: " + string(errout))
		return errors.Wrap(err, "error while waiting for ffmpeg")
	}

	botPost.Message += "\nRefining transcription..."
	if err := p.pluginAPI.Post.UpdatePost(botPost); err != nil {
		return err
	}

	summaryPrompt, err := p.prompts.ChatCompletion(ai.PromptMeetingSummaryOnly, map[string]string{"Transcription": transcription})
	if err != nil {
		return err
	}

	keyPointsPrompt, err := p.prompts.ChatCompletion(ai.PromptMeetingKeyPoints, map[string]string{"Transcription": transcription})
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
	}
	if err := p.pluginAPI.Post.UpdatePost(botPost); err != nil {
		return err
	}

	if err := p.multiStreamResultToPost(botPost, template, summaryStream, keyPointsStream); err != nil {
		return err
	}

	return nil
}
