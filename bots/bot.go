// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bots

import (
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost/server/public/model"
)

type Bot struct {
	cfg   llm.BotConfig
	mmBot *model.Bot
}

func NewBot(cfg llm.BotConfig, bot *model.Bot) *Bot {
	return &Bot{
		cfg:   cfg,
		mmBot: bot,
	}
}

func (b *Bot) GetConfig() llm.BotConfig {
	return b.cfg
}

func (b *Bot) GetMMBot() *model.Bot {
	return b.mmBot
}
