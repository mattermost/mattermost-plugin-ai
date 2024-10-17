package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
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

	p.bots = []*Bot{
		{
			cfg: ai.BotConfig{
				Name: "ai",
			},
			mmBot: &model.Bot{
				UserId:   "botid",
				Username: "ai",
			},
		},
	}

	var promptErr error
	p.prompts, promptErr = ai.NewPrompts(promptsFolder)
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

func TestUsageRestrictions(t *testing.T) {
	e := SetupTestEnvironment(t)
	defer e.Cleanup(t)

	testCases := []struct {
		name           string
		bot            *Bot
		channel        *model.Channel
		requestingUser string
		expectedError  error
	}{
		{
			name: "All allowed",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAll,
					UserAccessLevel:    ai.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "Channel blocked",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelBlock,
					ChannelIDs:         []string{"channel1"},
					UserAccessLevel:    ai.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User blocked",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAll,
					UserAccessLevel:    ai.UserAccessLevelBlock,
					UserIDs:            []string{"user1"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "Channel allowed",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAllow,
					ChannelIDs:         []string{"channel1"},
					UserAccessLevel:    ai.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "User allowed",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAll,
					UserAccessLevel:    ai.UserAccessLevelAllow,
					UserIDs:            []string{"user1"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "Channel not allowed",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAllow,
					ChannelIDs:         []string{"channel2"},
					UserAccessLevel:    ai.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User not allowed",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAll,
					UserAccessLevel:    ai.UserAccessLevelAllow,
					UserIDs:            []string{"user2"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "Channel none",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelNone,
					UserAccessLevel:    ai.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User none",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAll,
					UserAccessLevel:    ai.UserAccessLevelNone,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "Channel block but not in list",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelBlock,
					ChannelIDs:         []string{"channel2"},
					UserAccessLevel:    ai.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "User block but not in list",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAll,
					UserAccessLevel:    ai.UserAccessLevelBlock,
					UserIDs:            []string{"user2"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "Channel allow and user allow",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAllow,
					ChannelIDs:         []string{"channel1"},
					UserAccessLevel:    ai.UserAccessLevelAllow,
					UserIDs:            []string{"user1"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "Channel allow but user not allowed",
			bot: &Bot{
				cfg: ai.BotConfig{
					ChannelAccessLevel: ai.ChannelAccessLevelAllow,
					ChannelIDs:         []string{"channel1"},
					UserAccessLevel:    ai.UserAccessLevelAllow,
					UserIDs:            []string{"user2"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := e.plugin.checkUsageRestrictions(tc.requestingUser, tc.bot, tc.channel)
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHandleMessages(t *testing.T) {
	e := SetupTestEnvironment(t)
	defer e.Cleanup(t)

	t.Run("don't respond to ourselves", func(t *testing.T) {
		err := e.plugin.handleMessages(&model.Post{
			UserId: "botid",
		})
		require.ErrorIs(t, err, ErrNoResponse)
	})

	t.Run("don't respond to remote posts", func(t *testing.T) {
		remoteid := "remoteid"
		err := e.plugin.handleMessages(&model.Post{
			UserId:   "userid",
			RemoteId: &remoteid,
		})
		require.ErrorIs(t, err, ErrNoResponse)
	})

	t.Run("don't respond to plugins", func(t *testing.T) {
		e.ResetMocks(t)
		post := &model.Post{
			UserId: "userid",
		}
		post.AddProp("from_plugin", true)
		err := e.plugin.handleMessages(post)
		require.ErrorIs(t, err, ErrNoResponse)
	})

	t.Run("don't respond to webhooks", func(t *testing.T) {
		e.ResetMocks(t)
		post := &model.Post{
			UserId: "userid",
		}
		post.AddProp("from_webhook", true)
		err := e.plugin.handleMessages(post)
		require.ErrorIs(t, err, ErrNoResponse)
	})
}
