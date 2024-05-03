package main

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

type Bot struct {
	cfg   ai.BotConfig
	mmBot *model.Bot
}

func NewBot(cfg ai.BotConfig, bot *model.Bot) *Bot {
	return &Bot{
		cfg:   cfg,
		mmBot: bot,
	}
}

type MigrationConfig struct {
	Config struct {
		Services []struct {
			Name         string `json:"name"`
			ServiceName  string `json:"serviceName"`
			DefaultModel string `json:"defaultModel"`
			OrgID        string `json:"orgId"`
			URL          string `json:"url"`
			APIKey       string `json:"apiKey"`
			TokenLimit   int    `json:"tokenLimit"`
		} `json:"services"`
	} `json:"config"`
}

func (p *Plugin) MigrateServicesToBots() error {
	mtx, err := cluster.NewMutex(p.API, "migrate_services_to_bots")
	if err != nil {
		return fmt.Errorf("failed to create mutex: %w", err)
	}
	mtx.Lock()
	defer mtx.Unlock()

	migrationDone := false
	p.pluginAPI.KV.Get("migrate_services_to_bots_done", &migrationDone)
	if migrationDone {
		return nil
	}

	p.API.LogDebug("Migrating services to bots")

	existingConfig := p.getConfiguration().Clone()

	if len(existingConfig.Bots) != 0 {
		p.pluginAPI.KV.Set("migrate_services_to_bots_done", true)
		return nil
	}

	oldConfig := MigrationConfig{}
	err = p.API.LoadPluginConfiguration(&oldConfig)
	if err != nil {
		return fmt.Errorf("failed to load plugin configuration for migration: %w", err)
	}

	existingConfig.Bots = make([]ai.BotConfig, 0, len(oldConfig.Config.Services))
	for _, service := range oldConfig.Config.Services {
		existingConfig.Bots = append(existingConfig.Bots, ai.BotConfig{
			DisplayName: service.Name,
			ID:          service.Name,
			Service: ai.ServiceConfig{
				Type:         service.ServiceName,
				DefaultModel: service.DefaultModel,
				OrgID:        service.OrgID,
				APIURL:       service.URL,
				APIKey:       service.APIKey,
				TokenLimit:   service.TokenLimit,
			},
		})
	}

	// If there is one bot then give it the standard name
	if len(existingConfig.Bots) == 1 {
		existingConfig.Bots[0].Name = "ai"
		existingConfig.Bots[0].DisplayName = "AI Copilot"
	}

	out := map[string]any{}
	marshalBytes, err := json.Marshal(existingConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	if err := json.Unmarshal(marshalBytes, &out); err != nil {
		return fmt.Errorf("failed to unmarshal configuration to output: %w", err)
	}

	if err := p.pluginAPI.Configuration.SavePluginConfig(out); err != nil {
		return fmt.Errorf("failed to save plugin configuration: %w", err)
	}
	p.setConfiguration(existingConfig)
	p.pluginAPI.KV.Set("migrate_services_to_bots_done", true)

	return nil
}

func (p *Plugin) EnsureBots() error {
	mtx, err := cluster.NewMutex(p.API, "ai_ensure_bots")
	if err != nil {
		return fmt.Errorf("failed to create mutex: %w", err)
	}
	mtx.Lock()
	defer mtx.Unlock()

	previousMMBots, err := p.pluginAPI.Bot.List(0, 1000, pluginapi.BotOwner("mattermost-ai"), pluginapi.BotIncludeDeleted())
	if err != nil {
		return fmt.Errorf("failed to list bots: %w", err)
	}

	cfgBots := p.getConfiguration().Bots
	// Only allow one bot if not multi-LLM licensed
	if !p.licenseChecker.IsMultiLLMLicensed() {
		p.pluginAPI.Log.Error("Only one bot allowed with current license.")
		cfgBots = cfgBots[:1]
	}

	aiBotConfigsByUsername := make(map[string]ai.BotConfig)
	for _, bot := range cfgBots {
		aiBotConfigsByUsername[bot.Name] = bot
	}

	prevousMMBotsByUsername := make(map[string]*model.Bot)
	for _, bot := range previousMMBots {
		prevousMMBotsByUsername[bot.Username] = bot
	}

	// For each of the bots we found, if it's not in the configuration, delete it.
	for _, bot := range previousMMBots {
		if _, ok := aiBotConfigsByUsername[bot.Username]; !ok {
			if _, err := p.pluginAPI.Bot.UpdateActive(bot.UserId, false); err != nil {
				p.API.LogError("Failed to delete bot", "bot_name", bot.Username, "error", err.Error())
				continue
			}
		}
	}

	// For each bot in the configuration, try to find an existing bot matching the username.
	// If it exists, update it to match. Otherwise, create a new bot.
	for _, bot := range cfgBots {
		if bot.Name == "" {
			continue
		}
		description := "Powered by " + bot.Service.Type
		if prevBot, ok := prevousMMBotsByUsername[bot.Name]; ok {
			if _, err := p.pluginAPI.Bot.Patch(prevBot.UserId, &model.BotPatch{
				DisplayName: &bot.DisplayName,
				Description: &description,
			}); err != nil {
				p.API.LogError("Failed to patch bot", "bot_name", bot.Name, "error", err.Error())
				continue
			}
			if _, err := p.pluginAPI.Bot.UpdateActive(prevBot.UserId, true); err != nil {
				p.API.LogError("Failed to update bot active", "bot_name", bot.Name, "error", err.Error())
				continue
			}
		} else {
			err := p.pluginAPI.Bot.Create(&model.Bot{
				Username:    bot.Name,
				DisplayName: bot.DisplayName,
				Description: description,
			})
			if err != nil {
				p.API.LogError("Failed to ensure bot", "bot_name", bot.Name, "error", err.Error())
				continue
			}
		}
	}

	if err := p.UpdateBotsCache(); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) UpdateBotsCache() error {
	botsConfig := p.getConfiguration().Bots

	bots, err := p.pluginAPI.Bot.List(0, 1000, pluginapi.BotOwner("mattermost-ai"))
	if err != nil {
		return fmt.Errorf("failed to list bots: %w", err)
	}

	p.botsLock.Lock()
	defer p.botsLock.Unlock()
	p.botsByUsername = make(map[string]*Bot, len(botsConfig))
	p.botsByID = make(map[string]*Bot, len(botsConfig))
	for _, botCfg := range botsConfig {
		for _, bot := range bots {
			if bot.Username == botCfg.Name {
				createdBot := NewBot(botCfg, bot)
				p.botsByUsername[botCfg.Name] = createdBot
				p.botsByID[bot.UserId] = createdBot
			}
		}
	}

	return nil
}

func (p *Plugin) GetBotConfig(botUsername string) (ai.BotConfig, error) {
	bot := p.GetBotByUsername(botUsername)
	if bot == nil {
		return ai.BotConfig{}, fmt.Errorf("bot not found")
	}

	return bot.cfg, nil
}

// GetBotByUsername retrieves the bot associated with the given bot username
func (p *Plugin) GetBotByUsername(botUsername string) *Bot {
	p.botsLock.RLock()
	defer p.botsLock.RUnlock()
	if bot, ok := p.botsByUsername[botUsername]; ok {
		return bot
	}

	return nil
}

// GetBotByID retrieves the bot associated with the given bot ID
func (p *Plugin) GetBotByID(botID string) *Bot {
	p.botsLock.RLock()
	defer p.botsLock.RUnlock()
	if bot, ok := p.botsByID[botID]; ok {
		return bot
	}

	return nil
}

// GetBotForDMChannel returns the bot for the given DM channel.
func (p *Plugin) GetBotForDMChannel(channel *model.Channel) *Bot {
	p.botsLock.RLock()
	defer p.botsLock.RUnlock()

	for userID, bot := range p.botsByID {
		if mmapi.IsDMWith(userID, channel) {
			return bot
		}
	}
	return nil
}

// IsAnyBot returns true if the given user is an AI bot.
func (p *Plugin) IsAnyBot(userID string) bool {
	p.botsLock.RLock()
	defer p.botsLock.RUnlock()
	_, ok := p.botsByID[userID]
	return ok
}

// GetBotMentioned returns the bot mentioned in the text, if any.
func (p *Plugin) GetBotMentioned(text string) *Bot {
	p.botsLock.RLock()
	defer p.botsLock.RUnlock()

	for _, bot := range p.botsByUsername {
		if userIsMentioned(text, bot.mmBot.Username) {
			return bot
		}
	}

	return nil
}
