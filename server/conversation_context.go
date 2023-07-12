package main

import (
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-server/v6/model"
)

func (p *Plugin) MakeConversationContext(user *model.User, channel *model.Channel, post *model.Post) ai.ConversationContext {
	context := ai.NewConversationContext(user, channel, post)
	if p.pluginAPI.Configuration.GetConfig().TeamSettings.SiteName != nil {
		context.ServerName = *p.pluginAPI.Configuration.GetConfig().TeamSettings.SiteName
	}

	if license := p.pluginAPI.System.GetLicense(); license != nil && license.Customer != nil {
		context.CompanyName = license.Customer.Company
	}

	return context
}
