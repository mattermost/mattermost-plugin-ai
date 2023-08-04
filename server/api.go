package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

const (
	ContextPostKey    = "post"
	ContextChannelKey = "channel"
)

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := gin.Default()
	router.Use(p.ginlogger)
	router.Use(p.MattermostAuthorizationRequired)

	postRouter := router.Group("/post/:postid")
	postRouter.Use(p.postAuthorizationRequired)
	postRouter.POST("/react", p.handleReact)
	postRouter.POST("/feedback/positive", p.handlePositivePostFeedback)
	postRouter.POST("/feedback/negative", p.handleNegativePostFeedback)
	postRouter.POST("/summarize", p.handleSummarize)
	postRouter.POST("/transcribe", p.handleTranscribe)

	textRouter := router.Group("/text")
	textRouter.POST("/simplify", p.handleSimplify)
	textRouter.POST("/change_tone/:tone", p.handleChangeTone)

	adminRouter := router.Group("/admin")
	adminRouter.GET("/feedback", p.handleGetFeedback)

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
