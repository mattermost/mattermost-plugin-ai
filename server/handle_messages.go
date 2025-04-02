// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/embeddings"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	ActivateAIProp  = "activate_ai"
	FromWebhookProp = "from_webhook"
	FromBotProp     = "from_bot"
	FromPluginProp  = "from_plugin"
	WranglerProp    = "wrangler"
)

var (
	// ErrNoResponse is returned when no response is posted under a normal condition.
	ErrNoResponse = errors.New("no response")
)

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	// Index the new message in the vector database
	if err := p.indexPost(post); err != nil {
		p.pluginAPI.Log.Error("Failed to index post in vector database", "error", err)
	}

	if err := p.handleMessages(post); err != nil {
		if errors.Is(err, ErrNoResponse) {
			p.pluginAPI.Log.Debug(err.Error())
		} else {
			p.pluginAPI.Log.Error(err.Error())
		}
	}
}

func (p *Plugin) handleMessages(post *model.Post) error {
	// Don't respond to ourselves
	if p.IsAnyBot(post.UserId) {
		return fmt.Errorf("not responding to ourselves: %w", ErrNoResponse)
	}

	// Never respond to remote posts
	if post.RemoteId != nil && *post.RemoteId != "" {
		return fmt.Errorf("not responding to remote posts: %w", ErrNoResponse)
	}

	// Wrangler posts should be ignored
	if post.GetProp(WranglerProp) != nil {
		return fmt.Errorf("not responding to wrangler posts: %w", ErrNoResponse)
	}

	// Don't respond to plugins unless they ask for it
	if post.GetProp(FromPluginProp) != nil && post.GetProp(ActivateAIProp) == nil {
		return fmt.Errorf("not responding to plugin posts: %w", ErrNoResponse)
	}

	// Don't respond to webhooks
	if post.GetProp(FromWebhookProp) != nil {
		return fmt.Errorf("not responding to webhook posts: %w", ErrNoResponse)
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		return fmt.Errorf("unable to get channel: %w", err)
	}

	postingUser, err := p.pluginAPI.User.Get(post.UserId)
	if err != nil {
		return err
	}

	// Don't respond to other bots unless they ask for it
	if (postingUser.IsBot || post.GetProp(FromBotProp) != nil) && post.GetProp(ActivateAIProp) == nil {
		return fmt.Errorf("not responding to other bots: %w", ErrNoResponse)
	}

	// Check we are mentioned like @ai
	if bot := p.GetBotMentioned(post.Message); bot != nil {
		return p.handleMentions(bot, post, postingUser, channel)
	}

	// Check if this is post in the DM channel with any bot
	if bot := p.GetBotForDMChannel(channel); bot != nil {
		return p.handleDMs(bot, channel, postingUser, post)
	}

	return nil
}

func (p *Plugin) handleMentions(bot *Bot, post *model.Post, postingUser *model.User, channel *model.Channel) error {
	if err := p.checkUsageRestrictions(postingUser.Id, bot, channel); err != nil {
		return err
	}

	stream, err := p.processUserRequestToBot(bot, postingUser, channel, post)
	if err != nil {
		return fmt.Errorf("unable to process bot mention: %w", err)
	}

	responseRootID := post.Id
	if post.RootId != "" {
		responseRootID = post.RootId
	}

	responsePost := &model.Post{
		ChannelId: channel.Id,
		RootId:    responseRootID,
	}
	if err := p.streamResultToNewPost(bot.mmBot.UserId, postingUser.Id, stream, responsePost); err != nil {
		return fmt.Errorf("unable to stream response: %w", err)
	}

	return nil
}

func (p *Plugin) handleDMs(bot *Bot, channel *model.Channel, postingUser *model.User, post *model.Post) error {
	if err := p.checkUsageRestrictionsForUser(bot, postingUser.Id); err != nil {
		return err
	}

	stream, err := p.processUserRequestToBot(bot, postingUser, channel, post)
	if err != nil {
		return fmt.Errorf("unable to process bot mention: %w", err)
	}

	responseRootID := post.Id
	if post.RootId != "" {
		responseRootID = post.RootId
	}

	responsePost := &model.Post{
		ChannelId: channel.Id,
		RootId:    responseRootID,
	}
	if err := p.streamResultToNewPost(bot.mmBot.UserId, postingUser.Id, stream, responsePost); err != nil {
		return fmt.Errorf("unable to stream response: %w", err)
	}

	return nil
}

// indexPost adds a post to the vector database for future searches
func (p *Plugin) indexPost(post *model.Post) error {
	// If search is not configured, skip indexing
	if p.search == nil {
		return nil
	}

	// Get channel to retrieve team ID
	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		return fmt.Errorf("failed to get channel for post: %w", err)
	}

	if !p.ShouldIndexPost(post, channel) {
		return nil
	}

	// Create document for vector db
	doc := embeddings.PostDocument{
		PostID:    post.Id,
		CreateAt:  post.CreateAt,
		TeamID:    channel.TeamId,
		ChannelID: post.ChannelId,
		UserID:    post.UserId,
		Content:   post.Message,
	}

	// Store in vector DB
	if err := p.search.Store(context.Background(), []embeddings.PostDocument{doc}); err != nil {
		return fmt.Errorf("failed to store post in vector database: %w", err)
	}

	return nil
}

// MessageHasBeenUpdated is called when a message is updated
// For updated posts, we remove the old version and add the new version
func (p *Plugin) MessageHasBeenUpdated(c *plugin.Context, newPost, oldPost *model.Post) {
	// If search is not configured, skip indexing
	if p.search == nil {
		return
	}

	if err := p.search.Delete(context.Background(), []string{oldPost.Id}); err != nil {
		p.pluginAPI.Log.Error("Failed to delete post from vector database", "error", err)
		return
	}

	if err := p.indexPost(newPost); err != nil {
		p.pluginAPI.Log.Error("Failed to index updated post in vector database", "error", err)
		return
	}
}
