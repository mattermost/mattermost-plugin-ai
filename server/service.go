package main

import (
	"fmt"

	"github.com/crspeller/mattermost-plugin-summarize/server/ai"
	"github.com/mattermost/mattermost-server/v6/model"
)

func (p *Plugin) processUserRequestToBot(post *model.Post, channel *model.Channel) error {
	if post.RootId == "" {
		return p.newConversation(post)
	}

	return p.continueConversation(post)
}

func (p *Plugin) newConversation(post *model.Post) error {
	conversation := ai.PostToBotConversation(p.botid, post)
	result, err := p.genericAnswerer.ContinueQuestionThread(conversation)
	if err != nil {
		return err
	}

	if err := p.streamResultToPost(result, post.ChannelId, post.Id); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) streamResultToPost(stream *ai.TextStreamResult, channelID string, rootID string) error {
	post := &model.Post{
		UserId:    p.botid,
		Message:   "",
		ChannelId: channelID,
		RootId:    rootID,
	}

	if err := p.pluginAPI.Post.CreatePost(post); err != nil {
		return err
	}

	go func() {
		for next := range stream.Stream {
			post.Message += next
			if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
				return
			}
		}
	}()

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
		conversation := ai.ThreadToBotConversation(p.botid, threadData.Posts)
		result, err = p.genericAnswerer.ContinueQuestionThread(conversation)
		if err != nil {
			return err
		}
	}

	if err := p.streamResultToPost(result, post.ChannelId, post.RootId); err != nil {
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

	conversation := ai.ThreadToBotConversation(p.botid, questionThreadData.Posts)

	result, err := p.threadAnswerer.ContinueThreadInterrogation(originalThread, conversation)
	if err != nil {
		return nil, err
	}

	return result, nil
}

const ThreadIDProp = "referenced_thread"

// DM the user with a standard message. Run the inferance
func (p *Plugin) startNewSummaryThread(rootID string, userID string) (string, error) {
	threadData, err := p.getThreadAndMeta(rootID)
	if err != nil {
		return "", err
	}

	formattedThread := formatThread(threadData)
	summary, err := p.summarizer.SummarizeThread(formattedThread)
	if err != nil {
		return "", err
	}

	post := &model.Post{
		Message: fmt.Sprintf("A summary of [this thread](/_redirect/pl/%s):\n```\n%s\n```", rootID, summary.ReadAll()),
	}
	post.AddProp(ThreadIDProp, rootID)

	if err := p.pluginAPI.Post.DM(p.botid, userID, post); err != nil {
		return "", err
	}

	return post.Id, nil
}
