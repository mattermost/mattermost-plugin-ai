package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/gin-gonic/gin"
)

func (p *Plugin) handlePostConversation(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	// We only allow bots to use this API handler for the time being.
	if _, err := p.pluginAPI.Bot.Get(userID, false); errors.Is(err, pluginapi.ErrNotFound) {
		c.AbortWithError(http.StatusForbidden, errors.New("forbidden"))
		return
	} else if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get bot: %w", err))
		return
	}

	var conv []*model.Post
	if err := json.NewDecoder(c.Request.Body).Decode(&conv); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	defer c.Request.Body.Close()
	if len(conv) == 0 {
		c.AbortWithError(http.StatusBadRequest, errors.New("invalid empty conversation"))
		return
	}

	// The last post is expected to contain the current request, everything else is used as context.
	post := conv[len(conv)-1]

	if post.ChannelId == "" {
		c.AbortWithError(http.StatusBadRequest, errors.New("invalid empty channel id"))
		return
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if errors.Is(err, pluginapi.ErrNotFound) {
		c.AbortWithError(http.StatusBadRequest, errors.New("channel not found"))
		return
	} else if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get channel: %w", err))
		return
	}

	if post.UserId == "" {
		c.AbortWithError(http.StatusBadRequest, errors.New("invalid empty user id"))
		return
	}

	postingUser, err := p.pluginAPI.User.Get(post.UserId)
	if errors.Is(err, pluginapi.ErrNotFound) {
		c.AbortWithError(http.StatusBadRequest, errors.New("user not found"))
		return
	} else if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get posting user: %w", err))
		return
	}

	// Don't respond to ourselves
	if p.IsAnyBot(post.UserId) {
		c.AbortWithError(http.StatusBadRequest, errors.New("not responding to ourselves"))
		return
	}

	bot := p.GetBotMentioned(post.Message)
	if bot == nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("missing bot mention"))
		return
	}

	list := &model.PostList{
		Order: make([]string, 0, len(conv)),
		Posts: make(map[string]*model.Post, len(conv)),
	}
	for i, post := range conv {
		list.Order = append(list.Order, post.Id)
		list.Posts[post.Id] = conv[i]
	}

	threadData, err := p.getMetadataForPosts(list)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get thread data: %w", err))
		return
	}

	result, err := p.continueConversation(bot, threadData, p.MakeConversationContext(bot, postingUser, channel, post))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to continue conversation: %w", err))
		return
	}

	for {
		select {
		case msg := <-result.Stream:
			if _, err := c.Writer.WriteString(msg); err != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error while writing result: %w", err))
			}
			// Flushing lets us stream partial results without requiring the client to wait for the full response.
			c.Writer.Flush()
		case err, ok := <-result.Err:
			if !ok {
				return
			}
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error while streaming result: %w", err))
			return
		}
	}
}
