// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/config"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

type BotMigrationConfig struct {
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

func MigrateServicesToBots(mutexAPI cluster.MutexPluginAPI, pluginAPI *pluginapi.Client, cfg config.Config) (bool, config.Config, error) {
	mtx, err := cluster.NewMutex(mutexAPI, "migrate_services_to_bots")
	if err != nil {
		return false, cfg, fmt.Errorf("failed to create mutex: %w", err)
	}
	mtx.Lock()
	defer mtx.Unlock()

	migrationDone := false
	_ = pluginAPI.KV.Get("migrate_services_to_bots_done", &migrationDone)
	if migrationDone {
		return false, cfg, nil
	}

	pluginAPI.Log.Debug("Migrating services to bots")

	existingConfig := cfg.Clone()

	if len(existingConfig.Bots) != 0 {
		_, _ = pluginAPI.KV.Set("migrate_services_to_bots_done", true)
		return false, cfg, nil
	}

	oldConfig := BotMigrationConfig{}
	err = pluginAPI.Configuration.LoadPluginConfiguration(&oldConfig)
	if err != nil {
		return false, cfg, fmt.Errorf("failed to load plugin configuration for migration: %w", err)
	}

	existingConfig.Bots = make([]llm.BotConfig, 0, len(oldConfig.Config.Services))
	for _, service := range oldConfig.Config.Services {
		existingConfig.Bots = append(existingConfig.Bots, llm.BotConfig{
			DisplayName: service.Name,
			ID:          service.Name,
			Service: llm.ServiceConfig{
				Type:            service.ServiceName,
				DefaultModel:    service.DefaultModel,
				OrgID:           service.OrgID,
				APIURL:          service.URL,
				APIKey:          service.APIKey,
				InputTokenLimit: service.TokenLimit,
			},
		})
	}

	// If there is one bot then give it the standard name
	if len(existingConfig.Bots) == 1 {
		existingConfig.Bots[0].Name = "ai"
		existingConfig.Bots[0].DisplayName = "Copilot"
	}

	out := map[string]any{}
	marshalBytes, err := json.Marshal(existingConfig)
	if err != nil {
		return false, cfg, fmt.Errorf("failed to marshal configuration: %w", err)
	}
	if err := json.Unmarshal(marshalBytes, &out); err != nil {
		return false, cfg, fmt.Errorf("failed to unmarshal configuration to output: %w", err)
	}

	if err := pluginAPI.Configuration.SavePluginConfig(out); err != nil {
		return false, cfg, fmt.Errorf("failed to save plugin configuration: %w", err)
	}
	_, _ = pluginAPI.KV.Set("migrate_services_to_bots_done", true)

	return true, cfg, nil
}
