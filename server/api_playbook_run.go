package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

type PlaybookRun struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ChannelID   string `json:"channel_id"`
	StatusPosts []struct {
		ID string `json:"id"`
	} `json:"status_posts"`
	StatusUpdateTemplate string `json:"reminder_message_template"`
}

func (p *Plugin) playbookRunAuthorizationRequired(c *gin.Context) {
	playbookRunID := c.Param("playbookrunid")
	userID := c.GetHeader("Mattermost-User-Id")

	req, err := http.NewRequest("GET", fmt.Sprintf("/playbooks/api/v0/runs/%s", playbookRunID), nil)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "could not create request"))
		return
	}
	req.Header.Set("Mattermost-User-Id", userID)

	resp := p.pluginAPI.Plugin.HTTP(req)
	if resp == nil {
		c.AbortWithError(http.StatusInternalServerError, errors.New("failed to get playbook run, response was nil"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.AbortWithError(resp.StatusCode, errors.New("failed to get playbook run"))
		return
	}

	var playbookRun PlaybookRun
	err = json.NewDecoder(resp.Body).Decode(&playbookRun)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to decode response"))
		return
	}
	c.Set(ContextPlaybookRunKey, playbookRun)
}

func (p *Plugin) handleGenerateStatus(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	playbookRun := c.MustGet(ContextPlaybookRunKey).(PlaybookRun)
	channelID := playbookRun.ChannelID

	var generateRequest struct {
		Instructions []string `json:"instructions"`
		Messages     []string `json:"messages"`
		Bot          string   `json:"bot"`
	}

	if err := json.NewDecoder(c.Request.Body).Decode(&generateRequest); err != nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("You need to pass a list of instructions, it can be an empty list"))
		return
	}

	bot := p.GetBotByID(generateRequest.Bot)
	if bot == nil {
		bot = c.MustGet(ContextBotKey).(*Bot)
	}

	if !p.pluginAPI.User.HasPermissionToChannel(userID, channelID, model.PermissionReadChannel) {
		c.AbortWithError(http.StatusForbidden, errors.New("user doesn't have permission to read channel"))
		return
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	posts, err := p.pluginAPI.Post.GetPostsForChannel(channelID, 0, 100)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to get posts for channel"))
		return
	}

	postsData, err := p.getMetadataForPosts(posts)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to get metadata for posts"))
		return
	}
	// Remove deleted posts
	postsData.Posts = slices.DeleteFunc(postsData.Posts, func(post *model.Post) bool {
		return post.DeleteAt != 0
	})
	fomattedPosts := formatThread(postsData)

	ccontext := p.MakeConversationContext(bot, user, nil, nil)
	ccontext.PromptParameters = map[string]string{
		"Posts":            fomattedPosts,
		"Template":         playbookRun.StatusUpdateTemplate,
		"RunName":          playbookRun.Name,
		"Instructions":     strings.TrimSpace(strings.Join(generateRequest.Instructions, "\n")),
		"PreviousMessages": strings.TrimSpace(strings.Join(generateRequest.Messages, "\n-----\n")),
	}

	prompt, err := p.prompts.ChatCompletion("playbook_run_status", ccontext, llm.NewNoTools())
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to generate prompt"))
		return
	}

	resultStream, err := p.getLLM(bot.cfg).ChatCompletion(prompt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to get completion"))
		return
	}

	locale := *p.API.GetConfig().LocalizationSettings.DefaultServerLocale
	// Hack into current streaming solution. TODO: generalize this
	p.streamResultToPost(context.Background(), resultStream, &model.Post{
		ChannelId: channelID,
		Id:        "playbooks_post_update",
		Message:   "",
	}, locale)

	// result := resultStream.ReadAll()

	// c.JSON(http.StatusOK, result)
}
