package main

import (
	"fmt"
	"slices"

	"errors"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

var ErrUsageRestriction = errors.New("usage restriction")

func (p *Plugin) checkUsageRestrictions(requestingUserID string, bot *Bot, channel *model.Channel) error {
	if err := p.checkUsageRestrictionsForUser(bot, requestingUserID); err != nil {
		return err
	}

	if err := p.checkUsageRestrictionsForChannel(bot, channel); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) checkUsageRestrictionsForChannel(bot *Bot, channel *model.Channel) error {
	switch bot.cfg.ChannelAssistanceLevel {
	case ai.ChannelAssistanceLevelAll:
		return nil
	case ai.ChannelAssistanceLevelAllow:
		if !slices.Contains(bot.cfg.ChannelIDs, channel.Id) {
			return fmt.Errorf("channel not allowed: %w", ErrUsageRestriction)
		}
		return nil
	case ai.ChannelAssistanceLevelBlock:
		if slices.Contains(bot.cfg.ChannelIDs, channel.Id) {
			return fmt.Errorf("channel blocked: %w", ErrUsageRestriction)
		}
		return nil
	case ai.ChannelAssistanceLevelNone:
		return fmt.Errorf("channel usage block for bot: %w", ErrUsageRestriction)
	}

	return fmt.Errorf("unknown channel assistance level")
}

func (p *Plugin) checkUsageRestrictionsForUser(bot *Bot, requestingUserID string) error {
	switch bot.cfg.UserAssistanceLevel {
	case ai.UserAssistanceLevelAll:
		return nil
	case ai.UserAssistanceLevelAllow:
		if !slices.Contains(bot.cfg.UserIDs, requestingUserID) {
			return fmt.Errorf("user not allowed: %w", ErrUsageRestriction)
		}
		return nil
	case ai.UserAssistanceLevelBlock:
		if slices.Contains(bot.cfg.UserIDs, requestingUserID) {
			return fmt.Errorf("user blocked: %w", ErrUsageRestriction)
		}
		return nil
	case ai.UserAssistanceLevelNone:
		return fmt.Errorf("user usage block for bot: %w", ErrUsageRestriction)
	}

	return fmt.Errorf("unknown user assistance level")
}
