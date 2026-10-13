// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/conversation"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/i18n"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/llmcontext"
	"github.com/mattermost/mattermost-plugin-agents/v2/mcp"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmapi/mocks"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmtools"
	"github.com/mattermost/mattermost-plugin-agents/v2/store"
	"github.com/mattermost/mattermost-plugin-agents/v2/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const connectFlowJiraAuthURL = "https://mm.example.com/plugins/mattermost-ai/mcp/oauth/Jira/start"

// connectFlowMCPToolProvider supplies connected tools plus OAuth-needed
// failures, filtered by selection like the real client manager.
type connectFlowMCPToolProvider struct {
	tools      []llm.Tool
	authErrors []llm.ToolAuthError
}

func (p *connectFlowMCPToolProvider) GetToolsWithSelection(_ context.Context, _ mcp.CatalogRequest, selection mcp.ToolSelection) ([]llm.Tool, *mcp.Errors) {
	var tools []llm.Tool
	for _, tool := range p.tools {
		if selection.Allows(tool.ServerOrigin) {
			tools = append(tools, tool)
		}
	}
	var authErrors []llm.ToolAuthError
	for _, authErr := range p.authErrors {
		if selection.Allows(authErr.ServerOrigin) {
			authErrors = append(authErrors, authErr)
		}
	}
	if len(authErrors) == 0 {
		return tools, nil
	}
	return tools, &mcp.Errors{ToolAuthErrors: authErrors}
}

func newConnectFlowBuilder(t *testing.T, provider *connectFlowMCPToolProvider) *llmcontext.Builder {
	t.Helper()
	mockAPI := &plugintest.API{}
	mockLicenseState(mockAPI, true)
	mockAPI.On("GetTeam", "team-id").Return(&model.Team{Id: "team-id", Name: "team"}, nil).Maybe()
	return llmcontext.NewLLMContextBuilder(
		pluginapi.NewClient(mockAPI, nil),
		&channelFollowUpTestToolProvider{},
		provider,
		&channelFollowUpTestConfig{},
	)
}

func jiraAuthError() llm.ToolAuthError {
	return llm.ToolAuthError{
		ServerName:        "Jira",
		ServerOrigin:      "https://jira.example.com",
		AuthURL:           connectFlowJiraAuthURL,
		ServerDescription: "Jira issues",
		ToolNames:         []string{"get_issue"},
	}
}

func seedPendingConnectTurn(t *testing.T, convStore *loadedStateFlowStore, convID, postID string) {
	t.Helper()
	content, err := json.Marshal([]conversation.ContentBlock{{
		Type:            conversation.BlockTypeToolUse,
		ID:              "connect-1",
		Name:            mmtools.ConnectMCPServerToolName,
		Input:           json.RawMessage(`{"server":"Jira"}`),
		Status:          conversation.StatusPending,
		UserInteraction: llm.UserInteractionConnect,
		Shared:          new(false),
	}})
	require.NoError(t, err)
	require.NoError(t, convStore.CreateTurn(&store.Turn{
		ID:             "assistant-turn",
		ConversationID: convID,
		PostID:         &postID,
		Role:           "assistant",
		Content:        content,
		Sequence:       1,
	}))
}

// TestHandleToolCallConnectMCPServer covers the Connect card round-trip: an
// accept carries no answer and its result reflects the catalog rebuilt after
// the click, a skip records a decline, and only the requester may decide.
func TestHandleToolCallConnectMCPServer(t *testing.T) {
	dmChannel := &model.Channel{Id: "dm-channel", Type: model.ChannelTypeDirect, Name: "bot-id__user-id"}
	openChannel := &model.Channel{Id: "channel-id", TeamId: "team-id", Type: model.ChannelTypeOpen}

	cases := []struct {
		name                 string
		channel              *model.Channel
		userID               string
		acceptedIDs          []string
		stillUnconnected     bool
		wantErr              error
		wantStatus           string
		wantResult           string
		wantResultErr        bool
		wantFollowUpTool     string
		wantFollowUpNoTool   string
		wantFollowUpListsSrv bool
	}{
		{
			name:               "accepted and connected in DM exposes the new tools",
			channel:            dmChannel,
			userID:             "user-id",
			acceptedIDs:        []string{"connect-1"},
			wantStatus:         conversation.StatusSuccess,
			wantResult:         "User connected Jira. Its tools are now available. Use search_tools to find them and load_tool to load one before calling it.",
			wantFollowUpTool:   mcp.SearchToolsName,
			wantFollowUpNoTool: mmtools.ConnectMCPServerToolName,
		},
		{
			name:                 "accepted but still not connected in channel streams immediately",
			channel:              openChannel,
			userID:               "user-id",
			acceptedIDs:          []string{"connect-1"},
			stillUnconnected:     true,
			wantStatus:           conversation.StatusSuccess,
			wantResult:           "Jira is still not connected: the user did not finish connecting it. Do not perform actions that need Jira. Tell the user they can connect Jira and ask again.",
			wantFollowUpTool:     mmtools.ConnectMCPServerToolName,
			wantFollowUpListsSrv: true,
		},
		{
			name:                 "skipped records a decline and continues",
			channel:              openChannel,
			userID:               "user-id",
			acceptedIDs:          []string{},
			stillUnconnected:     true,
			wantStatus:           conversation.StatusRejected,
			wantResult:           "User declined to connect Jira",
			wantResultErr:        true,
			wantFollowUpListsSrv: true,
		},
		{
			name:             "another channel member cannot accept",
			channel:          openChannel,
			userID:           "other-user",
			acceptedIDs:      []string{"connect-1"},
			stillUnconnected: true,
			wantErr:          ErrNotRequester,
		},
		{
			name:        "another channel member cannot skip",
			channel:     openChannel,
			userID:      "other-user",
			acceptedIDs: []string{},
			wantErr:     ErrNotRequester,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			convStore, conv := loadedStateConversationStore()
			approvalPostID := "approval-post-id"
			seedPendingConnectTurn(t, convStore, conv.ID, approvalPostID)

			provider := &connectFlowMCPToolProvider{}
			if tc.stillUnconnected {
				provider.authErrors = []llm.ToolAuthError{jiraAuthError()}
			} else {
				provider.tools = []llm.Tool{loadedStateTool()}
			}

			mockAPI := &plugintest.API{}
			pluginAPI := pluginapi.NewClient(mockAPI, nil)
			licenseChecker := enterprise.NewLicenseChecker(pluginAPI)
			botsService := bots.New(mockAPI, pluginAPI, licenseChecker, nil, nil, newPassthroughAccessChecker(), &http.Client{}, nil)
			lm := &loadedStateLLM{}
			botsService.SetBotsForTesting([]*bots.Bot{loadedStateBot(lm)})

			mmClient := mocks.NewMockClient(t)
			mmClient.On("LogDebug", mock.Anything, mock.Anything).Maybe().Return()
			mmClient.On("GetUser", "user-id").Maybe().Return(&model.User{Id: "user-id", Username: "user"}, nil)
			mmClient.On("KVGet", mock.Anything, mock.Anything).Maybe().Return(nil)
			mmClient.On("GetConfig").Maybe().Return(&model.Config{})

			streamingService := &loadedStateStreamingService{}
			c := &Conversations{
				mmClient:         mmClient,
				contextBuilder:   newConnectFlowBuilder(t, provider),
				bots:             botsService,
				convService:      conversation.NewService(convStore, nil, nil, nil),
				streamingService: streamingService,
			}

			approvalPost := &model.Post{Id: approvalPostID, UserId: "bot-id"}
			approvalPost.AddProp(streaming.ConversationIDProp, conv.ID)

			// The webapp accepts a Connect card with no answer entry.
			err := c.HandleToolCall(context.Background(), tc.userID, approvalPost, tc.channel, tc.acceptedIDs, nil)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				turns, turnsErr := convStore.GetTurnsForConversation(conv.ID)
				require.NoError(t, turnsErr)
				require.Len(t, turns, 1)
				var unchanged []conversation.ContentBlock
				require.NoError(t, json.Unmarshal(turns[0].Content, &unchanged))
				assert.Equal(t, conversation.StatusPending, unchanged[0].Status)
				assert.Empty(t, lm.requests)
				return
			}
			require.NoError(t, err)
			streamingService.waitForStreaming()

			turns, err := convStore.GetTurnsForConversation(conv.ID)
			require.NoError(t, err)
			require.Len(t, turns, 2)

			var updatedBlocks []conversation.ContentBlock
			require.NoError(t, json.Unmarshal(turns[0].Content, &updatedBlocks))
			assert.Equal(t, tc.wantStatus, updatedBlocks[0].Status)
			assert.Equal(t, llm.UserInteractionConnect, updatedBlocks[0].UserInteraction)
			require.NotNil(t, updatedBlocks[0].Shared)
			assert.True(t, *updatedBlocks[0].Shared)

			var resultBlocks []conversation.ContentBlock
			require.NoError(t, json.Unmarshal(turns[1].Content, &resultBlocks))
			require.Len(t, resultBlocks, 1)
			assert.Equal(t, tc.wantResult, resultBlocks[0].Content)
			if tc.wantResultErr {
				assert.Equal(t, conversation.StatusError, resultBlocks[0].Status)
			} else {
				assert.Equal(t, conversation.StatusSuccess, resultBlocks[0].Status)
			}
			assert.NotNil(t, resultBlocks[0].DecidedAt, "connect results are terminal")

			require.Len(t, lm.requests, 1, "expected a follow-up LLM request")
			followUp := lm.requests[0]
			assert.Contains(t, requestToolResults(followUp), tc.wantResult)
			assert.Zero(t, countUserMessagesContaining(followUp.Posts, toolRejectionGuidance),
				"a skipped connect is not a tool rejection")
			if tc.wantFollowUpTool != "" {
				assert.NotNil(t, followUp.Context.Tools.GetTool(tc.wantFollowUpTool))
			}
			if tc.wantFollowUpNoTool != "" {
				assert.Nil(t, followUp.Context.Tools.GetTool(tc.wantFollowUpNoTool))
			}
			_, listed := followUp.Context.UnconnectedMCPServerByName("Jira")
			assert.Equal(t, tc.wantFollowUpListsSrv, listed)
		})
	}
}

type capturedEphemeral struct {
	userID string
	post   *model.Post
}

func connectToolCall(id, server string, status llm.ToolCallStatus) llm.ToolCall {
	return llm.ToolCall{
		ID:        id,
		Name:      mmtools.ConnectMCPServerToolName,
		Arguments: json.RawMessage(`{"server":"` + server + `"}`),
		Status:    status,
	}
}

// TestMCPConnectFallbackEphemeral pins the plain-link fallback: the
// requester gets one ephemeral link per server per conversation, built from
// the server-side start URL, and only for pending connect calls.
func TestMCPConnectFallbackEphemeral(t *testing.T) {
	githubURL := "https://mm.example.com/plugins/mattermost-ai/mcp/oauth/GitHub%20(Prod)/start"
	servers := []llm.UnconnectedMCPServer{
		{Name: "Jira", AuthURL: connectFlowJiraAuthURL},
		{Name: "GitHub (Prod)", AuthURL: githubURL},
		{Name: "No_URL*"},
	}

	cases := []struct {
		name       string
		priorTurns []conversation.ContentBlock
		events     [][]llm.ToolCall
		wantLinks  []string
	}{
		{
			name:      "pending connect sends one link",
			events:    [][]llm.ToolCall{{connectToolCall("c1", "Jira", llm.ToolCallStatusPending)}},
			wantLinks: []string{"To continue, connect **Jira**: [Connect Jira](" + connectFlowJiraAuthURL + ")"},
		},
		{
			name: "same server twice in one stream is linked once",
			events: [][]llm.ToolCall{
				{connectToolCall("c1", "Jira", llm.ToolCallStatusPending), connectToolCall("c2", "Jira", llm.ToolCallStatusPending)},
				{connectToolCall("c3", "Jira", llm.ToolCallStatusPending)},
			},
			wantLinks: []string{"To continue, connect **Jira**: [Connect Jira](" + connectFlowJiraAuthURL + ")"},
		},
		{
			name: "server already requested earlier in the conversation is not linked again",
			priorTurns: []conversation.ContentBlock{{
				Type: conversation.BlockTypeToolUse, ID: "earlier", Name: mmtools.ConnectMCPServerToolName,
				Input: json.RawMessage(`{"server":"Jira"}`), Status: conversation.StatusRejected, UserInteraction: llm.UserInteractionConnect,
			}},
			events: [][]llm.ToolCall{{connectToolCall("c1", "Jira", llm.ToolCallStatusPending)}},
		},
		{
			name: "current call already persisted still gets its link",
			priorTurns: []conversation.ContentBlock{{
				Type: conversation.BlockTypeToolUse, ID: "c1", Name: mmtools.ConnectMCPServerToolName,
				Input: json.RawMessage(`{"server":"Jira"}`), Status: conversation.StatusPending, UserInteraction: llm.UserInteractionConnect,
			}},
			events:    [][]llm.ToolCall{{connectToolCall("c1", "Jira", llm.ToolCallStatusPending)}},
			wantLinks: []string{"To continue, connect **Jira**: [Connect Jira](" + connectFlowJiraAuthURL + ")"},
		},
		{
			name: "a different server requested earlier does not suppress this one",
			priorTurns: []conversation.ContentBlock{{
				Type: conversation.BlockTypeToolUse, ID: "earlier", Name: mmtools.ConnectMCPServerToolName,
				Input: json.RawMessage(`{"server":"GitHub (Prod)"}`), Status: conversation.StatusSuccess, UserInteraction: llm.UserInteractionConnect,
			}},
			events:    [][]llm.ToolCall{{connectToolCall("c1", "Jira", llm.ToolCallStatusPending)}},
			wantLinks: []string{"To continue, connect **Jira**: [Connect Jira](" + connectFlowJiraAuthURL + ")"},
		},
		{
			name:      "server names are markdown-escaped and URLs cannot close the link",
			events:    [][]llm.ToolCall{{connectToolCall("c1", "GitHub (Prod)", llm.ToolCallStatusPending)}},
			wantLinks: []string{"To continue, connect **GitHub (Prod)**: [Connect GitHub (Prod)](https://mm.example.com/plugins/mattermost-ai/mcp/oauth/GitHub%20%28Prod%29/start)"},
		},
		{
			name:   "resolved or rejected calls are not linked",
			events: [][]llm.ToolCall{{connectToolCall("c1", "Jira", llm.ToolCallStatusRejected), connectToolCall("c2", "Jira", llm.ToolCallStatusSuccess)}},
		},
		{
			name:   "unlisted server is not linked",
			events: [][]llm.ToolCall{{connectToolCall("c1", "Confluence", llm.ToolCallStatusPending)}},
		},
		{
			name:   "server without a start URL is not linked",
			events: [][]llm.ToolCall{{connectToolCall("c1", "No_URL*", llm.ToolCallStatusPending)}},
		},
		{
			name:   "other pending tools are ignored",
			events: [][]llm.ToolCall{{{ID: "t1", Name: "jira__get_issue", Arguments: json.RawMessage(`{"server":"Jira"}`), Status: llm.ToolCallStatusPending}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			convStore, conv := loadedStateConversationStore()
			if tc.priorTurns != nil {
				content, err := json.Marshal(tc.priorTurns)
				require.NoError(t, err)
				require.NoError(t, convStore.CreateTurn(&store.Turn{
					ID: "prior-turn", ConversationID: conv.ID, Role: "assistant", Content: content, Sequence: 1,
				}))
			}

			var mu sync.Mutex
			var sent []capturedEphemeral
			mmClient := mocks.NewMockClient(t)
			mmClient.On("GetConfig").Maybe().Return(&model.Config{})
			mmClient.On("SendEphemeralPost", mock.Anything, mock.Anything).Maybe().Run(func(args mock.Arguments) {
				mu.Lock()
				defer mu.Unlock()
				sent = append(sent, capturedEphemeral{userID: args.String(0), post: args.Get(1).(*model.Post)})
			}).Return()

			c := &Conversations{
				mmClient:    mmClient,
				convService: conversation.NewService(convStore, nil, nil, nil),
				i18n:        i18n.Init(),
			}

			llmContext := &llm.Context{
				RequestingUser: &model.User{Id: "user-id", Locale: "en"},
				ToolCatalog:    llm.ToolCatalogContext{UnconnectedMCPServers: servers},
			}
			source := make(chan llm.TextStreamEvent, len(tc.events)+1)
			for _, calls := range tc.events {
				source <- llm.TextStreamEvent{Type: llm.EventTypeToolCalls, Value: calls}
			}
			source <- llm.TextStreamEvent{Type: llm.EventTypeEnd}
			close(source)

			responsePost := &model.Post{Id: "response-id", ChannelId: "channel-id", RootId: "root-id"}
			decorated := c.decorateStreamWithMCPConnectFallback(loadedStateBot(&loadedStateLLM{}), &llm.TextStreamResult{Stream: source}, responsePost, conv.ID, llmContext)

			forwarded := 0
			for range decorated.Stream {
				forwarded++
			}
			assert.Equal(t, len(tc.events)+1, forwarded, "every event must pass through unchanged")

			mu.Lock()
			defer mu.Unlock()
			links := make([]string, 0, len(sent))
			for _, s := range sent {
				assert.Equal(t, "user-id", s.userID, "only the requester receives the link")
				assert.Equal(t, "bot-id", s.post.UserId)
				assert.Equal(t, "channel-id", s.post.ChannelId)
				assert.Equal(t, "root-id", s.post.RootId)
				links = append(links, s.post.Message)
			}
			assert.Equal(t, nilIfEmptyStrings(tc.wantLinks), nilIfEmptyStrings(links))
		})
	}
}

func TestMCPConnectFallbackSkipsFlowsWithoutUnconnectedServers(t *testing.T) {
	c := &Conversations{mmClient: mocks.NewMockClient(t)}
	stream := &llm.TextStreamResult{Stream: make(chan llm.TextStreamEvent)}
	bot := loadedStateBot(&loadedStateLLM{})
	post := &model.Post{Id: "response-id"}

	cases := []struct {
		name       string
		llmContext *llm.Context
	}{
		{name: "nil context"},
		{name: "no requesting user", llmContext: &llm.Context{ToolCatalog: llm.ToolCatalogContext{UnconnectedMCPServers: []llm.UnconnectedMCPServer{{Name: "Jira"}}}}},
		{name: "nothing unconnected and no reconnect hook", llmContext: &llm.Context{RequestingUser: &model.User{Id: "user-id"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Same(t, stream, c.decorateStreamWithMCPConnectFallback(bot, stream, post, "conv-id", tc.llmContext))
		})
	}
}

func TestMCPConnectFallbackTopLevelPostThreadsUnderResponse(t *testing.T) {
	var sent *model.Post
	mmClient := mocks.NewMockClient(t)
	mmClient.On("SendEphemeralPost", "user-id", mock.Anything).Once().Run(func(args mock.Arguments) {
		sent = args.Get(1).(*model.Post)
	}).Return()

	c := &Conversations{mmClient: mmClient}
	c.sendMCPConnectFallback(loadedStateBot(&loadedStateLLM{}), &model.Post{Id: "response-id", ChannelId: "dm"}, &model.User{Id: "user-id"},
		llm.UnconnectedMCPServer{Name: "Jira_Cloud", AuthURL: connectFlowJiraAuthURL})

	require.NotNil(t, sent)
	assert.Equal(t, "response-id", sent.RootId)
	assert.Equal(t, `To continue, connect **Jira\_Cloud**: [Connect Jira\_Cloud](`+connectFlowJiraAuthURL+")", sent.Message)
}

func nilIfEmptyStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
