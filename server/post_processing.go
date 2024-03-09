package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

type ThreadData struct {
	Posts     []*model.Post
	UsersByID map[string]*model.User
}

func (t *ThreadData) cutoffBeforePostID(postID string) {
	// Iterate in reverse because it's more likely that the post we are responding to is near the end.
	for i := len(t.Posts) - 1; i >= 0; i-- {
		post := t.Posts[i]
		if post.Id == postID {
			t.Posts = t.Posts[:i]
			break
		}
	}
}

func (t *ThreadData) cutoffAtPostID(postID string) {
	// Iterate in reverse because it's more likely that the post we are responding to is near the end.
	for i := len(t.Posts) - 1; i >= 0; i-- {
		post := t.Posts[i]
		if post.Id == postID {
			t.Posts = t.Posts[:i+1]
			break
		}
	}
}

func (t *ThreadData) latestPost() *model.Post {
	if len(t.Posts) == 0 {
		return nil
	}
	return t.Posts[len(t.Posts)-1]
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

const LLMRequesterUserID = "llm_requester_user_id"
const UnsafeLinksPostProp = "unsafe_links"

func (p *Plugin) modifyPostForBot(requesterUserID string, post *model.Post) {
	post.UserId = p.botid
	post.Type = "custom_llmbot" // This must be the only place we add this type for security.
	post.AddProp(LLMRequesterUserID, requesterUserID)
	// This tags that the post has unsafe links since they could have been generted by a prompt injection.
	// This will prevent the server from making OpenGraph requests and markdown images being rendered.
	post.AddProp(UnsafeLinksPostProp, "true")
}

func (p *Plugin) botCreatePost(requesterUserID string, post *model.Post) error {
	p.modifyPostForBot(requesterUserID, post)

	if err := p.pluginAPI.Post.CreatePost(post); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) botDM(userID string, post *model.Post) error {
	p.modifyPostForBot(userID, post)

	if err := p.pluginAPI.Post.DM(p.botid, userID, post); err != nil {
		return fmt.Errorf("failed to post DM: %w", err)
	}

	return nil
}

func (p *Plugin) streamResultToNewPost(requesterUserID string, stream *ai.TextStreamResult, post *model.Post) error {
	if err := p.botCreatePost(requesterUserID, post); err != nil {
		return fmt.Errorf("unable to create post: %w", err)
	}

	if err := p.streamResultToPost(stream, post); err != nil {
		return fmt.Errorf("unable to stream result to post: %w", err)
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

func (p *Plugin) sendPostStreamingUpdateEvent(post *model.Post, message string) {
	p.API.PublishWebSocketEvent("postupdate", map[string]interface{}{
		"post_id": post.Id,
		"next":    message,
	}, &model.WebsocketBroadcast{
		ChannelId: post.ChannelId,
	})
}

const PostStreamingControlCancel = "cancel"
const PostStreamingControlEnd = "end"
const PostStreamingControlStart = "start"

func (p *Plugin) sendPostStreamingControlEvent(post *model.Post, control string) {
	p.API.PublishWebSocketEvent("postupdate", map[string]interface{}{
		"post_id": post.Id,
		"control": control,
	}, &model.WebsocketBroadcast{
		ChannelId: post.ChannelId,
	})
}

func (p *Plugin) streamResultToPost(stream *ai.TextStreamResult, post *model.Post) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.streamingContextsMutex.Lock()
	p.streamingContexts[post.Id] = cancel
	p.streamingContextsMutex.Unlock()
	go func() {
		defer func() {
			p.streamingContextsMutex.Lock()
			delete(p.streamingContexts, post.Id)
			p.streamingContextsMutex.Unlock()
		}()

		p.sendPostStreamingControlEvent(post, PostStreamingControlStart)
		defer func() {
			p.sendPostStreamingControlEvent(post, PostStreamingControlEnd)
		}()

		for {
			select {
			case next := <-stream.Stream:
				post.Message += next
				p.sendPostStreamingUpdateEvent(post, post.Message)
			case err, ok := <-stream.Err:
				// Stream has closed cleanly
				if !ok {
					if strings.TrimSpace(post.Message) == "" {
						p.API.LogError("LLM closed stream with no result")
						post.Message = "Sorry! The LLM did not return a result."
						p.sendPostStreamingUpdateEvent(post, post.Message)
					}
					if err = p.pluginAPI.Post.UpdatePost(post); err != nil {
						p.API.LogError("Streaming failed to update post", "error", err)
						return
					}
					return
				}
				// Handle partial results
				if strings.TrimSpace(post.Message) == "" {
					p.API.LogError("Streaming result to post failed", "error", err)
					post.Message = "Sorry! An error occurred while accessing the LLM. See server logs for details."
				} else {
					p.API.LogError("Streaming result to post failed partway", "error", err)
					post.Message += "\n\nSorry! An error occurred while streaming from the LLM. See server logs for details."
				}
				if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
					p.API.LogError("Error recovering from streaming error", "error", err)
					return
				}
				p.sendPostStreamingUpdateEvent(post, post.Message)
				return
			case <-ctx.Done():
				if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
					p.API.LogError("Error updating post on stop signaled", "error", err)
					return
				}
				p.sendPostStreamingControlEvent(post, PostStreamingControlCancel)
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

/*func (p *Plugin) multiStreamResultToPost(post *model.Post, messageTemplate []string, streams ...*ai.TextStreamResult) error {
	if len(messageTemplate) < 2*len(streams) {
		return errors.New("bad multi stream template")
	}

	results := make(chan WorkerResult)
	errors := make(chan error)

	// Create workers for receiving the text stream results.
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
}*/
