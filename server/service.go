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

	responsePost := &model.Post{
		ChannelId: post.ChannelId,
		RootId:    post.Id,
	}
	if err := p.streamResultToNewPost(result, responsePost); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) modifyPostForBot(post *model.Post) {
	post.UserId = p.botid
	post.Type = "custom_llmbot"
}

func (p *Plugin) botCreatePost(post *model.Post) error {
	p.modifyPostForBot(post)

	if err := p.pluginAPI.Post.CreatePost(post); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) botDM(userID string, post *model.Post) error {
	p.modifyPostForBot(post)

	if err := p.pluginAPI.Post.DM(p.botid, userID, post); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) streamResultToNewPost(stream *ai.TextStreamResult, post *model.Post) error {
	if err := p.botCreatePost(post); err != nil {
		return err
	}

	if err := p.streamResultToPost(stream, post); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) streamResultToNewDM(stream *ai.TextStreamResult, userID string, post *model.Post) error {
	if err := p.botDM(userID, post); err != nil {
		return err
	}

	if err := p.streamResultToPost(stream, post); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) streamResultToPost(stream *ai.TextStreamResult, post *model.Post) error {
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

	conversation := ai.ThreadToBotConversation(p.botid, questionThreadData.Posts)

	result, err := p.threadAnswerer.ContinueThreadInterrogation(originalThread, conversation)
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
	summaryStream, err := p.summarizer.SummarizeThread(formattedThread)
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
