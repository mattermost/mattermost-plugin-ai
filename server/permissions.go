package main

import (
	"strings"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
)

func (p *Plugin) checkUsageRestrictions(userID string, channel *model.Channel) error {
	if !p.pluginAPI.User.HasPermissionToTeam(userID, p.getConfiguration().OnlyUsersOnTeam, model.PermissionViewTeam) {
		return errors.New("user not on allowed team")
	}

	if !strings.Contains(p.getConfiguration().AllowedTeamIDs, channel.TeamId) {
		return errors.New("can't work on this team.")
	}

	if !p.getConfiguration().AllowPrivateChannels {
		if channel.Type != model.ChannelTypeOpen {
			return errors.New("can't work on private channels.")
		}
	}

	return nil
}
