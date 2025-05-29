// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/require"
)

type TestEnvironment struct {
	plugin  *AgentsService
	mockAPI *plugintest.API
	bots    *bots.MMBots
}

// createTestBots creates a test MMBots instance for testing
func createTestBots(mockAPI *plugintest.API, client *pluginapi.Client) *bots.MMBots {
	licenseChecker := enterprise.NewLicenseChecker(client)
	testBots := bots.New(mockAPI, client, licenseChecker)
	return testBots
}

func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	mockAPI := &plugintest.API{}
	client := pluginapi.NewClient(mockAPI, nil)

	// Create test bots instance
	testBots := createTestBots(mockAPI, client)

	// Create agents service
	p := AgentsService{}
	p.pluginAPI = client
	p.SetBotsForTesting(testBots, client)

	var promptErr error
	p.prompts, promptErr = llm.NewPrompts(llm.PromptsFolder)
	require.NoError(t, promptErr)

	e := &TestEnvironment{
		plugin:  &p,
		mockAPI: mockAPI,
		bots:    testBots,
	}

	return e
}

// setupTestBot configures a test bot in the environment
func (e *TestEnvironment) setupTestBot(botConfig llm.BotConfig) {
	// Create a mock bot user
	mmBot := &model.Bot{
		UserId:      "bot-user-id",
		Username:    botConfig.Name,
		DisplayName: botConfig.DisplayName,
	}

	// Create the bot instance
	bot := bots.NewBot(botConfig, mmBot)

	// Set the bot directly for testing
	e.bots.SetBotsForTesting([]*bots.Bot{bot})
}

func (e *TestEnvironment) Cleanup(t *testing.T) {
	t.Helper()
	e.mockAPI.AssertExpectations(t)
}
