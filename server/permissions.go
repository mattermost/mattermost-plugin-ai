package main

import (
	"strings"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
)

func (p *Plugin) checkUsageRestrictions(userID string, channel *model.Channel) error {
	if err := p.checkUsageRestrictionsForUser(userID); err != nil {
		return err
	}

	if p.getConfiguration().Config.SecurityConfig.EnableUseRestrictions {
		if !strings.Contains(p.getConfiguration().Config.SecurityConfig.AllowedTeamIDs, channel.TeamId) {
			return errors.New("can't work on this team.")
		}

		if !p.getConfiguration().Config.SecurityConfig.AllowPrivateChannels {
			if channel.Type != model.ChannelTypeOpen {
				return errors.New("can't work on private channels.")
			}
		}
	}

	return nil
}

func (p *Plugin) checkUsageRestrictionsForUser(userID string) error {
	if p.getConfiguration().Config.SecurityConfig.EnableUseRestrictions {
		if !p.pluginAPI.User.HasPermissionToTeam(userID, p.getConfiguration().Config.SecurityConfig.OnlyUsersOnTeam, model.PermissionViewTeam) {
			return errors.New("user not on allowed team")
		}
	}

	return nil
}
