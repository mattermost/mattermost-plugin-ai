package main

import (
	"context"
	"encoding/json"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) registerChannelActions(service *microactions.Service) error {
	// Create Channel Action
	if err := service.RegisterAction(
		"create_channel",
		"Creates a new channel",
		p.createChannelAction,
		map[string]any{
			"type": "object",
			"required": []string{"team_id", "name", "display_name", "type"},
			"properties": map[string]any{
				"team_id": {
					"type": "string",
				},
				"name": {
					"type": "string",
				},
				"display_name": {
					"type": "string",
				},
				"type": {
					"type": "string",
					"enum": []string{"O", "P"},
				},
				"purpose": {
					"type": "string",
				},
				"header": {
					"type": "string",
				},
			},
		},
		map[string]any{
			"type": "object",
			"required": []string{"id", "name", "display_name"},
			"properties": map[string]any{
				"id": {
					"type": "string",
				},
				"name": {
					"type": "string",
				},
				"display_name": {
					"type": "string",
				},
			},
		},
		[]string{"create_public_channel"},
	); err != nil {
		return err
	}

	// Add Channel Member Action
	if err := service.RegisterAction(
		"add_channel_member",
		"Adds a user to a channel",
		p.addChannelMemberAction,
		map[string]any{
			"type": "object",
			"required": []string{"channel_id", "user_id"},
			"properties": map[string]any{
				"channel_id": {
					"type": "string",
				},
				"user_id": {
					"type": "string",
				},
			},
		},
		map[string]any{
			"type": "object",
			"required": []string{"channel_id", "user_id"},
			"properties": map[string]any{
				"channel_id": {
					"type": "string",
				},
				"user_id": {
					"type": "string",
				},
			},
		},
		[]string{"add_user_to_channel"},
	); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) createChannelAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	channel := &model.Channel{
		TeamId:      payload["team_id"].(string),
		Name:        payload["name"].(string),
		DisplayName: payload["display_name"].(string),
		Type:        model.ChannelType(payload["type"].(string)),
	}

	if purpose, ok := payload["purpose"].(string); ok {
		channel.Purpose = purpose
	}
	if header, ok := payload["header"].(string); ok {
		channel.Header = header
	}

	createdChannel, appErr := p.API.CreateChannel(channel)
	if appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"id":           createdChannel.Id,
		"name":         createdChannel.Name,
		"display_name": createdChannel.DisplayName,
	}, nil
}

func (p *Plugin) addChannelMemberAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	channelId := payload["channel_id"].(string)
	userId := payload["user_id"].(string)

	_, appErr := p.API.AddChannelMember(channelId, userId)
	if appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"channel_id": channelId,
		"user_id":    userId,
	}, nil
}
