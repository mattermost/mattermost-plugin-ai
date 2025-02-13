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

	// Create Post Action
	if err := service.RegisterAction(
		"create_post",
		"Creates a new post",
		p.createPostAction,
		map[string]any{
			"type": "object",
			"required": []string{"channel_id", "message"},
			"properties": map[string]any{
				"channel_id": {
					"type": "string",
				},
				"message": {
					"type": "string",
				},
				"root_id": {
					"type": "string",
				},
				"file_ids": {
					"type": "array",
					"items": {
						"type": "string",
					},
				},
				"props": {
					"type": "object",
				},
			},
		},
		map[string]any{
			"type": "object",
			"required": []string{"id", "create_at", "channel_id", "message"},
			"properties": map[string]any{
				"id": {
					"type": "string",
				},
				"create_at": {
					"type": "integer",
				},
				"channel_id": {
					"type": "string",
				},
				"message": {
					"type": "string",
				},
			},
		},
		[]string{"create_post"},
	); err != nil {
		return err
	}

	// Update User Preferences Action
	if err := service.RegisterAction(
		"update_user_preferences",
		"Updates preferences for a user",
		p.updateUserPreferencesAction,
		map[string]any{
			"type": "object",
			"required": []string{"user_id", "preferences"},
			"properties": map[string]any{
				"user_id": {
					"type": "string",
				},
				"preferences": {
					"type": "array",
					"items": {
						"type": "object",
						"required": []string{"user_id", "category", "name", "value"},
						"properties": map[string]any{
							"user_id": {
								"type": "string",
							},
							"category": {
								"type": "string",
							},
							"name": {
								"type": "string",
							},
							"value": {
								"type": "string",
							},
						},
					},
				},
			},
		},
		map[string]any{
			"type": "object",
			"required": []string{"user_id"},
			"properties": map[string]any{
				"user_id": {
					"type": "string",
				},
			},
		},
		[]string{"edit_other_users"},
	); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) createPostAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	post := &model.Post{
		ChannelId: payload["channel_id"].(string),
		Message:   payload["message"].(string),
	}

	if rootID, ok := payload["root_id"].(string); ok {
		post.RootId = rootID
	}

	if fileIds, ok := payload["file_ids"].([]any); ok {
		post.FileIds = make([]string, len(fileIds))
		for i, id := range fileIds {
			post.FileIds[i] = id.(string)
		}
	}

	if props, ok := payload["props"].(map[string]any); ok {
		post.Props = props
	}

	// Get user ID from context and set as post creator
	if userID, ok := ctx.Value("user_id").(string); ok {
		post.UserId = userID
	}

	createdPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"id":         createdPost.Id,
		"create_at":  createdPost.CreateAt,
		"channel_id": createdPost.ChannelId,
		"message":    createdPost.Message,
	}, nil
}

func (p *Plugin) updateUserPreferencesAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	userID := payload["user_id"].(string)
	preferencesRaw := payload["preferences"].([]any)
	
	preferences := make([]model.Preference, len(preferencesRaw))
	for i, prefRaw := range preferencesRaw {
		pref := prefRaw.(map[string]any)
		preferences[i] = model.Preference{
			UserId:    pref["user_id"].(string),
			Category:  pref["category"].(string),
			Name:      pref["name"].(string),
			Value:     pref["value"].(string),
		}
	}

	if appErr := p.API.UpdatePreferencesForUser(userID, preferences); appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"user_id": userID,
	}, nil
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
