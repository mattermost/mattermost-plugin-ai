package main

import (
	"net/http"
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
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "Channel blocked",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelBlock,
					ChannelIDs:         []string{"channel1"},
					UserAccessLevel:    llm.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User blocked",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelBlock,
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
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAllow,
					ChannelIDs:         []string{"channel1"},
					UserAccessLevel:    llm.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "User allowed",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelAllow,
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
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAllow,
					ChannelIDs:         []string{"channel2"},
					UserAccessLevel:    llm.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User not allowed",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelAllow,
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
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelNone,
					UserAccessLevel:    llm.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User none",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelNone,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "Channel block but not in list",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelBlock,
					ChannelIDs:         []string{"channel2"},
					UserAccessLevel:    llm.UserAccessLevelAll,
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "User block but not in list",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelBlock,
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
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAllow,
					ChannelIDs:         []string{"channel1"},
					UserAccessLevel:    llm.UserAccessLevelAllow,
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
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAllow,
					ChannelIDs:         []string{"channel1"},
					UserAccessLevel:    llm.UserAccessLevelAllow,
					UserIDs:            []string{"user2"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User allowed via team membership",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelAllow,
					TeamIDs:            []string{"team1"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "User blocked via team membership",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelBlock,
					TeamIDs:            []string{"team1"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User not in allowed team",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelAllow,
					TeamIDs:            []string{"team2"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
		{
			name: "User allowed via direct ID even if not in team",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelAllow,
					UserIDs:            []string{"user1"},
					TeamIDs:            []string{"team2"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  nil,
		},
		{
			name: "User blocked via direct ID even if in allowed team",
			bot: &Bot{
				cfg: llm.BotConfig{
					ChannelAccessLevel: llm.ChannelAccessLevelAll,
					UserAccessLevel:    llm.UserAccessLevelBlock,
					UserIDs:            []string{"user1"},
					TeamIDs:            []string{"team1"},
				},
			},
			channel:        &model.Channel{Id: "channel1"},
			requestingUser: "user1",
			expectedError:  ErrUsageRestriction,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock responses for team membership checks
			if len(tc.bot.cfg.TeamIDs) > 0 {
				member := &model.TeamMember{
					TeamId: "team1",
					UserId: "user1",
				}
				e.mockAPI.On("GetTeamMember", "team1", "user1").Return(member, nil).Maybe()
				e.mockAPI.On("GetTeamMember", "team2", "user1").Return(nil, &model.AppError{Message: "not found", StatusCode: http.StatusNotFound}).Maybe()
			}

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
