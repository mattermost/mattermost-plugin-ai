package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/crspeller/mattermost-plugin-summarize/server/ai"
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
	router.POST("/feedback/post/:postid/positive", p.handlePositivePostFeedback)
	router.POST("/feedback/post/:postid/negative", p.handleNegativePostFeedback)
	router.GET("/feedback", p.handleGetFeedback)
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

func (p *Plugin) handlePositivePostFeedback(c *gin.Context) {
	p.handlePostFeedback(c, true)
}
func (p *Plugin) handleNegativePostFeedback(c *gin.Context) {
	p.handlePostFeedback(c, false)
}

func (p *Plugin) handlePostFeedback(c *gin.Context, positive bool) {
	postID := c.Param("postid")
	userID := c.GetHeader("Mattermost-User-Id")

	post, err := p.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	threadData, err := p.getThreadAndMeta(post.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	conversation := ai.ThreadToBotConversation(p.botid, threadData.Posts)

	serialized, err := json.Marshal(conversation)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "couldn't marshal json"))
		return
	}

	if _, err := p.execBuilder(p.builder.
		Insert("LLM_Feedback").
		SetMap(map[string]interface{}{
			"PostID":           postID,
			"UserID":           userID,
			"System":           "",
			"Prompt":           string(serialized),
			"Response":         post.Message,
			"PositiveFeedback": positive,
		})); err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "couldn't insert feedback"))
		return
	}

	c.Status(http.StatusOK)
}

func (p *Plugin) handleGetFeedback(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	if !p.pluginAPI.User.HasPermissionTo(userID, model.PermissionManageSystem) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var result []struct {
		PostID           string
		UserID           string
		System           string
		Prompt           string
		Response         string
		PositiveFeedback bool
	}
	if err := p.doQuery(&result, p.builder.
		Select("*").
		From("LLM_Feedback"),
	); err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to get feedback table"))
		return
	}

	c.IndentedJSON(http.StatusOK, result)
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
