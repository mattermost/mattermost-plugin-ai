// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	CommandAI = "ai"
)

func (p *Plugin) registerCommands() error {
	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          CommandAI,
		AutoComplete:     true,
		AutoCompleteDesc: "AI assistant for Mattermost",
		AutoCompleteHint: "[command]",
	}); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)
	if len(split) < 2 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Please specify a command. Use /ai help for available commands.",
		}, nil
	}

	command := split[1]

	switch command {
	case "create-smart-webhook":
		return p.executeCreateSmartWebhook(c, args)
	case "help":
		return p.executeHelp(c, args)
	default:
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: %s. Use /ai help for available commands.", command),
		}, nil
	}
}

func (p *Plugin) executeHelp(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	helpText := "Available commands:\n" +
		"* `/ai create-smart-webhook [username] [icon_url]` - Create a smart webhook for the current channel"

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         helpText,
	}, nil
}

func (p *Plugin) executeCreateSmartWebhook(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	// Parse arguments
	split := strings.Fields(args.Command)

	// Default values
	username := "Smart Webhook"
	iconURL := ""

	// Override defaults if parameters were provided
	if len(split) > 2 {
		username = split[2]
	}

	if len(split) > 3 {
		iconURL = split[3]
	}

	// Generate a unique ID
	id := model.NewId()

	// Store data in key-value store
	// Format: channelId,Username,iconUrl
	value := fmt.Sprintf("%s,%s,%s", args.ChannelId, username, iconURL)
	key := fmt.Sprintf("smart_webhook_%s", id)

	if err := p.API.KVSet(key, []byte(value)); err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Failed to create smart webhook: %v", err),
		}, nil
	}

	// Construct response with webhook endpoint info
	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	if siteURL == nil {
		siteURL = new(string)
	}

	channel, err := p.pluginAPI.Channel.Get(args.ChannelId)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Failed to create smart webhook: %v", err),
		}, nil
	}

	webhookURL := fmt.Sprintf("%s/plugins/mattermost-ai/smart-webhook/%s", *siteURL, id)

	responseText := fmt.Sprintf("Successfully created smart webhook!\n"+
		"**Webhook ID**: %s\n"+
		"**Username**: %s\n"+
		"**Channel**: %s\n"+
		"**Webhook URL**: %s\n\n"+
		"To use this webhook, send a POST request with JSON to the webhook URL.",
		id, username, channel.DisplayName, webhookURL)

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         responseText,
	}, nil
}
