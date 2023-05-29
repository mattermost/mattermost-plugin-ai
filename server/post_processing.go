package main

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-server/v6/model"
)

type ThreadData struct {
	Posts     []*model.Post
	UsersByID map[string]*model.User
}

func (p *Plugin) getThreadAndMeta(postID string) (*ThreadData, error) {
	posts, err := p.pluginAPI.Post.GetPostThread(postID)
	if err != nil {
		return nil, err
	}

	userIDsUnique := make(map[string]bool)
	for _, post := range posts.Posts {
		userIDsUnique[post.UserId] = true
	}
	userIDs := make([]string, 0, len(userIDsUnique))
	for userID := range userIDsUnique {
		userIDs = append(userIDs, userID)
	}

	usersByID := make(map[string]*model.User)
	for _, userID := range userIDs {
		user, err := p.pluginAPI.User.Get(userID)
		if err != nil {
			return nil, err
		}
		usersByID[userID] = user
	}

	postsSlice := posts.ToSlice()

	return &ThreadData{
		Posts:     postsSlice,
		UsersByID: usersByID,
	}, nil

}

func formatThread(data *ThreadData) string {
	result := ""
	for _, post := range data.Posts {
		result += fmt.Sprintf("%s: %s\n\n", data.UsersByID[post.UserId].Username, post.Message)
	}

	return result
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
		for {
			select {
			case next := <-stream.Stream:
				post.Message += next
				if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
					p.API.LogError("Streaming failed to update post", "error", err)
					return
				}
			case err, ok := <-stream.Err:
				if !ok {
					return
				}
				p.API.LogError("Streaming result to post failed", "error", err)
				post.Message = "Sorry! An error occoured while accessing the LLM. See server logs for details."
				if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
					p.API.LogError("Error recovering from streaming error", "error", err)
					return
				}
				return
			}
		}
	}()

	return nil
}
