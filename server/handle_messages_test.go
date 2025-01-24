// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestHandleMessages(t *testing.T) {
	e := SetupTestEnvironment(t)
	defer e.Cleanup(t)

	t.Run("don't respond to ourselves", func(t *testing.T) {
		err := e.plugin.handleMessages(&model.Post{
			UserId: "botid",
		})
		require.ErrorIs(t, err, ErrNoResponse)
	})

	t.Run("don't respond to remote posts", func(t *testing.T) {
		remoteid := "remoteid"
		err := e.plugin.handleMessages(&model.Post{
			UserId:   "userid",
			RemoteId: &remoteid,
		})
		require.ErrorIs(t, err, ErrNoResponse)
	})

	t.Run("don't respond to plugins", func(t *testing.T) {
		e.ResetMocks(t)
		post := &model.Post{
			UserId: "userid",
		}
		post.AddProp("from_plugin", true)
		err := e.plugin.handleMessages(post)
		require.ErrorIs(t, err, ErrNoResponse)
	})

	t.Run("don't respond to webhooks", func(t *testing.T) {
		e.ResetMocks(t)
		post := &model.Post{
			UserId: "userid",
		}
		post.AddProp("from_webhook", true)
		err := e.plugin.handleMessages(post)
		require.ErrorIs(t, err, ErrNoResponse)
	})
}
