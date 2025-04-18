// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mmapi

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

func IsDMWith(userID string, channel *model.Channel) bool {
	return channel != nil &&
		channel.Type == model.ChannelTypeDirect &&
		userID != "" &&
		strings.Contains(channel.Name, userID)
}
