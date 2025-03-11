// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"net/http"
	"slices"

	"errors"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/mattermost/mattermost-plugin-ai/server/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost-plugin-ai/server/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	postsPerPage = 60
	maxPosts     = 200
)

func (p *Plugin) getPostsByChannelBetween(channelID string, startTime, endTime int64) (*model.PostList, error) {
	// Find the ID of first post in our time range
	firstPostID, err := p.getFirstPostBeforeTimeRangeID(channelID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// Initialize result list
	result := &model.PostList{
		Posts: make(map[string]*model.Post),
		Order: []string{},
	}

	// Keep fetching previous pages until we either:
	// 1. Reach the endTime
	// 2. Hit the maxPosts limit
	// 3. Run out of posts
	totalPosts := 0
	page := 0

	for totalPosts < maxPosts {
		morePosts, err := p.pluginAPI.Post.GetPostsBefore(channelID, firstPostID, page, postsPerPage)
		if err != nil {
			return nil, err
		}

		if len(morePosts.Posts) == 0 {
			break // No more posts
		}

		// Add posts that fall within our time range
		for _, post := range morePosts.Posts {
			if post.CreateAt >= startTime && post.CreateAt <= endTime {
				result.Posts[post.Id] = post
				result.Order = append([]string{post.Id}, result.Order...) // Prepend ID to maintain chronological order
				totalPosts++
				if totalPosts >= maxPosts {
					break
				}
			}
			if post.CreateAt < startTime {
				break // We've gone too far back
			}
		}

		page++
	}

	return result, nil
}

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

	bot := c.MustGet(ContextBotKey).(*Bot)
	if err := p.checkUsageRestrictions(userID, bot, channel); err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}
}

func (p *Plugin) handleInterval(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	channel := c.MustGet(ContextChannelKey).(*model.Channel)
	bot := c.MustGet(ContextBotKey).(*Bot)

	if !p.licenseChecker.IsBasicsLicensed() {
		c.AbortWithError(http.StatusForbidden, enterprise.ErrNotLicensed)
		return
	}

	data := struct {
		StartTime    int64  `json:"start_time"`
		EndTime      int64  `json:"end_time"` // 0 means "until present"
		PresetPrompt string `json:"preset_prompt"`
		Prompt       string `json:"prompt"`
	}{}
	err := json.NewDecoder(c.Request.Body).Decode(&data)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	defer c.Request.Body.Close()

	// Validate time range
	if data.EndTime != 0 && data.StartTime >= data.EndTime {
		c.AbortWithError(http.StatusBadRequest, errors.New("start_time must be before end_time"))
		return
	}

	// Cap the date range at 14 days
	maxDuration := int64(14 * 24 * 60 * 60) // 14 days in seconds
	if data.EndTime != 0 && (data.EndTime-data.StartTime) > maxDuration {
		c.AbortWithError(http.StatusBadRequest, errors.New("date range cannot exceed 14 days"))
		return
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var posts *model.PostList
	if data.EndTime == 0 {
		posts, err = p.pluginAPI.Post.GetPostsSince(channel.Id, data.StartTime)
	} else {
		posts, err = p.getPostsByChannelBetween(channel.Id, data.StartTime, data.EndTime)
	}
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

	context := p.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
		p.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
	)
	context.Parameters = map[string]any{
		"Posts": formattedThread,
	}

	promptPreset := ""
	switch data.PresetPrompt {
	case "summarize":
		promptPreset = llm.PromptSummarizeChannelSinceSystem
	case "action_items":
		promptPreset = llm.PromptFindActionItemsSystem
	case "open_questions":
		promptPreset = llm.PromptFindOpenQuestionsSystem
	}

	if promptPreset == "" {
		c.AbortWithError(http.StatusBadRequest, errors.New("invalid preset prompt"))
		return
	}

	systemPrompt, err := p.prompts.Format(promptPreset, context)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	userPrompt, err := p.prompts.Format(llm.PromptThreadUser, context)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	completionRequest := llm.CompletionRequest{
		Posts: []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: systemPrompt,
			},
			{
				Role:    llm.PostRoleUser,
				Message: userPrompt,
			},
		},
		Context: context,
	}

	resultStream, err := p.getLLM(bot.cfg).ChatCompletion(completionRequest)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	post := &model.Post{}
	post.AddProp(NoRegen, "true")
	if err := p.streamResultToNewDM(bot.mmBot.UserId, resultStream, user.Id, post); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	promptTitle := ""
	switch data.PresetPrompt {
	case "summarize":
		if data.EndTime == 0 {
			promptTitle = "Summarize Unreads"
		} else {
			promptTitle = "Date Range Summary"
		}
	case "action_items":
		if data.EndTime == 0 {
			promptTitle = "Find Action Items"
		} else {
			promptTitle = "Date Range Action Items"
		}
	case "open_questions":
		if data.EndTime == 0 {
			promptTitle = "Find Open Questions"
		} else {
			promptTitle = "Date Range Open Questions"
		}
	}

	p.saveTitleAsync(post.Id, promptTitle)

	result := struct {
		PostID    string `json:"postid"`
		ChannelID string `json:"channelid"`
	}{
		PostID:    post.Id,
		ChannelID: post.ChannelId,
	}
	c.Render(http.StatusOK, render.JSON{Data: result})
}
