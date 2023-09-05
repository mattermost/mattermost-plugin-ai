package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

type ThreadData struct {
	Posts     []*model.Post
	FilesByID map[string]*model.FileInfo
	UsersByID map[string]*model.User
}

func (p *Plugin) getThreadAndMeta(postID string) (*ThreadData, error) {
	posts, err := p.pluginAPI.Post.GetPostThread(postID)
	if err != nil {
		return nil, err
	}
	return p.getMetadataForPosts(posts)

}

func (p *Plugin) getMetadataForPosts(posts *model.PostList) (*ThreadData, error) {
	sort.Slice(posts.Order, func(i, j int) bool {
		return posts.Posts[posts.Order[i]].CreateAt < posts.Posts[posts.Order[j]].CreateAt
	})

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

	threadData := &ThreadData{
		Posts:     postsSlice,
		UsersByID: usersByID,
		FilesByID: make(map[string]*model.FileInfo),
	}

	for _, post := range postsSlice {
		if len(post.FileIds) > 0 {
			for _, fileID := range post.FileIds {
				fileInfo, err := p.pluginAPI.File.GetInfo(fileID)
				if err != nil {
					return nil, err
				}
				threadData.FilesByID[fileID] = fileInfo
			}
		}
	}

	return threadData, nil
}

func formatThread(data *ThreadData) string {
	result := strings.Builder{}
	for _, post := range data.Posts {
		result.WriteString(formatPost(data, post))
	}

	return result.String()
}

func formatPost(data *ThreadData, post *model.Post) string {
	result := strings.Builder{}

	result.WriteString(fmt.Sprintf("%s: %s\n", data.UsersByID[post.UserId].Username, post.Message))
	if len(post.FileIds) > 0 {
		for _, fileID := range post.FileIds {
			file := data.FilesByID[fileID]
			result.WriteString(fmt.Sprintf("File Attachment '%s': %s\n", file.Name, file.Content))
		}
	}
	result.WriteString("\n")

	return result.String()
}

func ThreadToBotConversation(botID string, threadData *ThreadData) ai.BotConversation {
	result := ai.BotConversation{
		Posts: make([]ai.Post, 0, len(threadData.Posts)),
	}

	for _, post := range threadData.Posts {
		result.Posts = append(result.Posts, ai.Post{
			Role:    ai.GetPostRole(botID, post),
			Message: formatPost(threadData, post),
		})
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
				post.Message = "Sorry! An error occurred while accessing the LLM. See server logs for details."
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

type WorkerResult struct {
	StreamNumber int
	Value        string
}

func (p *Plugin) multiStreamResultToPost(post *model.Post, messageTemplate []string, streams ...*ai.TextStreamResult) error {
	if len(messageTemplate) < 2*len(streams) {
		return errors.New("bad multi stream template")
	}

	results := make(chan WorkerResult)
	errors := make(chan error)

	// Create workers for recieving the text stream results.
	for i, stream := range streams {
		go func(streamNumber int, stream *ai.TextStreamResult) {
			for {
				select {
				case next := <-stream.Stream:
					results <- WorkerResult{
						StreamNumber: streamNumber,
						Value:        next,
					}
				case err, ok := <-stream.Err:
					if !ok {
						return
					}
					errors <- err
					return
				}
			}
		}(i, stream)
	}

	// Single post updating goroutine
	go func() {
		for {
			select {
			case next := <-results:
				// Update template
				messageTemplate[next.StreamNumber*2+1] += next.Value

				post.Message = strings.Join(messageTemplate, "")
				if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
					p.API.LogError("Streaming failed to update post", "error", err)
					return
				}
			case err, ok := <-errors:
				if !ok {
					return
				}
				p.API.LogError("Streaming result to post failed", "error", err)
				post.Message = "Sorry! An error occurred while accessing the LLM. See server logs for details."
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
