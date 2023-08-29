package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

func (p *Plugin) channelAuthorizationRequired(c *gin.Context) {
	channelID := c.Param("channelid")
	userID := c.GetHeader("Mattermost-User-Id")

	channel, err := p.pluginAPI.Channel.Get(channelID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Set(ContextChannelKey, channel)

	if !p.pluginAPI.User.HasPermissionToChannel(userID, channel.Id, model.PermissionReadChannel) {
		c.AbortWithError(http.StatusForbidden, errors.New("user doesn't have permission to read channel"))
		return
	}

	if err := p.checkUsageRestrictions(userID, channel); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}
}

func (p *Plugin) handleSummarizeSince(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	channel := c.MustGet(ContextChannelKey).(*model.Channel)

	data := struct {
		Since int64 `json:"since"`
	}{}
	err := json.NewDecoder(c.Request.Body).Decode(&data)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	defer c.Request.Body.Close()

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	result, err := p.summarizeChannelSince(user, channel, data.Since)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Render(200, render.JSON{Data: struct {
		Message string `json:"message"`
	}{result}})
}
