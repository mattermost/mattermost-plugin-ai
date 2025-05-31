// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"context"
	"errors"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/bots"
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

func (p *AgentsService) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if err := p.handleMessages(post); err != nil {
		if errors.Is(err, ErrNoResponse) {
			p.pluginAPI.Log.Debug(err.Error())
		} else {
			p.pluginAPI.Log.Error(err.Error())
		}
	}
}

func (p *AgentsService) handleMessages(post *model.Post) error {
	// Don't respond to ourselves
	if p.bots.IsAnyBot(post.UserId) {
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
	if bot := p.bots.GetBotMentioned(post.Message); bot != nil {
		return p.handleMentions(bot, post, postingUser, channel)
	}

	// Check if this is post in the DM channel with any bot
	if bot := p.bots.GetBotForDMChannel(channel); bot != nil {
		return p.handleDMs(bot, channel, postingUser, post)
	}

	return nil
}

func (p *AgentsService) handleMentions(bot *bots.Bot, post *model.Post, postingUser *model.User, channel *model.Channel) error {
	if err := p.CheckUsageRestrictions(postingUser.Id, bot, channel); err != nil {
		return err
	}

	stream, err := p.conversationService.ProcessUserRequest(bot, postingUser, channel, post)
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
	if err := p.streamingService.StreamToNewPost(context.Background(), bot.GetMMBot().UserId, postingUser.Id, stream, responsePost, post.Id); err != nil {
		return fmt.Errorf("unable to stream response: %w", err)
	}

	return nil
}

func (p *AgentsService) handleDMs(bot *bots.Bot, channel *model.Channel, postingUser *model.User, post *model.Post) error {
	if err := p.CheckUsageRestrictionsForUser(bot, postingUser.Id); err != nil {
		return err
	}

	stream, err := p.conversationService.ProcessUserRequest(bot, postingUser, channel, post)
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
	if err := p.streamingService.StreamToNewPost(context.Background(), bot.GetMMBot().UserId, postingUser.Id, stream, responsePost, post.Id); err != nil {
		return fmt.Errorf("unable to stream response: %w", err)
	}

	return nil
}
