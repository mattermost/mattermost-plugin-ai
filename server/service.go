package main

import (
	"fmt"
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
