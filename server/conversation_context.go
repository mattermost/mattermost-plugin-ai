package main

import (
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) MakeConversationContext(user *model.User, channel *model.Channel, post *model.Post) ai.ConversationContext {
	context := ai.NewConversationContext(user, channel, post)
	if p.pluginAPI.Configuration.GetConfig().TeamSettings.SiteName != nil {
		context.ServerName = *p.pluginAPI.Configuration.GetConfig().TeamSettings.SiteName
	}

	if license := p.pluginAPI.System.GetLicense(); license != nil && license.Customer != nil {
		context.CompanyName = license.Customer.Company
	}

	if channel != nil && (channel.Type != model.ChannelTypeDirect && channel.Type != model.ChannelTypeGroup) {
		team, err := p.pluginAPI.Team.Get(channel.TeamId)
		if err != nil {
			p.pluginAPI.Log.Error("Unable to get team for context", "error", err.Error(), "team_id", channel.TeamId)
		} else {
			context.Team = team
		}
	}

	return context
}
