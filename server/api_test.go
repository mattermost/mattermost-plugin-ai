package main

import (
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
		"react":             "/post/postid/react",
		"feedback positive": "/post/postid/feedback/positive",
		"feedback negative": "/post/postid/feedback/negative",
		"summarize":         "/post/postid/summarize",
		"transcribe":        "/post/postid/transcribe",
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

func TestTextRouter(t *testing.T) {
	// This just makes gin not output a whole bunch of debug stuff.
	// maybe pipe this to the test log?
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	for urlName, url := range map[string]string{
		"simplify":            "/text/simplify",
		"change tone":         "/text/change_tone/test",
		"generic change text": "/text/ask_ai_change_text",
	} {

		for name, test := range map[string]struct {
			request        *http.Request
			expectedStatus int
			config         Config
			envSetup       func(e *TestEnvironment)
		}{
			"test user not allowed": {
				request:        httptest.NewRequest("POST", url, nil),
				expectedStatus: http.StatusForbidden,
				config: Config{
					EnableUseRestrictions: true,
					OnlyUsersOnTeam:       "someotherteam",
				},
				envSetup: func(e *TestEnvironment) {
					e.mockAPI.On("HasPermissionToTeam", "userid", "someotherteam", model.PermissionViewTeam).Return(false)
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

func TestAdminRouter(t *testing.T) {
	// This just makes gin not output a whole bunch of debug stuff.
	// maybe pipe this to the test log?
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	for urlName, url := range map[string]string{
		"feedback": "/admin/feedback",
	} {

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
		"summarize since": "/channel/channelid/summarize/since",
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
