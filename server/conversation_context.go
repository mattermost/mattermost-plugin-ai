// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"time"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

// BuildLLMContextUserRequest is a helper function to collect the required context for a user request.
func (p *Plugin) BuildLLMContextUserRequest(bot *Bot, requestingUser *model.User, channel *model.Channel, opts ...llm.ContextOption) *llm.Context {
	allOpts := []llm.ContextOption{
		p.WithLLMContextServerInfo(),
		p.WithLLMContextRequestingUser(requestingUser),
		p.WithLLMContextChannel(channel),
		p.WithLLMContextBot(bot),
	}
	allOpts = append(allOpts, opts...)

	return llm.NewContext(allOpts...)
}

func (p *Plugin) WithLLMContextServerInfo() llm.ContextOption {
	return func(c *llm.Context) {
		if p.pluginAPI.Configuration.GetConfig().TeamSettings.SiteName != nil {
			c.ServerName = *p.pluginAPI.Configuration.GetConfig().TeamSettings.SiteName
		}

		if license := p.pluginAPI.System.GetLicense(); license != nil && license.Customer != nil {
			c.CompanyName = license.Customer.Company
		}
	}
}

func (p *Plugin) WithLLMContextChannel(channel *model.Channel) llm.ContextOption {
	return func(c *llm.Context) {
		c.Channel = channel

		if channel == nil || (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) {
			return
		}

		team, err := p.pluginAPI.Team.Get(channel.TeamId)
		if err != nil {
			p.pluginAPI.Log.Error("Unable to get team for context", "error", err.Error(), "team_id", channel.TeamId)
			return
		}

		c.Team = team
	}
}

func (p *Plugin) WithLLMContextRequestingUser(user *model.User) llm.ContextOption {
	return func(c *llm.Context) {
		c.RequestingUser = user
		if user != nil {
			tz := user.GetPreferredTimezone()
			loc, err := time.LoadLocation(tz)
			if err == nil && loc != nil {
				c.Time = time.Now().In(loc).Format(time.RFC1123)
			}
		}
	}
}

func (p *Plugin) WithLLMContextDefaultTools(bot *Bot, isDM bool) llm.ContextOption {
	return func(c *llm.Context) {
		c.Tools = p.getToolsStoreForUser(bot, isDM, c.RequestingUser.Id)
	}
}

func (p *Plugin) WithLLMContextParameters(params map[string]interface{}) llm.ContextOption {
	return func(c *llm.Context) {
		c.Parameters = params
	}
}

func (p *Plugin) WithLLMContextBot(bot *Bot) llm.ContextOption {
	return func(c *llm.Context) {
		c.BotName = bot.cfg.DisplayName
		c.CustomInstructions = bot.cfg.CustomInstructions
	}
}

// WithLLMContextToolCallCallback configures the tool store to use the tool calls stream
func (p *Plugin) WithLLMContextToolCallCallback(postID string) llm.ContextOption {
	return func(c *llm.Context) {
		// No longer needed as we use the stream directly
	}
}
