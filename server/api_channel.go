package main

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/enterprise"
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

func (p *Plugin) handleSince(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	channel := c.MustGet(ContextChannelKey).(*model.Channel)

	if !p.licenseChecker.IsBasicsLicenseed() {
		c.AbortWithError(http.StatusForbidden, enterprise.ErrNotLicensed)
		return
	}

	data := struct {
		Since        int64  `json:"since"`
		PresetPrompt string `json:"preset_prompt"`
		Prompt       string `json:"prompt"`
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

	posts, err := p.pluginAPI.Post.GetPostsSince(channel.Id, data.Since)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	threadData, err := p.getMetadataForPosts(posts)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Remove deleted posts
	threadData.Posts = slices.DeleteFunc(threadData.Posts, func(post *model.Post) bool {
		return post.DeleteAt != 0
	})

	formattedThread := formatThread(threadData)

	context := p.MakeConversationContext(user, channel, nil)
	context.PromptParameters = map[string]string{
		"Posts": formattedThread,
	}

	promptPreset := ""
	switch data.PresetPrompt {
	case "summarize":
		promptPreset = ai.PromptSummarizeChannelSince
	case "action_items":
		promptPreset = ai.PromptFindActionItemsSince
	case "open_questions":
		promptPreset = ai.PromptFindOpenQuestionsSince
	}

	if promptPreset == "" {
		c.AbortWithError(http.StatusBadRequest, errors.New("invalid preset prompt"))
		return
	}

	prompt, err := p.prompts.ChatCompletion(promptPreset, context)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	resultStream, err := p.getLLM().ChatCompletion(prompt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	post := &model.Post{}
	post.AddProp(NoRegen, "true")
	if err := p.streamResultToNewDM(resultStream, user.Id, post); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	promptTitle := ""
	switch data.PresetPrompt {
	case "summarize":
		promptTitle = "Summarize Unreads"
	case "action_items":
		promptTitle = "Find Action Items"
	case "open_questions":
		promptTitle = "Find Open Questions"
	}

	if err := p.saveTitle(post.Id, promptTitle); err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "failed to save title"))
		return
	}

	result := struct {
		PostID    string `json:"postid"`
		ChannelID string `json:"channelid"`
	}{
		PostID:    post.Id,
		ChannelID: post.ChannelId,
	}
	c.Render(http.StatusOK, render.JSON{Data: result})
}
