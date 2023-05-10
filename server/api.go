package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := gin.Default()
	router.Use(p.ginlogger)
	router.Use(p.MattermostAuthorizationRequired)
	router.POST("/react/:postid", p.handleReact)
	router.ServeHTTP(w, r)
}

func (p *Plugin) ginlogger(c *gin.Context) {
	c.Next()

	for _, ginErr := range c.Errors {
		p.API.LogError(ginErr.Error())
	}
}

func (p *Plugin) MattermostAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if !strings.Contains(p.getConfiguration().AllowedUserIDs, userID) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
}

func (p *Plugin) handleReact(c *gin.Context) {
	postID := c.Param("postid")

	post, err := p.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if !p.getConfiguration().AllowPrivateChannels {
		channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if channel.Type != model.ChannelTypeOpen {
			c.AbortWithError(http.StatusUnauthorized, errors.New("Can't operate on private channels."))
			return
		}

		if !strings.Contains(p.getConfiguration().AllowedTeamIDs, channel.TeamId) {
			c.AbortWithError(http.StatusUnauthorized, errors.New("Can't operate on this team."))
			return
		}
	}

	emojiName, err := p.emojiSelector.SelectEmoji(post.Message)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if _, found := model.GetSystemEmojiId(emojiName); !found {
		p.pluginAPI.Post.AddReaction(&model.Reaction{
			EmojiName: "large_red_square",
			UserId:    p.botid,
			PostId:    post.Id,
		})
		c.AbortWithError(http.StatusInternalServerError, errors.New("LLM returned somthing other than emoji: "+emojiName))
		return
	}

	p.pluginAPI.Post.AddReaction(&model.Reaction{
		EmojiName: emojiName,
		UserId:    p.botid,
		PostId:    post.Id,
	})

	c.Status(http.StatusOK)
}
