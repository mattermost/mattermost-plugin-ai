package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

func (p *Plugin) mattermostAdminAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")

	if !p.pluginAPI.User.HasPermissionTo(userID, model.PermissionManageSystem) {
		c.AbortWithError(http.StatusForbidden, errors.New("must be a system admin"))
		return
	}
}

func (p *Plugin) handleGetFeedback(c *gin.Context) {
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
