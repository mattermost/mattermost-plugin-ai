package main

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/microactions"
	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) registerChannelActions(service *microactions.Service) error {
	// Create Channel Action
	if err := service.RegisterAction(
		"create_channel",
		"Creates a new channel",
		p.createChannelAction,
		map[string]any{
			"type":     "object",
			"required": []string{"team_id", "name", "display_name", "type"},
			"properties": map[string]any{
				"team_id": map[string]string{
					"type": "string",
				},
				"name": map[string]string{
					"type": "string",
				},
				"display_name": map[string]string{
					"type": "string",
				},
				"type": map[string]any{
					"type": "string",
					"enum": []string{"O", "P"},
				},
				"purpose": map[string]string{
					"type": "string",
				},
				"header": map[string]string{
					"type": "string",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"id", "name", "display_name"},
			"properties": map[string]any{
				"id": map[string]string{
					"type": "string",
				},
				"name": map[string]string{
					"type": "string",
				},
				"display_name": map[string]string{
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
			"type":     "object",
			"required": []string{"channel_id", "user_id"},
			"properties": map[string]any{
				"channel_id": map[string]string{
					"type": "string",
				},
				"user_id": map[string]string{
					"type": "string",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"channel_id", "user_id"},
			"properties": map[string]any{
				"channel_id": map[string]string{
					"type": "string",
				},
				"user_id": map[string]string{
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
			"type":     "object",
			"required": []string{"channel_id", "message"},
			"properties": map[string]any{
				"channel_id": map[string]string{
					"type": "string",
				},
				"message": map[string]string{
					"type": "string",
				},
				"root_id": map[string]string{
					"type": "string",
				},
				"file_ids": map[string]any{
					"type": "array",
					"items": map[string]string{
						"type": "string",
					},
				},
				"props": map[string]string{
					"type": "object",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"id", "create_at", "channel_id", "message"},
			"properties": map[string]any{
				"id": map[string]string{
					"type": "string",
				},
				"create_at": map[string]string{
					"type": "integer",
				},
				"channel_id": map[string]string{
					"type": "string",
				},
				"message": map[string]string{
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
			"type":     "object",
			"required": []string{"user_id", "preferences"},
			"properties": map[string]any{
				"user_id": map[string]string{
					"type": "string",
				},
				"preferences": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":     "object",
						"required": []string{"user_id", "category", "name", "value"},
						"properties": map[string]any{
							"user_id": map[string]any{
								"type": "string",
							},
							"category": map[string]any{
								"type": "string",
							},
							"name": map[string]any{
								"type": "string",
							},
							"value": map[string]any{
								"type": "string",
							},
						},
					},
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"user_id"},
			"properties": map[string]any{
				"user_id": map[string]any{
					"type": "string",
				},
			},
		},
		[]string{"edit_other_users"},
	); err != nil {
		return err
	}

	// Execute Slash Command Action
	if err := service.RegisterAction(
		"execute_slash_command",
		"Executes a slash command",
		p.executeSlashCommandAction,
		map[string]any{
			"type":     "object",
			"required": []string{"channel_id", "command"},
			"properties": map[string]any{
				"channel_id": map[string]any{
					"type": "string",
				},
				"command": map[string]any{
					"type": "string",
				},
				"team_id": map[string]any{
					"type": "string",
				},
				"root_id": map[string]any{
					"type": "string",
				},
				"parent_id": map[string]any{
					"type": "string",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"response_type", "text"},
			"properties": map[string]any{
				"response_type": map[string]any{
					"type": "string",
					"enum": []string{"in_channel", "ephemeral"},
				},
				"text": map[string]any{
					"type": "string",
				},
				"username": map[string]any{
					"type": "string",
				},
				"icon_url": map[string]any{
					"type": "string",
				},
				"goto_location": map[string]any{
					"type": "string",
				},
				"attachments": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
					},
				},
			},
		},
		[]string{"execute_slash_commands"},
	); err != nil {
		return err
	}

	// Create User Action
	if err := service.RegisterAction(
		"create_user",
		"Creates a new user",
		p.createUserAction,
		map[string]any{
			"type":     "object",
			"required": []string{"username", "email", "password"},
			"properties": map[string]any{
				"username": map[string]string{
					"type": "string",
				},
				"email": map[string]string{
					"type": "string",
				},
				"password": map[string]string{
					"type": "string",
				},
				"nickname": map[string]string{
					"type": "string",
				},
				"first_name": map[string]string{
					"type": "string",
				},
				"last_name": map[string]string{
					"type": "string",
				},
				"locale": map[string]string{
					"type": "string",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"id", "username", "email"},
			"properties": map[string]any{
				"id": map[string]string{
					"type": "string",
				},
				"username": map[string]string{
					"type": "string",
				},
				"email": map[string]string{
					"type": "string",
				},
			},
		},
		[]string{"create_user"},
	); err != nil {
		return err
	}

	// Remove Channel Member Action
	if err := service.RegisterAction(
		"remove_channel_member",
		"Removes a user from a channel",
		p.removeChannelMemberAction,
		map[string]any{
			"type":     "object",
			"required": []string{"channel_id", "user_id"},
			"properties": map[string]any{
				"channel_id": map[string]string{
					"type": "string",
				},
				"user_id": map[string]string{
					"type": "string",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"channel_id", "user_id"},
			"properties": map[string]any{
				"channel_id": map[string]string{
					"type": "string",
				},
				"user_id": map[string]string{
					"type": "string",
				},
			},
		},
		[]string{"remove_user_from_channel"},
	); err != nil {
		return err
	}

	// Create Team Action
	if err := service.RegisterAction(
		"create_team",
		"Creates a new team",
		p.createTeamAction,
		map[string]any{
			"type":     "object",
			"required": []string{"name", "display_name", "type"},
			"properties": map[string]any{
				"name": map[string]string{
					"type": "string",
				},
				"display_name": map[string]string{
					"type": "string",
				},
				"type": map[string]any{
					"type": "string",
					"enum": []string{"O", "I"},
				},
				"description": map[string]string{
					"type": "string",
				},
				"allow_open_invite": map[string]string{
					"type": "boolean",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"id", "name", "display_name"},
			"properties": map[string]any{
				"id": map[string]string{
					"type": "string",
				},
				"name": map[string]string{
					"type": "string",
				},
				"display_name": map[string]string{
					"type": "string",
				},
			},
		},
		[]string{"create_team"},
	); err != nil {
		return err
	}

	// Add Team Member Action
	if err := service.RegisterAction(
		"add_team_member",
		"Adds a user to a team",
		p.addTeamMemberAction,
		map[string]any{
			"type":     "object",
			"required": []string{"team_id", "user_id"},
			"properties": map[string]any{
				"team_id": map[string]string{
					"type": "string",
				},
				"user_id": map[string]string{
					"type": "string",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"team_id", "user_id"},
			"properties": map[string]any{
				"team_id": map[string]string{
					"type": "string",
				},
				"user_id": map[string]string{
					"type": "string",
				},
			},
		},
		[]string{"add_user_to_team"},
	); err != nil {
		return err
	}

	// Update Channel Action
	if err := service.RegisterAction(
		"update_channel",
		"Updates an existing channel",
		p.updateChannelAction,
		map[string]any{
			"type":     "object",
			"required": []string{"id", "name", "display_name", "type"},
			"properties": map[string]any{
				"id": map[string]string{
					"type": "string",
				},
				"name": map[string]string{
					"type": "string",
				},
				"display_name": map[string]string{
					"type": "string",
				},
				"type": map[string]any{
					"type": "string",
					"enum": []string{"O", "P"},
				},
				"purpose": map[string]string{
					"type": "string",
				},
				"header": map[string]string{
					"type": "string",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"id", "name", "display_name"},
			"properties": map[string]any{
				"id": map[string]string{
					"type": "string",
				},
				"name": map[string]string{
					"type": "string",
				},
				"display_name": map[string]string{
					"type": "string",
				},
			},
		},
		[]string{"manage_public_channel_properties"},
	); err != nil {
		return err
	}

	// Remove Team Member Action
	if err := service.RegisterAction(
		"remove_team_member",
		"Removes a user from a team",
		p.removeTeamMemberAction,
		map[string]any{
			"type":     "object",
			"required": []string{"team_id", "user_id", "requestor_id"},
			"properties": map[string]any{
				"team_id": map[string]string{
					"type": "string",
				},
				"user_id": map[string]string{
					"type": "string",
				},
				"requestor_id": map[string]string{
					"type": "string",
				},
			},
		},
		map[string]any{
			"type":     "object",
			"required": []string{"team_id", "user_id"},
			"properties": map[string]any{
				"team_id": map[string]string{
					"type": "string",
				},
				"user_id": map[string]string{
					"type": "string",
				},
			},
		},
		[]string{"remove_user_from_team"},
	); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) createUserAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	user := &model.User{
		Username: payload["username"].(string),
		Email:    payload["email"].(string),
		Password: payload["password"].(string),
	}

	if nickname, ok := payload["nickname"].(string); ok {
		user.Nickname = nickname
	}
	if firstName, ok := payload["first_name"].(string); ok {
		user.FirstName = firstName
	}
	if lastName, ok := payload["last_name"].(string); ok {
		user.LastName = lastName
	}
	if locale, ok := payload["locale"].(string); ok {
		user.Locale = locale
	}

	createdUser, appErr := p.API.CreateUser(user)
	if appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"id":       createdUser.Id,
		"username": createdUser.Username,
		"email":    createdUser.Email,
	}, nil
}

func (p *Plugin) removeChannelMemberAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	channelId := payload["channel_id"].(string)
	userId := payload["user_id"].(string)

	if appErr := p.API.DeleteChannelMember(channelId, userId); appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"channel_id": channelId,
		"user_id":    userId,
	}, nil
}

func (p *Plugin) executeSlashCommandAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	userID, ok := ctx.Value("user_id").(string)
	if !ok {
		return nil, fmt.Errorf("user_id not found in context")
	}

	args := &model.CommandArgs{
		Command:   payload["command"].(string),
		ChannelId: payload["channel_id"].(string),
		UserId:    userID,
	}

	// Optional fields
	if teamID, ok := payload["team_id"].(string); ok {
		args.TeamId = teamID
	}
	if rootID, ok := payload["root_id"].(string); ok {
		args.RootId = rootID
	}
	if parentID, ok := payload["parent_id"].(string); ok {
		args.ParentId = parentID
	}

	response, err := p.API.ExecuteSlashCommand(args)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"response_type": response.ResponseType,
		"text":          response.Text,
	}

	// Add optional fields if they exist
	if response.Username != "" {
		result["username"] = response.Username
	}
	if response.IconURL != "" {
		result["icon_url"] = response.IconURL
	}
	if response.GotoLocation != "" {
		result["goto_location"] = response.GotoLocation
	}
	if len(response.Attachments) > 0 {
		result["attachments"] = response.Attachments
	}

	return result, nil
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
			UserId:   pref["user_id"].(string),
			Category: pref["category"].(string),
			Name:     pref["name"].(string),
			Value:    pref["value"].(string),
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

func (p *Plugin) createTeamAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	team := &model.Team{
		Name:        payload["name"].(string),
		DisplayName: payload["display_name"].(string),
		Type:        payload["type"].(string),
	}

	if description, ok := payload["description"].(string); ok {
		team.Description = description
	}
	if allowOpenInvite, ok := payload["allow_open_invite"].(bool); ok {
		team.AllowOpenInvite = allowOpenInvite
	}

	createdTeam, appErr := p.API.CreateTeam(team)
	if appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"id":           createdTeam.Id,
		"name":         createdTeam.Name,
		"display_name": createdTeam.DisplayName,
	}, nil
}

func (p *Plugin) addTeamMemberAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	teamId := payload["team_id"].(string)
	userId := payload["user_id"].(string)

	_, appErr := p.API.CreateTeamMember(teamId, userId)
	if appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"team_id": teamId,
		"user_id": userId,
	}, nil
}

func (p *Plugin) removeTeamMemberAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	teamId := payload["team_id"].(string)
	userId := payload["user_id"].(string)
	requestorId := payload["requestor_id"].(string)

	if appErr := p.API.DeleteTeamMember(teamId, userId, requestorId); appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"team_id": teamId,
		"user_id": userId,
	}, nil
}

func (p *Plugin) updateChannelAction(ctx context.Context, payload map[string]any) (map[string]any, error) {
	channel := &model.Channel{
		Id:          payload["id"].(string),
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

	updatedChannel, appErr := p.API.UpdateChannel(channel)
	if appErr != nil {
		return nil, appErr
	}

	return map[string]any{
		"id":           updatedChannel.Id,
		"name":         updatedChannel.Name,
		"display_name": updatedChannel.DisplayName,
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
