package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

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
