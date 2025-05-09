// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mmapi

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type Client interface {
	GetUser(userID string) (*model.User, error)
	GetPost(postID string) (*model.Post, error)
	AddReaction(*model.Reaction) error
}

func NewClient(pluginAPI *pluginapi.Client) Client {
	return &client{
		PostService: pluginAPI.Post,
		UserService: pluginAPI.User,
		pluginAPI:   pluginAPI,
	}
}

type client struct {
	pluginapi.PostService
	pluginapi.UserService
	pluginAPI *pluginapi.Client
}

func (m *client) GetUser(userID string) (*model.User, error) {
	return m.pluginAPI.User.Get(userID)
}
