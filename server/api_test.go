package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPostRouter(t *testing.T) {
	// This just makes gin not output a whole bunch of debug stuff.
	// maybe pipe this to the test log?
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	for urlName, url := range map[string]string{
		"react":                   "/post/postid/react",
		"summarize":               "/post/postid/summarize",
		"transcribe":              "/post/postid/transcribe/file/fileid",
		"summarize_transcription": "/post/postid/summarize_transcription",
		"stop":                    "/post/postid/stop",
		"regenerate":              "/post/postid/regenerate",
	} {
		for name, test := range map[string]struct {
			request        *http.Request
			expectedStatus int
			config         Config
			envSetup       func(e *TestEnvironment)
		}{
			"test no permission to channel": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: false,
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypeOpen,
						TeamId: "teamid",
					}, nil)
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(false)
				},
			},
			"test user not allowed": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					OnlyUsersOnTeam:       "someotherteam",
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypeOpen,
						TeamId: "teamid",
					}, nil)
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(true)
					e.mockAPI.On("HasPermissionToTeam", "userid", "someotherteam", model.PermissionViewTeam).Return(false)
				},
			},
			"not allowed team": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					AllowedTeamIDs:        "someteam",
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypeOpen,
						TeamId: "teamid",
					}, nil)
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(true)
				},
			},
			"not on private channels": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					AllowPrivateChannels:  false,
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(true)
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypePrivate,
						TeamId: "teamid",
					}, nil)
				},
			},
			"not on dms": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					AllowPrivateChannels:  false,
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(true)
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypeDirect,
						TeamId: "teamid",
					}, nil)
				},
			},
		} {
			t.Run(urlName+" "+name, func(t *testing.T) {
				e := SetupTestEnvironment(t)
				defer e.Cleanup(t)

				test.config.DefaultBotName = "ai"
				e.plugin.setConfiguration(makeConfig(test.config))

				e.mockAPI.On("GetPost", "postid").Return(&model.Post{
					ChannelId: "channelid",
				}, nil)
				e.mockAPI.On("LogError", mock.Anything).Maybe()

				test.envSetup(e)

				test.request.Header.Add("Mattermost-User-ID", "userid")
				recorder := httptest.NewRecorder()
				e.plugin.ServeHTTP(&plugin.Context{}, recorder, test.request)
				resp := recorder.Result()
				require.Equal(t, test.expectedStatus, resp.StatusCode)
			})
		}
	}
}

func TestAdminRouter(t *testing.T) {
	// This just makes gin not output a whole bunch of debug stuff.
	// maybe pipe this to the test log?
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	for urlName, url := range map[string]string{} {
		for name, test := range map[string]struct {
			request        *http.Request
			expectedStatus int
			config         Config
			envSetup       func(e *TestEnvironment)
		}{
			"only admins": {
				request:        httptest.NewRequest("GET", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: false,
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("HasPermissionTo", "userid", model.PermissionManageSystem).Return(false)
				},
			},
		} {
			t.Run(urlName+" "+name, func(t *testing.T) {
				e := SetupTestEnvironment(t)
				defer e.Cleanup(t)

				e.plugin.setConfiguration(makeConfig(test.config))

				e.mockAPI.On("LogError", mock.Anything).Maybe()

				test.envSetup(e)

				test.request.Header.Add("Mattermost-User-ID", "userid")
				recorder := httptest.NewRecorder()
				e.plugin.ServeHTTP(&plugin.Context{}, recorder, test.request)
				resp := recorder.Result()
				require.Equal(t, test.expectedStatus, resp.StatusCode)
			})
		}
	}
}

func TestChannelRouter(t *testing.T) {
	// This just makes gin not output a whole bunch of debug stuff.
	// maybe pipe this to the test log?
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	for urlName, url := range map[string]string{
		"summarize since": "/channel/channelid/since",
	} {
		for name, test := range map[string]struct {
			request        *http.Request
			expectedStatus int
			config         Config
			envSetup       func(e *TestEnvironment)
		}{
			"test no permission to channel": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: false,
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypeOpen,
						TeamId: "teamid",
					}, nil)
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(false)
				},
			},
			"test user not allowed": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					OnlyUsersOnTeam:       "someotherteam",
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypeOpen,
						TeamId: "teamid",
					}, nil)
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(true)
					e.mockAPI.On("HasPermissionToTeam", "userid", "someotherteam", model.PermissionViewTeam).Return(false)
				},
			},
			"not allowed team": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					AllowedTeamIDs:        "someteam",
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypeOpen,
						TeamId: "teamid",
					}, nil)
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(true)
				},
			},
			"not on private channels": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					AllowPrivateChannels:  false,
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(true)
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypePrivate,
						TeamId: "teamid",
					}, nil)
				},
			},
			"not on dms": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					AllowPrivateChannels:  false,
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("HasPermissionToChannel", "userid", "channelid", model.PermissionReadChannel).Return(true)
					e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{
						Id:     "channelid",
						Type:   model.ChannelTypeDirect,
						TeamId: "teamid",
					}, nil)
				},
			},
		} {
			t.Run(urlName+" "+name, func(t *testing.T) {
				e := SetupTestEnvironment(t)
				defer e.Cleanup(t)

				test.config.DefaultBotName = "ai"
				e.plugin.setConfiguration(makeConfig(test.config))

				e.mockAPI.On("LogError", mock.Anything).Maybe()

				test.envSetup(e)

				test.request.Header.Add("Mattermost-User-ID", "userid")
				recorder := httptest.NewRecorder()
				e.plugin.ServeHTTP(&plugin.Context{}, recorder, test.request)
				resp := recorder.Result()
				require.Equal(t, test.expectedStatus, resp.StatusCode)
			})
		}
	}
}

func TestPostConversation(t *testing.T) {
	// This just makes gin not output a whole bunch of debug stuff.
	// maybe pipe this to the test log?
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	url := "/conversation"

	newPayloadReader := func(conv []*model.Post) io.Reader {
		data, err := json.Marshal(conv)
		require.NoError(t, err)
		return bytes.NewReader(data)
	}

	for name, test := range map[string]struct {
		request        *http.Request
		expectedStatus int
		config         Config
		envSetup       func(e *TestEnvironment)
	}{
		"not a bot": {
			request:        httptest.NewRequest("POST", url, nil),
			expectedStatus: http.StatusForbidden,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(nil, &model.AppError{StatusCode: http.StatusNotFound})
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"failed to get bot": {
			request:        httptest.NewRequest("POST", url, nil),
			expectedStatus: http.StatusInternalServerError,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(nil, &model.AppError{StatusCode: http.StatusInternalServerError})
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"empty conv": {
			request:        httptest.NewRequest("POST", url, nil),
			expectedStatus: http.StatusBadRequest,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"missing channel": {
			request:        httptest.NewRequest("POST", url, newPayloadReader(nil)),
			expectedStatus: http.StatusBadRequest,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"channel not found": {
			request: httptest.NewRequest("POST", url, newPayloadReader([]*model.Post{
				{
					ChannelId: "channelid",
				},
			})),
			expectedStatus: http.StatusBadRequest,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("GetChannel", "channelid").Return(nil, &model.AppError{StatusCode: http.StatusNotFound})
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"failure to get channel": {
			request: httptest.NewRequest("POST", url, newPayloadReader([]*model.Post{
				{
					ChannelId: "channelid",
				},
			})),
			expectedStatus: http.StatusInternalServerError,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("GetChannel", "channelid").Return(nil, &model.AppError{StatusCode: http.StatusInternalServerError})
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"missing poster id": {
			request: httptest.NewRequest("POST", url, newPayloadReader([]*model.Post{
				{
					ChannelId: "channelid",
				},
			})),
			expectedStatus: http.StatusBadRequest,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{Id: "channelid"}, nil)
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"poster not found": {
			request: httptest.NewRequest("POST", url, newPayloadReader([]*model.Post{
				{
					ChannelId: "channelid",
					UserId:    "posterid",
				},
			})),
			expectedStatus: http.StatusBadRequest,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{Id: "channelid"}, nil)
				e.mockAPI.On("GetUser", "posterid").Return(nil, &model.AppError{StatusCode: http.StatusNotFound})
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"failure to get poster": {
			request: httptest.NewRequest("POST", url, newPayloadReader([]*model.Post{
				{
					ChannelId: "channelid",
					UserId:    "posterid",
				},
			})),
			expectedStatus: http.StatusInternalServerError,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{Id: "channelid"}, nil)
				e.mockAPI.On("GetUser", "posterid").Return(nil, &model.AppError{StatusCode: http.StatusInternalServerError})
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"not responding to ourselves": {
			request: httptest.NewRequest("POST", url, newPayloadReader([]*model.Post{
				{
					ChannelId: "channelid",
					UserId:    "botid",
					Message:   "@ai what time is it?",
				},
			})),
			expectedStatus: http.StatusBadRequest,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{Id: "channelid"}, nil)
				e.mockAPI.On("GetUser", "botid").Return(&model.User{Id: "botid"}, nil)
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
		"missing bot mention": {
			request: httptest.NewRequest("POST", url, newPayloadReader([]*model.Post{
				{
					ChannelId: "channelid",
					UserId:    "posterid",
				},
			})),
			expectedStatus: http.StatusBadRequest,
			config: Config{
				EnableUseRestrictions: false,
			},
			envSetup: func(e *TestEnvironment) {
				e.mockAPI.On("GetBot", "userid", false).Return(&model.Bot{UserId: "userid"}, nil)
				e.mockAPI.On("GetChannel", "channelid").Return(&model.Channel{Id: "channelid"}, nil)
				e.mockAPI.On("GetUser", "posterid").Return(&model.User{Id: "posterid"}, nil)
				e.mockAPI.On("LogError", mock.Anything).Once()
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := SetupTestEnvironment(t)
			defer e.Cleanup(t)

			test.config.DefaultBotName = "ai"
			e.plugin.setConfiguration(makeConfig(test.config))

			test.envSetup(e)

			test.request.Header.Add("Mattermost-User-ID", "userid")
			recorder := httptest.NewRecorder()
			e.plugin.ServeHTTP(&plugin.Context{}, recorder, test.request)
			resp := recorder.Result()
			require.Equal(t, test.expectedStatus, resp.StatusCode)
		})
	}
}
