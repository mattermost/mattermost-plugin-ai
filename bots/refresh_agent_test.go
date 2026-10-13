// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bots

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// botAccounts is the set of Mattermost bot accounts a refresh harness serves.
// Renames persist across calls, and every call hands out copies so the
// registry never shares a *model.Bot with the fake.
type botAccounts struct {
	mu       sync.Mutex
	byUserID map[string]*model.Bot
	getErr   *model.AppError
	patchErr *model.AppError
	// listGate, when set, makes the next bot list signal listEntered and then
	// wait for listGate to close.
	listGate    chan struct{}
	listEntered chan struct{}
}

func (a *botAccounts) list(*model.BotGetOptions) []*model.Bot {
	a.mu.Lock()
	gate, entered := a.listGate, a.listEntered
	a.listGate, a.listEntered = nil, nil
	out := make([]*model.Bot, 0, len(a.byUserID))
	for _, account := range a.byUserID {
		out = append(out, account.Clone())
	}
	a.mu.Unlock()
	if gate != nil {
		close(entered)
		<-gate
	}
	return out
}

func (a *botAccounts) get(userID string, _ bool) (*model.Bot, *model.AppError) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.getErr != nil {
		return nil, a.getErr
	}
	account, ok := a.byUserID[userID]
	if !ok {
		return nil, model.NewAppError("GetBot", "store.sql_bot.get.missing.app_error", nil, "", http.StatusNotFound)
	}
	return account.Clone(), nil
}

func (a *botAccounts) patch(userID string, patch *model.BotPatch) (*model.Bot, *model.AppError) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.patchErr != nil {
		return nil, a.patchErr
	}
	account, ok := a.byUserID[userID]
	if !ok {
		return nil, model.NewAppError("PatchBot", "store.sql_bot.get.missing.app_error", nil, "", http.StatusNotFound)
	}
	account.Patch(patch)
	return account.Clone(), nil
}

func (a *botAccounts) userByUsername(username string) (*model.User, *model.AppError) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, account := range a.byUserID {
		if account.Username == username {
			return &model.User{Id: account.UserId, Username: account.Username, IsBot: true}, nil
		}
	}
	return nil, model.NewAppError("GetUserByUsername", "app.user.missing_account.const", nil, "", http.StatusNotFound)
}

func (a *botAccounts) username(userID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.byUserID[userID].Username
}

// newRefreshAgentHarness returns an MMBots backed by store and by plugin-owned
// bot accounts named after the store's agents. It never expects a bot account
// to be created.
func newRefreshAgentHarness(t *testing.T, store *stubAgentStore) (*MMBots, *botAccounts) {
	t.Helper()

	accounts := &botAccounts{byUserID: map[string]*model.Bot{}}
	for _, agent := range store.agents {
		accounts.byUserID[agent.BotUserID] = &model.Bot{UserId: agent.BotUserID, Username: agent.Name, OwnerId: botOwnerID}
	}

	mockAPI := &plugintest.API{}
	client := pluginapi.NewClient(mockAPI, nil)
	mockAPI.On("GetConfig").Return(&model.Config{}).Maybe()
	mockAPI.On("GetLicense").Return(&model.License{SkuShortName: model.LicenseShortSkuEnterprise, Features: &model.Features{}}).Maybe()
	mockAPI.On("GetBots", mock.AnythingOfType("*model.BotGetOptions")).Return(accounts.list, nil).Maybe()
	mockAPI.On("GetBot", mock.AnythingOfType("string"), true).Return(accounts.get).Maybe()
	mockAPI.On("PatchBot", mock.AnythingOfType("string"), mock.AnythingOfType("*model.BotPatch")).Return(accounts.patch).Maybe()
	mockAPI.On("UpdateBotActive", mock.AnythingOfType("string"), mock.AnythingOfType("bool")).Return(&model.Bot{}, nil).Maybe()
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{LastPictureUpdate: 1}, nil).Maybe()
	mockAPI.On("GetUserByUsername", mock.AnythingOfType("string")).Return(accounts.userByUsername).Maybe()
	mockAPI.On("KVSetWithOptions", mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil).Maybe()
	mockAPI.On("KVDelete", mock.AnythingOfType("string")).Return(nil).Maybe()
	allowBotsLogging(mockAPI)

	services := []llm.ServiceConfig{{ID: "svc1", Type: llm.ServiceTypeOpenAI, APIKey: "k", DefaultModel: "gpt-4o"}}
	mmBots := New(mockAPI, client, enterprise.NewLicenseChecker(client), &mockConfig{services: services}, store, newPassthroughAccessChecker(), &http.Client{}, nil)
	t.Cleanup(mmBots.ShutdownServiceLLMs)
	return mmBots, accounts
}

func renameTestAgents() *stubAgentStore {
	return &stubAgentStore{agents: []llm.BotConfig{
		{ID: "a1", BotUserID: "bot-1", Name: "old-name", DisplayName: "Agent", ServiceID: "svc1"},
		{ID: "a2", BotUserID: "bot-2", Name: "steady", DisplayName: "Steady", ServiceID: "svc1"},
	}}
}

func TestRefreshAgentServesRenamedAgentUnderItsNewNameOnly(t *testing.T) {
	testCases := []struct {
		name           string
		failNextEnsure bool
	}{
		{name: "next full rebuild succeeds"},
		{name: "next full rebuild fails", failNextEnsure: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := renameTestAgents()
			mmBots, accounts := newRefreshAgentHarness(t, store)
			require.NoError(t, mmBots.EnsureBots())
			before := mmBots.GetBotMentioned("@old-name hi")
			require.NotNil(t, before)
			steady := mmBots.GetBotByID("bot-2")
			require.NotNil(t, steady)

			store.agents[0].Name = "new-name"
			require.NoError(t, mmBots.RefreshAgent("a1"))

			if tc.failNextEnsure {
				store.listErr = errors.New("database unavailable")
			}
			mmBots.ForceRefreshOnNextEnsure()
			ensureErr := mmBots.EnsureBots()
			if tc.failNextEnsure {
				require.Error(t, ensureErr)
			} else {
				require.NoError(t, ensureErr)
			}

			mentioned := mmBots.GetBotMentioned("@new-name hi")
			require.NotNil(t, mentioned)
			assert.Equal(t, "a1", mentioned.GetConfig().ID)
			assert.Equal(t, "bot-1", mentioned.GetMMBot().UserId)
			assert.Equal(t, "new-name", mentioned.GetMMBot().Username)
			assert.Nil(t, mmBots.GetBotMentioned("@old-name hi"), "the agent must not answer to its previous name")
			assert.Nil(t, mmBots.GetBotByUsername("old-name"), "the previous name must not select the agent")
			require.NotNil(t, mmBots.GetBotByUsername("new-name"))
			assert.Equal(t, "a1", mmBots.GetBotByUsername("new-name").GetConfig().ID)
			assert.Equal(t, "new-name", accounts.username("bot-1"))
			assert.Len(t, mmBots.GetAllBots(), 2)

			if tc.failNextEnsure {
				assert.Same(t, steady, mmBots.GetBotByID("bot-2"), "other bots must be left untouched")
			} else {
				require.NotNil(t, mmBots.GetBotByID("bot-2"))
			}
			assert.Equal(t, "old-name", before.GetConfig().Name, "the previous bot must not be mutated in place")
			assert.Equal(t, "old-name", before.GetMMBot().Username, "the previous bot account must not be mutated in place")
		})
	}
}

func TestRefreshAgentRemovesAgentItCannotRebuild(t *testing.T) {
	testCases := []struct {
		name string
		// breakRefresh runs after the stored agent is renamed.
		breakRefresh func(store *stubAgentStore, accounts *botAccounts)
		expectErr    bool
		// transient failures are recovered by the next full rebuild once
		// cleared.
		transient bool
	}{
		{
			name: "bot account cannot be patched",
			breakRefresh: func(_ *stubAgentStore, accounts *botAccounts) {
				accounts.patchErr = model.NewAppError("PatchBot", "app.user.save.username_exists.app_error", nil, "", http.StatusBadRequest)
			},
			expectErr: true,
			transient: true,
		},
		{
			name: "bot account cannot be fetched",
			breakRefresh: func(_ *stubAgentStore, accounts *botAccounts) {
				accounts.getErr = model.NewAppError("GetBot", "store.sql_bot.get.app_error", nil, "", http.StatusInternalServerError)
			},
			expectErr: true,
			transient: true,
		},
		{
			name: "agents cannot be listed",
			breakRefresh: func(store *stubAgentStore, _ *botAccounts) {
				store.listErr = errors.New("database unavailable")
			},
			expectErr: true,
			transient: true,
		},
		{
			name: "agent was deleted",
			breakRefresh: func(store *stubAgentStore, _ *botAccounts) {
				store.agents = store.agents[1:]
			},
		},
		{
			name: "agent became inactive",
			breakRefresh: func(store *stubAgentStore, _ *botAccounts) {
				store.agents[0].ServiceID = "missing-service"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := renameTestAgents()
			mmBots, accounts := newRefreshAgentHarness(t, store)
			require.NoError(t, mmBots.EnsureBots())
			require.NotNil(t, mmBots.GetBotByID("bot-1"))
			steady := mmBots.GetBotByID("bot-2")
			require.NotNil(t, steady)

			store.agents[0].Name = "new-name"
			tc.breakRefresh(store, accounts)

			err := mmBots.RefreshAgent("a1")
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Nil(t, mmBots.GetBotByID("bot-1"), "an agent that cannot be rebuilt must not keep its previous entry")
			assert.Nil(t, mmBots.GetBotMentioned("@old-name hi"))
			assert.Nil(t, mmBots.GetBotMentioned("@new-name hi"))
			assert.Same(t, steady, mmBots.GetBotByID("bot-2"), "other bots must be left untouched")
			assert.Len(t, mmBots.GetAllBots(), 1)

			store.listErr = nil
			accounts.getErr = nil
			accounts.patchErr = nil
			require.NoError(t, mmBots.EnsureBots())
			restored := mmBots.GetBotByID("bot-1")
			if !tc.transient {
				assert.Nil(t, restored)
				return
			}
			require.NotNil(t, restored, "the next full rebuild must retry the agent")
			assert.Equal(t, "new-name", restored.GetMMBot().Username)
		})
	}
}

func TestRefreshAgentWaitsForInFlightRebuild(t *testing.T) {
	store := renameTestAgents()
	mmBots, accounts := newRefreshAgentHarness(t, store)
	require.NoError(t, mmBots.EnsureBots())

	release := make(chan struct{})
	entered := make(chan struct{})
	accounts.mu.Lock()
	accounts.listGate, accounts.listEntered = release, entered
	accounts.mu.Unlock()

	mmBots.ForceRefreshOnNextEnsure()
	ensureDone := make(chan error, 1)
	go func() { ensureDone <- mmBots.EnsureBots() }()
	<-entered

	// The rebuild has read the agents under their previous names.
	store.agents[0].Name = "new-name"
	refreshDone := make(chan error, 1)
	go func() { refreshDone <- mmBots.RefreshAgent("a1") }()

	select {
	case <-refreshDone:
		t.Fatal("RefreshAgent must wait for the in-flight rebuild to swap its registry")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-ensureDone)
	require.NoError(t, <-refreshDone)

	assert.Nil(t, mmBots.GetBotMentioned("@old-name hi"), "the stale rebuild must not override the refresh")
	assert.Nil(t, mmBots.GetBotByUsername("old-name"), "the stale rebuild must not override the refresh")
	mentioned := mmBots.GetBotMentioned("@new-name hi")
	require.NotNil(t, mentioned)
	assert.Equal(t, "new-name", mentioned.GetConfig().Name)
}

func TestRefreshAgentConcurrentWithReaders(t *testing.T) {
	store := renameTestAgents()
	mmBots, _ := newRefreshAgentHarness(t, store)
	require.NoError(t, mmBots.EnsureBots())

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				for _, bot := range mmBots.GetAllBots() {
					_ = bot.GetConfig().Name
					_ = bot.GetMMBot().Username
				}
				_ = mmBots.GetBotMentioned("@old-name hi")
			}
		})
	}

	for range 20 {
		require.NoError(t, mmBots.RefreshAgent("a1"))
		mmBots.ForceRefreshOnNextEnsure()
		require.NoError(t, mmBots.EnsureBots())
	}
	close(stop)
	readers.Wait()

	assert.Len(t, mmBots.GetAllBots(), 2)
}
