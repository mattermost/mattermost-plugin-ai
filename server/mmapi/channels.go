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
