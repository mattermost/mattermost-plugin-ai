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
	GetPostThread(postID string) (*model.PostList, error)
	GetPostsSince(channelID string, since int64) (*model.PostList, error)
	GetFirstPostBeforeTimeRangeID(channelID string, startTime, endTime int64) (string, error)
	GetPostsBefore(channelID, postID string, page, perPage int) (*model.PostList, error)
}

func NewClient(pluginAPI *pluginapi.Client) Client {
	return &client{
		PostService: pluginAPI.Post,
		UserService: pluginAPI.User,
		pluginAPI:   pluginAPI,
		DBClient:    NewDBClient(pluginAPI),
	}
}

type client struct {
	pluginapi.PostService
	pluginapi.UserService
	*DBClient
	pluginAPI *pluginapi.Client
}

func (m *client) GetUser(userID string) (*model.User, error) {
	return m.pluginAPI.User.Get(userID)
}
