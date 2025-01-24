// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

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
