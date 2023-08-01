package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
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
	router.POST("/summarize/post/:postid", p.handleSummarize)
	router.POST("/transcribe/:postid", p.handleTranscribe)
	router.POST("/simplify", p.handleSimplify)
	router.POST("/ask_ai_change_text", p.handleAiChangeText)
	router.POST("/change_tone/:tone", p.handleChangeTone)
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

	if p.getConfiguration().EnableUseRestrictions {
		if !p.pluginAPI.User.HasPermissionToTeam(userID, p.getConfiguration().OnlyUsersOnTeam, model.PermissionViewTeam) {
			c.AbortWithError(http.StatusForbidden, errors.New("user not on allowed team"))
			return
		}
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

	_, err := p.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if _, err := p.execBuilder(p.builder.
		Insert("LLM_Feedback").
		SetMap(map[string]interface{}{
			"PostID":           postID,
			"UserID":           userID,
			"PositiveFeedback": positive,
		}).
		Suffix("ON CONFLICT (PostID) DO UPDATE SET PositiveFeedback = ?", positive)); err != nil {
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
		PositiveFeedback bool
	}
	if err := p.doQuery(&result, p.builder.
		Select("*").
		From("LLM_Feedback"),
	); err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to get feedback table"))
		return
	}

	totals := make(map[string]int)
	for _, entry := range result {
		if entry.PositiveFeedback {
			totals[entry.PostID] += 1
		} else {
			totals[entry.PostID] -= 1
		}
	}

	var output []struct {
		Conversation ai.BotConversation
		PostID       string
		Sentimant    int
	}

	for postID, total := range totals {
		thread, err := p.getThreadAndMeta(postID)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		conversation := ai.ThreadToBotConversation(p.botid, thread.Posts)

		output = append(output, struct {
			Conversation ai.BotConversation
			PostID       string
			Sentimant    int
		}{
			Conversation: conversation,
			PostID:       postID,
			Sentimant:    total,
		})
	}

	c.IndentedJSON(http.StatusOK, output)
}

func (p *Plugin) handleReact(c *gin.Context) {
	postID := c.Param("postid")
	userID := c.GetHeader("Mattermost-User-Id")

	post, err := p.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := p.checkUsageRestrictions(userID, channel); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	if err := p.selectEmoji(post, p.MakeConversationContext(user, channel, nil)); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

func (p *Plugin) handleSummarize(c *gin.Context) {
	postID := c.Param("postid")
	userID := c.GetHeader("Mattermost-User-Id")

	post, err := p.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := p.checkUsageRestrictions(userID, channel); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	if _, err := p.startNewSummaryThread(postID, p.MakeConversationContext(user, channel, nil)); err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "Unable to produce summary"))
		return
	}

	c.Status(http.StatusOK)
}

func (p *Plugin) handleTranscribe(c *gin.Context) {
	postID := c.Param("postid")
	userID := c.GetHeader("Mattermost-User-Id")

	post, err := p.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := p.checkUsageRestrictions(userID, channel); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	if err := p.handleCallRecordingPost(post, channel); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}

func (p *Plugin) handleSimplify(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	data := struct {
		Message string `json:"message"`
	}{}

	err := json.NewDecoder(c.Request.Body).Decode(&data)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer c.Request.Body.Close()

	if err := p.checkUsageRestrictionsForUser(userID); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	newMessage, err := p.simplifyText(data.Message)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	data.Message = *newMessage
	c.Render(200, render.JSON{Data: data})
}

func (p *Plugin) handleChangeTone(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	tone := c.Param("tone")

	data := struct {
		Message string `json:"message"`
	}{}

	err := json.NewDecoder(c.Request.Body).Decode(&data)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer c.Request.Body.Close()

	if err := p.checkUsageRestrictionsForUser(userID); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	newMessage, err := p.changeTone(tone, data.Message)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	data.Message = *newMessage
	c.Render(200, render.JSON{Data: data})
}

func (p *Plugin) handleAiChangeText(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	data := struct {
		Message string `json:"message"`
		Ask     string `json:"ask"`
	}{}

	err := json.NewDecoder(c.Request.Body).Decode(&data)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer c.Request.Body.Close()

	if err := p.checkUsageRestrictionsForUser(userID); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	newMessage, err := p.aiChangeText(data.Ask, data.Message)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	data.Message = *newMessage
	c.Render(200, render.JSON{Data: data})
}
