package main

import (
	"fmt"
	"strings"

	"errors"

	"github.com/mattermost/mattermost-plugin-ai/server/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
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
			return fmt.Errorf("can't work on this team: %w", ErrUsageRestriction)
		}

		if !cfg.AllowPrivateChannels {
			if channel.Type != model.ChannelTypeOpen {
				if !mmapi.IsDMWith(p.botid, channel) {
					return fmt.Errorf("can't work on private channels: %w", ErrUsageRestriction)
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
			return fmt.Errorf("user not on allowed team: %w", ErrUsageRestriction)
		}
	}

	return nil
}
