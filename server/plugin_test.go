// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/require"
)

type TestEnvironment struct {
	plugin  *Plugin
	mockAPI *plugintest.API
}

func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	p := Plugin{}

	// Setup mock team member responses
	p.pluginAPI = &pluginapi.Client{}

	p.bots = []*Bot{
		{
			cfg: llm.BotConfig{
				Name: "ai",
			},
			mmBot: &model.Bot{
				UserId:   "botid",
				Username: "ai",
			},
		},
	}

	var promptErr error
	p.prompts, promptErr = llm.NewPrompts(promptsFolder)
	require.NoError(t, promptErr)

	p.ffmpegPath = ""

	e := &TestEnvironment{
		plugin: &p,
	}
	e.ResetMocks(t)

	return e
}

func (e *TestEnvironment) ResetMocks(t *testing.T) {
	e.mockAPI = &plugintest.API{}
	e.plugin.SetAPI(e.mockAPI)
	e.plugin.pluginAPI = pluginapi.NewClient(e.plugin.API, e.plugin.Driver)
}

func (e *TestEnvironment) Cleanup(t *testing.T) {
	t.Helper()
	e.mockAPI.AssertExpectations(t)
}

func makeConfig(config Config) *configuration {
	return &configuration{
		Config: config,
	}
}
