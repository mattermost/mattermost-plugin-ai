// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mmapi

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestIsDMWith(t *testing.T) {
	for _, tc := range []struct {
		name    string
		userID  string
		channel *model.Channel
		want    bool
	}{
		{
			name:    "nil channel",
			userID:  "thisisuserid",
			channel: nil,
			want:    false,
		},
		{
			name:    "not direct channel",
			userID:  "thisisuserid",
			channel: &model.Channel{Type: model.ChannelTypeGroup},
			want:    false,
		},
		{
			name:    "empty user",
			userID:  "",
			channel: &model.Channel{Type: model.ChannelTypeDirect, Name: "thisisuserid__otheruserid"},
			want:    false,
		},
		{
			name:    "not DM with user",
			userID:  "thisisuserid",
			channel: &model.Channel{Type: model.ChannelTypeDirect, Name: "someotheruser__otheruserid"},
			want:    false,
		},
		{
			name:    "DM with user",
			userID:  "thisisuserid",
			channel: &model.Channel{Type: model.ChannelTypeDirect, Name: "thisisuserid__otheruserid"},
			want:    true,
		},
		{
			name:    "DM with user reversed",
			userID:  "thisisuserid",
			channel: &model.Channel{Type: model.ChannelTypeDirect, Name: "otheruserid__thisisuserid"},
			want:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, IsDMWith(tc.userID, tc.channel))
		})
	}
}
