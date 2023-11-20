package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

var ErrUsageRestriction = errors.New("usage restriction")

func (p *Plugin) checkUsageRestrictions(userID string, channel *model.Channel) error {
	if err := p.checkUsageRestrictionsForUser(userID); err != nil {
		return err
	}

	if err := p.checkUsageRestrictionsForChannel(channel); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) checkUsageRestrictionsForChannel(channel *model.Channel) error {
	cfg := p.getConfiguration()
	if cfg.EnableUseRestrictions {
		if cfg.AllowedTeamIDs != "" && !strings.Contains(cfg.AllowedTeamIDs, channel.TeamId) {
			return errors.Wrap(ErrUsageRestriction, "can't work on this team")
		}

		if !cfg.AllowPrivateChannels {
			if channel.Type != model.ChannelTypeOpen {
				if !(channel.Type == model.ChannelTypeDirect && strings.Contains(channel.Name, p.botid)) {
					return errors.Wrap(ErrUsageRestriction, "can't work on private channels")
				}
			}
		}
	}
	return nil
}

func (p *Plugin) checkUsageRestrictionsForUser(userID string) error {
	cfg := p.getConfiguration()
	if cfg.EnableUseRestrictions && cfg.OnlyUsersOnTeam != "" {
		if !p.pluginAPI.User.HasPermissionToTeam(userID, cfg.OnlyUsersOnTeam, model.PermissionViewTeam) {
			return errors.Wrap(ErrUsageRestriction, "user not on allowed team")
		}
	}

	return nil
}
