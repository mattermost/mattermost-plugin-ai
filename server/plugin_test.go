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

func TestBotMention(t *testing.T) {
	e := SetupTestEnvironment(t)
	defer e.Cleanup(t)
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
		e.plugin.setConfiguration(makeConfig(Config{
			EnableUseRestrictions: false,
		}))
		post := &model.Post{
			UserId: "userid",
		}
		post.AddProp("from_plugin", true)
		err := e.plugin.handleMessages(post)
		require.ErrorIs(t, err, ErrNoResponse)
	})

	t.Run("don't respond to webhooks", func(t *testing.T) {
		e.ResetMocks(t)
		e.plugin.setConfiguration(makeConfig(Config{
			EnableUseRestrictions: false,
		}))
		post := &model.Post{
			UserId: "userid",
		}
		post.AddProp("from_webhook", true)
		err := e.plugin.handleMessages(post)
		require.ErrorIs(t, err, ErrNoResponse)
	})
}

func TestHandleMentions(t *testing.T) {
	e := SetupTestEnvironment(t)
	defer e.Cleanup(t)

	standardPost := &model.Post{
		UserId:    "userid",
		ChannelId: "channelid",
		Message:   "hello @ai",
	}

	t.Run("don't respond to users that are not allowed", func(t *testing.T) {
		e.ResetMocks(t)
		e.plugin.setConfiguration(makeConfig(Config{
			OnlyUsersOnTeam:       "teamid",
			EnableUseRestrictions: true,
		}))
		e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
			Type:   model.ChannelTypeOpen,
			TeamId: "teamid",
		}, nil)
		e.mockAPI.On("GetUser", "userid").Return(&model.User{
			Id: "userid",
		}, nil)
		e.mockAPI.On("HasPermissionToTeam", "userid", "teamid", model.PermissionViewTeam).Return(false)

		err := e.plugin.handleMessages(standardPost)
		require.ErrorIs(t, err, ErrUsageRestriction)
	})

	t.Run("don't respond if not on allowed team", func(t *testing.T) {
		e.ResetMocks(t)
		e.plugin.setConfiguration(makeConfig(Config{
			AllowedTeamIDs:        "someotherteam,someotherteam2",
			AllowPrivateChannels:  true,
			EnableUseRestrictions: true,
		}))
		e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
			Type:   model.ChannelTypeOpen,
			TeamId: "notallowedteam",
		}, nil)
		e.mockAPI.On("GetUser", "userid").Return(&model.User{
			Id: "userid",
		}, nil)

		err := e.plugin.handleMessages(standardPost)
		require.ErrorIs(t, err, ErrUsageRestriction)
	})

	t.Run("don't respond if in private channel and not allowed", func(t *testing.T) {
		e.ResetMocks(t)
		e.plugin.setConfiguration(makeConfig(Config{
			AllowedTeamIDs:        "teamid",
			AllowPrivateChannels:  false,
			EnableUseRestrictions: true,
		}))
		e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
			Type:   model.ChannelTypePrivate,
			TeamId: "teamid",
		}, nil)
		e.mockAPI.On("GetUser", "userid").Return(&model.User{
			Id: "userid",
		}, nil)

		err := e.plugin.handleMessages(standardPost)
		require.ErrorIs(t, err, ErrUsageRestriction)
	})

	t.Run("don't respond to bots", func(t *testing.T) {
		e.ResetMocks(t)
		e.plugin.setConfiguration(makeConfig(Config{
			EnableUseRestrictions: false,
		}))
		e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
			Type:   model.ChannelTypePrivate,
			TeamId: "teamid",
		}, nil)
		e.mockAPI.On("GetUser", "userid").Return(&model.User{
			Id:    "userid",
			IsBot: true,
		}, nil)

		err := e.plugin.handleMessages(standardPost)
		require.ErrorIs(t, err, ErrNoResponse)
	})
}

func TestHandleDMs(t *testing.T) {
	e := SetupTestEnvironment(t)
	defer e.Cleanup(t)

	standardPost := &model.Post{
		UserId:    "userid",
		ChannelId: "channelid",
		Message:   "whatever",
	}

	t.Run("don't respond to users that are not allowed", func(t *testing.T) {
		e.ResetMocks(t)
		e.plugin.setConfiguration(makeConfig(Config{
			OnlyUsersOnTeam:       "teamid",
			EnableUseRestrictions: true,
		}))
		e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
			Type:   model.ChannelTypeDirect,
			Name:   "botid__userid",
			TeamId: "teamid",
		}, nil)
		e.mockAPI.On("GetUser", "userid").Return(&model.User{
			Id: "userid",
		}, nil)
		e.mockAPI.On("HasPermissionToTeam", "userid", "teamid", model.PermissionViewTeam).Return(false)

		err := e.plugin.handleMessages(standardPost)
		require.ErrorIs(t, err, ErrUsageRestriction)
	})

	t.Run("don't respond to bots", func(t *testing.T) {
		e.ResetMocks(t)
		e.plugin.setConfiguration(makeConfig(Config{
			EnableUseRestrictions: false,
		}))
		e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
			Type:   model.ChannelTypeDirect,
			Name:   "botid__userid",
			TeamId: "teamid",
		}, nil)
		e.mockAPI.On("GetUser", "userid").Return(&model.User{
			Id:    "userid",
			IsBot: true,
		}, nil)

		err := e.plugin.handleMessages(standardPost)
		require.ErrorIs(t, err, ErrNoResponse)
	})
}
