// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-agents/v2/audit"
	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/conversation"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmapi"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmapi/mocks"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmtools"
	"github.com/mattermost/mattermost-plugin-agents/v2/store"
	"github.com/mattermost/mattermost-plugin-agents/v2/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// plantedWakeReason is distinctive wake-job content: if it ever shows up
// in a marshaled audit record, scheduler message content leaked into the trail.
const plantedWakeReason = "SENTINEL_WAKE_REASON_MUST_NOT_APPEAR_IN_AUDIT"

// wakeUserID is a well-formed ID: the wake's usage-restriction gate runs the
// ABAC checker, which denies malformed user IDs outright.
var wakeUserID = model.NewId()

func wakeConversationStore() (*loadedStateFlowStore, *store.Conversation) {
	convStore := newLoadedStateFlowStore()
	conv := &store.Conversation{
		ID:           "conv-id",
		UserID:       wakeUserID,
		BotID:        "bot-id",
		SystemPrompt: "system",
		Operation:    "conversation",
	}
	_ = convStore.CreateConversation(conv)
	return convStore, conv
}

func TestWakeJobFromProps(t *testing.T) {
	tests := []struct {
		name    string
		props   any
		want    WakeJob
		wantErr bool
	}{
		{
			name:  "typed struct from same-process schedule",
			props: WakeJob{PostID: "post-id", Reason: "cursor agent"},
			want:  WakeJob{PostID: "post-id", Reason: "cursor agent"},
		},
		{
			name:  "generic JSON map after restart",
			props: map[string]any{"post_id": "post-id", "reason": "cursor agent"},
			want:  WakeJob{PostID: "post-id", Reason: "cursor agent"},
		},
		{
			name:    "missing post id",
			props:   map[string]any{"reason": "cursor agent"},
			wantErr: true,
		},
		{
			name:    "non-object props",
			props:   "bogus",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := wakeJobFromProps(tt.props)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, job)
		})
	}
}

func TestIsPendingWake(t *testing.T) {
	tests := []struct {
		name    string
		job     WakeJob
		pending string
		kvErr   error
		want    bool
		wantErr bool
	}{
		{name: "job scheduled before tracking fires", job: WakeJob{PostID: "p"}, want: true},
		{name: "latest wake fires", job: WakeJob{PostID: "p", ConversationID: "conv-id"}, pending: "wait_wake_b", want: true},
		{name: "superseded wake is dropped", job: WakeJob{PostID: "p", ConversationID: "conv-id"}, pending: "wait_wake_a", want: false},
		{name: "missing marker is dropped", job: WakeJob{PostID: "p", ConversationID: "conv-id"}, kvErr: mmapi.ErrKVNotFound, want: false},
		{name: "kv failure is an error", job: WakeJob{PostID: "p", ConversationID: "conv-id"}, kvErr: errors.New("kv down"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mmClient := mocks.NewMockClient(t)
			if tt.job.ConversationID != "" {
				mmClient.On("KVGet", pendingWakeKeyPrefix+tt.job.ConversationID, mock.Anything).Run(func(args mock.Arguments) {
					if tt.kvErr == nil {
						*args.Get(1).(*string) = tt.pending
					}
				}).Return(tt.kvErr).Once()
			}

			c := &Conversations{mmClient: mmClient}
			got, err := c.isPendingWake(tt.job, "wait_wake_b")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAcquireResumeLease(t *testing.T) {
	origPoll := resumeLeasePollInterval
	resumeLeasePollInterval = time.Millisecond
	t.Cleanup(func() { resumeLeasePollInterval = origPoll })

	tests := []struct {
		name     string
		attempts []bool
		kvErr    error
		wantErr  bool
	}{
		{name: "free lease is acquired", attempts: []bool{true}},
		{name: "held lease is waited for", attempts: []bool{false, false, true}},
		{name: "kv failure is an error", kvErr: errors.New("kv down"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := resumeLeaseKeyPrefix + "conv-id"
			mmClient := mocks.NewMockClient(t)
			if tt.kvErr != nil {
				mmClient.On("KVCompareAndSetWithExpiry", key, nil, mock.Anything, resumeLeaseTTL).Return(false, tt.kvErr).Once()
			}
			for _, acquired := range tt.attempts {
				mmClient.On("KVCompareAndSetWithExpiry", key, nil, mock.Anything, resumeLeaseTTL).Return(acquired, nil).Once()
			}

			c := &Conversations{mmClient: mmClient}
			release, err := c.acquireResumeLease("conv-id")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			mmClient.On("KVCompareAndSet", key, mock.Anything, nil).Return(true, nil).Once()
			release()
		})
	}
}

func TestResumeLeaseRenewsWhileHeld(t *testing.T) {
	origRenew := resumeLeaseRenewInterval
	resumeLeaseRenewInterval = time.Millisecond
	t.Cleanup(func() { resumeLeaseRenewInterval = origRenew })

	key := resumeLeaseKeyPrefix + "conv-id"
	var leaseID string
	renewed := make(chan struct{}, 1)
	mmClient := mocks.NewMockClient(t)
	mmClient.On("KVCompareAndSetWithExpiry", key, nil, mock.Anything, resumeLeaseTTL).Run(func(args mock.Arguments) {
		leaseID = args.Get(2).(string)
	}).Return(true, nil).Once()
	isLeaseID := mock.MatchedBy(func(s string) bool { return s != "" })
	mmClient.On("KVCompareAndSetWithExpiry", key, isLeaseID, isLeaseID, resumeLeaseTTL).Run(func(args mock.Arguments) {
		require.Equal(t, leaseID, args.Get(1))
		require.Equal(t, leaseID, args.Get(2))
		select {
		case renewed <- struct{}{}:
		default:
		}
	}).Return(true, nil)
	mmClient.On("KVCompareAndSet", key, mock.Anything, nil).Return(true, nil).Once()

	c := &Conversations{mmClient: mmClient}
	release, err := c.acquireResumeLease("conv-id")
	require.NoError(t, err)
	select {
	case <-renewed:
	case <-time.After(5 * time.Second):
		t.Fatal("lease was not renewed while held")
	}
	release()
}

func TestRunWakeDropsWakeSupersededWhileWaitingForLease(t *testing.T) {
	origPoll := resumeLeasePollInterval
	resumeLeasePollInterval = time.Millisecond
	t.Cleanup(func() { resumeLeasePollInterval = origPoll })

	leaseKey := resumeLeaseKeyPrefix + "conv-id"
	mmClient := mocks.NewMockClient(t)
	mmClient.On("KVCompareAndSetWithExpiry", leaseKey, nil, mock.Anything, resumeLeaseTTL).Return(false, nil).Once()
	mmClient.On("KVCompareAndSetWithExpiry", leaseKey, nil, mock.Anything, resumeLeaseTTL).Return(true, nil).Once()
	// A newer wait was recorded while this wake waited on the running resume.
	mmClient.On("KVGet", pendingWakeKeyPrefix+"conv-id", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(1).(*string) = "wait_wake_newer"
	}).Return(nil).Once()
	mmClient.On("LogDebug", mock.Anything, mock.Anything).Return().Once()
	mmClient.On("KVCompareAndSet", leaseKey, mock.Anything, nil).Return(true, nil).Once()

	c := &Conversations{mmClient: mmClient}
	// No GetPost expectation: resuming would fail the mock.
	c.runWake(WakeJob{PostID: "bot-post-id", ConversationID: "conv-id"}, "wait_wake_older")
}

func TestResumeConversationWritesWakeTurnAndFollowsUp(t *testing.T) {
	convStore, conv := wakeConversationStore()
	userContent, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: "please watch the job"}})
	require.NoError(t, err)
	require.NoError(t, convStore.CreateTurn(&store.Turn{
		ID:             "user-1",
		ConversationID: conv.ID,
		Role:           "user",
		Content:        userContent,
		Sequence:       1,
	}))

	lm := &loadedStateLLM{}
	bot := loadedStateBot(lm)
	streamingService := &loadedStateStreamingService{}

	mockAPI := &plugintest.API{}
	pluginAPI := pluginapi.NewClient(mockAPI, nil)
	licenseChecker := enterprise.NewLicenseChecker(pluginAPI)
	botsService := bots.New(mockAPI, pluginAPI, licenseChecker, nil, nil, newPassthroughAccessChecker(), &http.Client{}, nil)
	botsService.SetBotsForTesting([]*bots.Bot{bot})

	user := &model.User{Id: wakeUserID, Username: "user"}
	channel := &model.Channel{Id: "dm-channel", Type: model.ChannelTypeDirect, Name: "bot-id__user-id"}
	post := &model.Post{Id: "bot-post-id", UserId: "bot-id", ChannelId: "dm-channel", RootId: "root-post-id"}
	post.AddProp(streaming.ConversationIDProp, conv.ID)

	var wakePost *model.Post
	mmClient := mocks.NewMockClient(t)
	mmClient.On("GetPost", "bot-post-id").Return(post, nil).Once()
	mmClient.On("GetUser", wakeUserID).Return(user, nil).Once()
	mmClient.On("GetChannel", "dm-channel").Return(channel, nil).Once()
	mmClient.On("HasPermissionToChannel", wakeUserID, "dm-channel", model.PermissionReadChannel).Return(true).Once()
	mmClient.On("CreatePost", mock.Anything).Run(func(args mock.Arguments) {
		wakePost = args.Get(0).(*model.Post)
		wakePost.Id = "wake-post-id"
	}).Return(nil).Once()
	mmClient.On("UpdatePost", mock.Anything).Return(nil).Once()
	mmClient.On("KVGet", mock.Anything, mock.Anything).Maybe().Return(nil)
	mmClient.On("LogDebug", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("LogError", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("LogWarn", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("GetConfig").Return(&model.Config{}).Maybe()
	loggedAudit := captureLogAuditRec(mockAPI)

	c := &Conversations{
		mmClient:         mmClient,
		contextBuilder:   loadedStateBuilder(t),
		bots:             botsService,
		convService:      conversation.NewService(convStore, nil, nil, nil),
		streamingService: streamingService,
	}

	rec := plugin.MakeAuditRecord("waitForAsyncWork", model.AuditStatusFail)
	ctx := audit.WithRecord(context.Background(), rec)
	require.NoError(t, c.ResumeConversation(ctx, WakeJob{
		PostID: "bot-post-id",
		Reason: plantedWakeReason,
	}))
	streamingService.waitForStreaming()

	// The wake streams onto a new bot post in the same thread; the original
	// post's finished response is left untouched.
	require.NotNil(t, wakePost)
	require.Equal(t, "root-post-id", wakePost.RootId)
	require.Equal(t, "dm-channel", wakePost.ChannelId)
	require.Equal(t, "bot-id", wakePost.UserId)
	require.Equal(t, conv.ID, wakePost.GetProp(streaming.ConversationIDProp))

	turns, err := convStore.GetTurnsForConversation(conv.ID)
	require.NoError(t, err)
	require.Len(t, turns, 2)
	require.Equal(t, "user", turns[1].Role)
	require.Nil(t, turns[1].PostID)

	var blocks []conversation.ContentBlock
	require.NoError(t, json.Unmarshal(turns[1].Content, &blocks))
	require.Equal(t, conversation.BlockTypeText, blocks[0].Type)
	require.Equal(t, mmtools.WakeUserMessage, blocks[0].Text)

	// The model-supplied reason must never be voiced as the user.
	require.Len(t, lm.requests, 1)
	for _, p := range lm.requests[0].Posts {
		require.NotContains(t, p.Message, plantedWakeReason)
	}
	// Nobody is present to answer user-interaction tools on a wake.
	require.False(t, lm.requests[0].Context.ToolCatalog.InteractiveUserPresent)

	assertAuditDoesNotContain(t, loggedAudit, rec, plantedWakeReason)
}

func TestResumeConversationInChannelIgnoresThreadPostsAfterRequester(t *testing.T) {
	convStore, conv := wakeConversationStore()
	rootPostID := "mention-post-id"
	conv.RootPostID = &rootPostID
	require.NoError(t, convStore.CreateConversation(conv))
	userContent, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: "@matty watch the job"}})
	require.NoError(t, err)
	require.NoError(t, convStore.CreateTurn(&store.Turn{
		ID: "user-1", ConversationID: conv.ID, PostID: &rootPostID, Role: "user", Content: userContent, Sequence: 1,
	}))
	replyPostID := "bot-post-id"
	replyContent, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: "waiting on the job"}})
	require.NoError(t, err)
	require.NoError(t, convStore.CreateTurn(&store.Turn{
		ID: "assistant-1", ConversationID: conv.ID, PostID: &replyPostID, Role: "assistant", Content: replyContent, Sequence: 2,
	}))

	lm := &loadedStateLLM{}
	bot := loadedStateBot(lm)
	streamingService := &loadedStateStreamingService{}

	mockAPI := &plugintest.API{}
	pluginAPI := pluginapi.NewClient(mockAPI, nil)
	licenseChecker := enterprise.NewLicenseChecker(pluginAPI)
	botsService := bots.New(mockAPI, pluginAPI, licenseChecker, nil, nil, newPassthroughAccessChecker(), &http.Client{}, nil)
	botsService.SetBotsForTesting([]*bots.Bot{bot})

	peerID := model.NewId()
	user := &model.User{Id: wakeUserID, Username: "user"}
	channel := &model.Channel{Id: "open-channel", Type: model.ChannelTypeOpen, TeamId: "team-id"}
	post := &model.Post{Id: replyPostID, UserId: "bot-id", ChannelId: channel.Id, RootId: rootPostID, CreateAt: 2000}
	post.AddProp(streaming.ConversationIDProp, conv.ID)
	thread := &model.PostList{
		Order: []string{rootPostID, replyPostID, "peer-post-id"},
		Posts: map[string]*model.Post{
			rootPostID:     {Id: rootPostID, UserId: wakeUserID, ChannelId: channel.Id, CreateAt: 1000, Message: "@matty watch the job"},
			replyPostID:    post,
			"peer-post-id": {Id: "peer-post-id", UserId: peerID, ChannelId: channel.Id, RootId: rootPostID, CreateAt: 3000, Message: "PEER_INSTRUCTION"},
		},
	}

	mmClient := mocks.NewMockClient(t)
	mmClient.On("GetPost", replyPostID).Return(post, nil).Once()
	mmClient.On("GetUser", wakeUserID).Return(user, nil)
	mmClient.On("GetUser", peerID).Return(&model.User{Id: peerID, Username: "mallory"}, nil).Maybe()
	mmClient.On("GetUser", "bot-id").Return(&model.User{Id: "bot-id", Username: "matty", IsBot: true}, nil).Maybe()
	mmClient.On("GetChannel", channel.Id).Return(channel, nil).Once()
	mmClient.On("HasPermissionToChannel", wakeUserID, channel.Id, model.PermissionReadChannel).Return(true).Once()
	mmClient.On("GetPostThread", rootPostID).Return(thread, nil).Once()
	mmClient.On("CreatePost", mock.Anything).Run(func(args mock.Arguments) {
		args.Get(0).(*model.Post).Id = "wake-post-id"
	}).Return(nil).Once()
	mmClient.On("UpdatePost", mock.Anything).Return(nil).Once()
	mmClient.On("KVGet", mock.Anything, mock.Anything).Maybe().Return(nil)
	mmClient.On("LogDebug", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("LogError", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("LogWarn", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("GetConfig").Return(&model.Config{}).Maybe()

	c := &Conversations{
		mmClient:         mmClient,
		contextBuilder:   loadedStateBuilder(t),
		bots:             botsService,
		convService:      conversation.NewService(convStore, nil, nil, nil),
		streamingService: streamingService,
	}

	require.NoError(t, c.ResumeConversation(context.Background(), WakeJob{PostID: replyPostID, Reason: "job"}))
	streamingService.waitForStreaming()

	require.Len(t, lm.requests, 1)
	var sent strings.Builder
	for _, p := range lm.requests[0].Posts {
		sent.WriteString(p.Message)
	}
	require.Contains(t, sent.String(), "waiting on the job")
	require.NotContains(t, sent.String(), "PEER_INSTRUCTION")
}

func TestResumeConversationFailsPlaceholderWhenConversationPropUpdateFails(t *testing.T) {
	convStore, conv := wakeConversationStore()
	userContent, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: "please watch the job"}})
	require.NoError(t, err)
	require.NoError(t, convStore.CreateTurn(&store.Turn{
		ID:             "user-1",
		ConversationID: conv.ID,
		Role:           "user",
		Content:        userContent,
		Sequence:       1,
	}))

	mockAPI := &plugintest.API{}
	pluginAPI := pluginapi.NewClient(mockAPI, nil)
	licenseChecker := enterprise.NewLicenseChecker(pluginAPI)
	botsService := bots.New(mockAPI, pluginAPI, licenseChecker, nil, nil, newPassthroughAccessChecker(), &http.Client{}, nil)
	botsService.SetBotsForTesting([]*bots.Bot{loadedStateBot(&loadedStateLLM{})})

	user := &model.User{Id: wakeUserID, Username: "user"}
	channel := &model.Channel{Id: "dm-channel", Type: model.ChannelTypeDirect, Name: "bot-id__user-id"}
	post := &model.Post{Id: "bot-post-id", UserId: "bot-id", ChannelId: "dm-channel", RootId: "root-post-id"}
	post.AddProp(streaming.ConversationIDProp, conv.ID)

	var wakePost *model.Post
	mmClient := mocks.NewMockClient(t)
	mmClient.On("GetPost", "bot-post-id").Return(post, nil).Once()
	mmClient.On("GetUser", wakeUserID).Return(user, nil).Once()
	mmClient.On("GetChannel", "dm-channel").Return(channel, nil).Once()
	mmClient.On("HasPermissionToChannel", wakeUserID, "dm-channel", model.PermissionReadChannel).Return(true).Once()
	mmClient.On("CreatePost", mock.Anything).Run(func(args mock.Arguments) {
		wakePost = args.Get(0).(*model.Post)
		wakePost.Id = "wake-post-id"
	}).Return(nil).Once()
	mmClient.On("UpdatePost", mock.Anything).Return(model.NewAppError("UpdatePost", "store.system.update.app_error", nil, "", http.StatusInternalServerError)).Once()
	mmClient.On("UpdatePost", mock.Anything).Return(nil).Once()
	mmClient.On("LogDebug", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("LogError", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("LogWarn", mock.Anything, mock.Anything).Maybe().Return()
	mmClient.On("GetConfig").Return(&model.Config{}).Maybe()
	loggedAudit := captureLogAuditRec(mockAPI)

	c := &Conversations{
		mmClient:       mmClient,
		contextBuilder: loadedStateBuilder(t),
		bots:           botsService,
		convService:    conversation.NewService(convStore, nil, nil, nil),
	}

	rec := plugin.MakeAuditRecord("waitForAsyncWork", model.AuditStatusFail)
	ctx := audit.WithRecord(context.Background(), rec)
	err = c.ResumeConversation(ctx, WakeJob{
		PostID: "bot-post-id",
		Reason: plantedWakeReason,
	})
	require.Error(t, err)
	require.NotNil(t, wakePost)
	require.Contains(t, wakePost.Message, "error occurred")
	assertAuditDoesNotContain(t, loggedAudit, rec, plantedWakeReason)

	turns, err := convStore.GetTurnsForConversation(conv.ID)
	require.NoError(t, err)
	require.Len(t, turns, 1)
}

func TestResumeConversationDrops(t *testing.T) {
	livePost := func(convID string) *model.Post {
		post := &model.Post{Id: "bot-post-id", UserId: "bot-id", ChannelId: "dm-channel"}
		post.AddProp(streaming.ConversationIDProp, convID)
		return post
	}

	dmChannel := &model.Channel{Id: "dm-channel", Type: model.ChannelTypeDirect, Name: "bot-id__user-id"}
	openChannel := &model.Channel{Id: "dm-channel", Type: model.ChannelTypeOpen, TeamId: "team-id"}

	tests := []struct {
		name string
		post func(convID string) *model.Post
		// user, when set, means lookups reach the authorization check.
		user       *model.User
		canRead    bool
		channel    *model.Channel
		botCfg     func(*llm.BotConfig)
		priorWakes int
	}{
		{
			name: "post missing",
			post: func(string) *model.Post { return nil },
		},
		{
			name: "post has no conversation prop",
			post: func(string) *model.Post {
				return &model.Post{Id: "bot-post-id", UserId: "bot-id", ChannelId: "dm-channel"}
			},
		},
		{
			name: "conversation missing",
			post: func(string) *model.Post {
				post := &model.Post{Id: "bot-post-id", UserId: "bot-id", ChannelId: "dm-channel"}
				post.AddProp(streaming.ConversationIDProp, "missing-conv")
				return post
			},
		},
		{
			name:    "requester deactivated",
			post:    livePost,
			user:    &model.User{Id: wakeUserID, DeleteAt: 1},
			canRead: true,
		},
		{
			name:    "requester lost channel access",
			post:    livePost,
			user:    &model.User{Id: wakeUserID},
			canRead: false,
		},
		{
			name:    "requester no longer allowed to use the agent in DM",
			post:    livePost,
			user:    &model.User{Id: wakeUserID},
			canRead: true,
			botCfg:  func(cfg *llm.BotConfig) { cfg.UserAccessLevel = llm.UserAccessLevelNone },
		},
		{
			name:    "agent no longer allowed in the channel",
			post:    livePost,
			user:    &model.User{Id: wakeUserID},
			canRead: true,
			channel: openChannel,
			botCfg:  func(cfg *llm.BotConfig) { cfg.ChannelAccessLevel = llm.ChannelAccessLevelNone },
		},
		{
			name:       "wake chain limit reached",
			post:       livePost,
			user:       &model.User{Id: wakeUserID},
			canRead:    true,
			priorWakes: maxConsecutiveWakes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convStore, conv := wakeConversationStore()
			convService := conversation.NewService(convStore, nil, nil, nil)
			for range tt.priorWakes {
				require.NoError(t, convService.AppendSyntheticUserTurn(conv.ID, mmtools.WakeUserMessage))
			}

			bot := loadedStateBot(&loadedStateLLM{})
			if tt.botCfg != nil {
				cfg := bot.GetConfig()
				tt.botCfg(&cfg)
				bot = bots.NewBot(cfg, bot.GetService(), bot.GetMMBot(), bot.LLM())
			}
			channel := dmChannel
			if tt.channel != nil {
				channel = tt.channel
			}

			mockAPI := &plugintest.API{}
			pluginAPI := pluginapi.NewClient(mockAPI, nil)
			licenseChecker := enterprise.NewLicenseChecker(pluginAPI)
			botsService := bots.New(mockAPI, pluginAPI, licenseChecker, nil, nil, newPassthroughAccessChecker(), &http.Client{}, nil)
			botsService.SetBotsForTesting([]*bots.Bot{bot})

			mmClient := mocks.NewMockClient(t)
			if post := tt.post(conv.ID); post != nil {
				mmClient.On("GetPost", "bot-post-id").Return(post, nil).Once()
			} else {
				mmClient.On("GetPost", "bot-post-id").Return(nil, model.NewAppError("GetPost", "not found", nil, "", 404)).Once()
			}
			if tt.user != nil {
				mmClient.On("GetUser", wakeUserID).Return(tt.user, nil).Once()
				mmClient.On("GetChannel", "dm-channel").Return(channel, nil).Once()
				if tt.user.DeleteAt == 0 {
					mmClient.On("HasPermissionToChannel", wakeUserID, "dm-channel", model.PermissionReadChannel).Return(tt.canRead).Once()
				}
			}
			mmClient.On("LogDebug", mock.Anything, mock.Anything).Return().Once()

			c := &Conversations{
				mmClient:    mmClient,
				bots:        botsService,
				convService: convService,
			}

			require.NoError(t, c.ResumeConversation(context.Background(), WakeJob{
				PostID: "bot-post-id",
				Reason: "cursor cloud agent",
			}))

			turns, err := convStore.GetTurnsForConversation(conv.ID)
			require.NoError(t, err)
			require.Len(t, turns, tt.priorWakes)
		})
	}
}

func TestConsecutiveWakeCount(t *testing.T) {
	type seedTurn struct {
		postID  string
		message string
	}
	tests := []struct {
		name  string
		turns []seedTurn
		want  int
	}{
		{name: "no turns", want: 0},
		{
			name:  "wakes after the requester's post",
			turns: []seedTurn{{postID: "p1", message: "watch it"}, {message: mmtools.WakeUserMessage}, {message: mmtools.WakeUserMessage}},
			want:  2,
		},
		{
			name:  "requester posting again resets the chain",
			turns: []seedTurn{{message: mmtools.WakeUserMessage}, {message: mmtools.WakeUserMessage}, {postID: "p2", message: "still there?"}, {message: mmtools.WakeUserMessage}},
			want:  1,
		},
		{
			name:  "other synthetic user turns are not wakes",
			turns: []seedTurn{{postID: "p1", message: "watch it"}, {message: "something else"}},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convStore, conv := wakeConversationStore()
			for i, seed := range tt.turns {
				content, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: seed.message}})
				require.NoError(t, err)
				turn := &store.Turn{ID: model.NewId(), ConversationID: conv.ID, Role: "user", Content: content, Sequence: i + 1}
				if seed.postID != "" {
					turn.PostID = &seed.postID
				}
				require.NoError(t, convStore.CreateTurn(turn))
			}

			c := &Conversations{convService: conversation.NewService(convStore, nil, nil, nil)}
			got, err := c.consecutiveWakeCount(conv.ID)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// captureLogAuditRec is the conversations-package analog of
// api.TestEnvironment.CaptureAuditRecords: this package has no HTTP
// TestEnvironment, so we listen on the plugin API mock directly.
func captureLogAuditRec(mockAPI *plugintest.API) *[]*model.AuditRecord {
	records := &[]*model.AuditRecord{}
	mockAPI.On("LogAuditRec", mock.Anything).Run(func(args mock.Arguments) {
		if rec, ok := args.Get(0).(*model.AuditRecord); ok {
			*records = append(*records, rec)
		}
	}).Maybe()
	return records
}

func assertAuditDoesNotContain(t *testing.T, logged *[]*model.AuditRecord, rec *model.AuditRecord, sentinel string) {
	t.Helper()
	toCheck := make([]*model.AuditRecord, 0, 1+len(*logged))
	if rec != nil {
		toCheck = append(toCheck, rec)
	}
	toCheck = append(toCheck, *logged...)
	for _, r := range toCheck {
		raw, err := json.Marshal(r)
		require.NoError(t, err)
		require.NotContains(t, string(raw), sentinel)
	}
	require.Empty(t, *logged)
}
