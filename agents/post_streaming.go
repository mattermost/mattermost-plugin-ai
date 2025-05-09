// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

const PostStreamingControlCancel = "cancel"
const PostStreamingControlEnd = "end"
const PostStreamingControlStart = "start"

type PostStreamContext struct {
	cancel context.CancelFunc
}

func (p *AgentsService) streamResultToNewPost(botid string, requesterUserID string, stream *llm.TextStreamResult, post *model.Post, respondingToPostID string) error {
	// We use modifyPostForBot directly here to add the responding to post ID
	p.modifyPostForBot(botid, requesterUserID, post, respondingToPostID)

	if err := p.pluginAPI.Post.CreatePost(post); err != nil {
		return fmt.Errorf("unable to create post: %w", err)
	}

	// The callback is already set when creating the context

	ctx, err := p.getPostStreamingContext(context.Background(), post.Id)
	if err != nil {
		return err
	}

	go func() {
		defer p.finishPostStreaming(post.Id)
		user, err := p.pluginAPI.User.Get(requesterUserID)
		locale := *p.pluginAPI.Configuration.GetConfig().LocalizationSettings.DefaultServerLocale
		if err != nil {
			p.streamResultToPost(ctx, stream, post, locale)
			return
		}

		channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
		if err != nil {
			p.streamResultToPost(ctx, stream, post, locale)
			return
		}

		if channel.Type == model.ChannelTypeDirect {
			if channel.Name == botid+"__"+user.Id || channel.Name == user.Id+"__"+botid {
				p.streamResultToPost(ctx, stream, post, user.Locale)
				return
			}
		}
		p.streamResultToPost(ctx, stream, post, locale)
	}()

	return nil
}

func (p *AgentsService) streamResultToNewDM(botid string, stream *llm.TextStreamResult, userID string, post *model.Post, respondingToPostID string) error {
	// We use modifyPostForBot directly here to add the responding to post ID
	p.modifyPostForBot(botid, userID, post, respondingToPostID)

	if err := p.pluginAPI.Post.DM(botid, userID, post); err != nil {
		return fmt.Errorf("failed to post DM: %w", err)
	}

	// The callback is already set when creating the context

	ctx, err := p.getPostStreamingContext(context.Background(), post.Id)
	if err != nil {
		return err
	}

	go func() {
		defer p.finishPostStreaming(post.Id)
		user, err := p.pluginAPI.User.Get(userID)
		locale := *p.pluginAPI.Configuration.GetConfig().LocalizationSettings.DefaultServerLocale
		if err != nil {
			p.streamResultToPost(ctx, stream, post, locale)
			return
		}

		channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
		if err != nil {
			p.streamResultToPost(ctx, stream, post, locale)
			return
		}

		if channel.Type == model.ChannelTypeDirect {
			if channel.Name == botid+"__"+user.Id || channel.Name == user.Id+"__"+botid {
				p.streamResultToPost(ctx, stream, post, user.Locale)
				return
			}
		}
		p.streamResultToPost(ctx, stream, post, locale)
	}()

	return nil
}

func (p *AgentsService) sendPostStreamingUpdateEvent(post *model.Post, message string) {
	p.pluginAPI.Frontend.PublishWebSocketEvent("postupdate", map[string]interface{}{
		"post_id": post.Id,
		"next":    message,
	}, &model.WebsocketBroadcast{
		ChannelId: post.ChannelId,
	})
}

func (p *AgentsService) sendPostStreamingControlEvent(post *model.Post, control string) {
	p.pluginAPI.Frontend.PublishWebSocketEvent("postupdate", map[string]interface{}{
		"post_id": post.Id,
		"control": control,
	}, &model.WebsocketBroadcast{
		ChannelId: post.ChannelId,
	})
}

func (p *AgentsService) stopPostStreaming(postID string) {
	p.streamingContextsMutex.Lock()
	defer p.streamingContextsMutex.Unlock()
	if streamContext, ok := p.streamingContexts[postID]; ok {
		streamContext.cancel()
	}
	delete(p.streamingContexts, postID)
}

var ErrAlreadyStreamingToPost = fmt.Errorf("already streaming to post")

func (p *AgentsService) getPostStreamingContext(inCtx context.Context, postID string) (context.Context, error) {
	p.streamingContextsMutex.Lock()
	defer p.streamingContextsMutex.Unlock()

	if _, ok := p.streamingContexts[postID]; ok {
		return nil, ErrAlreadyStreamingToPost
	}

	ctx, cancel := context.WithCancel(inCtx)

	streamingContext := PostStreamContext{
		cancel: cancel,
	}

	p.streamingContexts[postID] = streamingContext

	return ctx, nil
}

// finishPostStreaming should be called when a post streaming operation is finished on success or failure.
// It is safe to call multiple times, must be called at least once.
func (p *AgentsService) finishPostStreaming(postID string) {
	p.streamingContextsMutex.Lock()
	defer p.streamingContextsMutex.Unlock()
	delete(p.streamingContexts, postID)
}

// streamResultToPost streams the result of a TextStreamResult to a post.
// it will internally handle logging needs and updating the post.
func (p *AgentsService) streamResultToPost(ctx context.Context, stream *llm.TextStreamResult, post *model.Post, userLocale string) {
	T := i18n.LocalizerFunc(p.i18n, userLocale)
	p.sendPostStreamingControlEvent(post, PostStreamingControlStart)
	defer func() {
		p.sendPostStreamingControlEvent(post, PostStreamingControlEnd)
	}()

	for {
		select {
		case event := <-stream.Stream:
			switch event.Type {
			case llm.EventTypeText:
				// Handle text event
				if textChunk, ok := event.Value.(string); ok {
					post.Message += textChunk
					p.sendPostStreamingUpdateEvent(post, post.Message)
				}
			case llm.EventTypeEnd:
				// Stream has closed cleanly
				if strings.TrimSpace(post.Message) == "" {
					p.pluginAPI.Log.Error("LLM closed stream with no result")
					post.Message = T("copilot.stream_to_post_llm_not_return", "Sorry! The LLM did not return a result.")
					p.sendPostStreamingUpdateEvent(post, post.Message)
				}
				if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
					p.pluginAPI.Log.Error("Streaming failed to update post", "error", err)
					return
				}
				return
			case llm.EventTypeError:
				// Handle error event
				var err error
				if errValue, ok := event.Value.(error); ok {
					err = errValue
				} else {
					err = fmt.Errorf("unknown error from LLM")
				}

				// Handle partial results
				if strings.TrimSpace(post.Message) == "" {
					post.Message = ""
				} else {
					post.Message += "\n\n"
				}
				p.pluginAPI.Log.Error("Streaming result to post failed partway", "error", err)
				post.Message = T("copilot.stream_to_post_access_llm_error", "Sorry! An error occurred while accessing the LLM. See server logs for details.")

				if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
					p.pluginAPI.Log.Error("Error recovering from streaming error", "error", err)
					return
				}
				p.sendPostStreamingUpdateEvent(post, post.Message)
				return
			case llm.EventTypeToolCalls:
				// Handle tool call event
				if toolCalls, ok := event.Value.([]llm.ToolCall); ok {
					// Ensure all tool calls have Pending status
					for i := range toolCalls {
						toolCalls[i].Status = llm.ToolCallStatusPending
					}

					// Add the tool call as a prop to the post
					toolCallJSON, err := json.Marshal(toolCalls)
					if err != nil {
						p.pluginAPI.Log.Error("Failed to marshal tool call", "error", err)
					} else {
						post.AddProp(ToolCallProp, string(toolCallJSON))
					}

					// Update the post with the tool call
					if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
						p.pluginAPI.Log.Error("Failed to update post with tool call", "error", err)
					}

					// Send websocket event with tool call data
					p.pluginAPI.Frontend.PublishWebSocketEvent("postupdate", map[string]interface{}{
						"post_id":   post.Id,
						"control":   "tool_call",
						"tool_call": string(toolCallJSON),
					}, &model.WebsocketBroadcast{
						ChannelId: post.ChannelId,
					})
				}
				return
			}
		case <-ctx.Done():
			if err := p.pluginAPI.Post.UpdatePost(post); err != nil {
				p.pluginAPI.Log.Error("Error updating post on stop signaled", "error", err)
				return
			}
			p.sendPostStreamingControlEvent(post, PostStreamingControlCancel)
			return
		}
	}
}

type WorkerResult struct {
	StreamNumber int
	Value        string
}
