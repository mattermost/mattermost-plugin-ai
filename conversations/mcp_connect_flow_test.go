// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"encoding/json"
	"errors"
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

// connectFlowConfig configures the Jira server whose origin the connect
// result looks for in the rebuilt catalog.
type connectFlowConfig struct {
	channelFollowUpTestConfig
}

func (c *connectFlowConfig) MCP() mcp.Config {
	return mcp.Config{Servers: []mcp.ServerConfig{{Name: "Jira", BaseURL: "https://jira.example.com", Enabled: true}}}
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
		Name:            llm.ConnectMCPServerToolName,
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

type connectFlowCatalog int

// Catalog states the rebuilt tool context can be in after the click.
const (
	connectFlowStillUnconnected connectFlowCatalog = iota
	connectFlowConnected
	connectFlowUnavailable
)

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
		catalog              connectFlowCatalog
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
			catalog:            connectFlowConnected,
			wantStatus:         conversation.StatusSuccess,
			wantResult:         "User connected Jira. Its tools are now available. Use search_tools to find them and load_tool to load one before calling it.",
			wantFollowUpTool:   mcp.SearchToolsName,
			wantFollowUpNoTool: llm.ConnectMCPServerToolName,
		},
		{
			name:                 "accepted but still not connected records an error and streams immediately",
			channel:              openChannel,
			userID:               "user-id",
			acceptedIDs:          []string{"connect-1"},
			catalog:              connectFlowStillUnconnected,
			wantStatus:           conversation.StatusError,
			wantResult:           "Jira is still not connected: the user did not finish connecting it. Do not perform actions that need Jira. Tell the user they can connect Jira and ask again.",
			wantResultErr:        true,
			wantFollowUpTool:     llm.ConnectMCPServerToolName,
			wantFollowUpListsSrv: true,
		},
		{
			name:          "accepted but the server's tools are not available records an error",
			channel:       dmChannel,
			userID:        "user-id",
			acceptedIDs:   []string{"connect-1"},
			catalog:       connectFlowUnavailable,
			wantStatus:    conversation.StatusError,
			wantResult:    "Jira is not available: none of its tools can be used in this conversation. Do not perform actions that need Jira. Tell the user Jira is unavailable here.",
			wantResultErr: true,
		},
		{
			name:                 "skipped records a decline and continues",
			channel:              openChannel,
			userID:               "user-id",
			acceptedIDs:          []string{},
			catalog:              connectFlowStillUnconnected,
			wantStatus:           conversation.StatusRejected,
			wantResult:           "User declined to connect Jira",
			wantResultErr:        true,
			wantFollowUpListsSrv: true,
		},
		{
			name:        "another channel member cannot accept",
			channel:     openChannel,
			userID:      "other-user",
			acceptedIDs: []string{"connect-1"},
			catalog:     connectFlowStillUnconnected,
			wantErr:     ErrNotRequester,
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
			switch tc.catalog {
			case connectFlowConnected:
				provider.tools = []llm.Tool{loadedStateTool()}
			case connectFlowStillUnconnected:
				provider.authErrors = []llm.ToolAuthError{jiraAuthError()}
			case connectFlowUnavailable:
				provider.tools = []llm.Tool{channelFollowUpTestMCPTool("github__create_pr", "https://github.example.com", "open a PR")}
			default:
				t.Fatalf("unhandled catalog %d", tc.catalog)
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
				configProvider:   &connectFlowConfig{},
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

// An MCP tool whose authorization lapses mid-turn makes the tool loop offer
// ConnectMCPServer for its server before the next completion request.
func TestToolLoopOffersReconnectWhenAuthorizationLapses(t *testing.T) {
	const origin = "https://jira.example.com"
	convStore, conv := loadedStateConversationStore()
	authErr := jiraAuthError()
	authErr.Error = errors.New("401 unauthorized")
	jiraTool := channelFollowUpTestMCPTool("jira__get_issue", origin, "fetch Jira issue details")
	jiraTool.Resolver = func(context.Context, *llm.Context, llm.ToolArgumentGetter) (string, error) {
		return "", &mcp.ReconnectNeededError{Auth: authErr}
	}

	builder := newChannelFollowUpTestBuilder(t, []llm.Tool{jiraTool}, &channelFollowUpTestConfig{})
	lm := &dynamicWorkflowLLM{}
	bot := loadedStateBot(lm)
	llmContext := builder.BuildLLMContextUserRequest(
		bot,
		&model.User{Id: "user-id", Username: "user", Locale: "en"},
		&model.Channel{Id: "dm-channel", Type: model.ChannelTypeDirect, Name: "bot-id__user-id"},
		builder.WithLLMContextInteractive(),
		builder.WithLLMContextTools(context.Background(), bot),
	)
	require.False(t, llmContext.IsToolCallable(llm.ConnectMCPServerToolName))
	c := &Conversations{
		convService:       conversation.NewService(convStore, nil, nil, nil),
		toolPolicyChecker: mapPolicyChecker{origin: {"get_issue": {policy: mcp.ToolPolicyAutoRunInDM, enabled: true}}},
	}

	streamResult, err := c.ProcessDMRequest(context.Background(), conv.ID, lm, llmContext, 0)
	require.NoError(t, err)
	_, err = streamResult.Stream.ReadAll()
	require.NoError(t, err)

	require.Len(t, lm.requests, 4)
	results := requestToolResults(lm.requests[3])
	require.NotEmpty(t, results)
	lapsed := results[len(results)-1]
	assert.Contains(t, lapsed, "Jira")
	assert.Contains(t, lapsed, llm.ConnectMCPServerToolName)
	assert.True(t, llmContext.IsToolCallable(llm.ConnectMCPServerToolName))
	_, listed := llmContext.UnconnectedMCPServerByName("Jira")
	assert.True(t, listed)
}

// pendingBatchLLM answers each completion request with the next scripted
// tool-call batch, then with plain text.
type pendingBatchLLM struct {
	mu      sync.Mutex
	batches [][]llm.ToolCall
}

func (l *pendingBatchLLM) ChatCompletion(context.Context, llm.CompletionRequest, ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.batches) == 0 {
		return dynamicWorkflowStream(llm.TextStreamEvent{Type: llm.EventTypeText, Value: "done"}), nil
	}
	batch := l.batches[0]
	l.batches = l.batches[1:]
	return dynamicWorkflowStream(llm.TextStreamEvent{Type: llm.EventTypeToolCalls, Value: batch}), nil
}

func (l *pendingBatchLLM) ChatCompletionNoStream(ctx context.Context, request llm.CompletionRequest, opts ...llm.LanguageModelOption) (string, error) {
	result, err := l.ChatCompletion(ctx, request, opts...)
	if err != nil {
		return "", err
	}
	return result.ReadAll()
}

func (l *pendingBatchLLM) CountTokens(context.Context, llm.CompletionRequest, ...llm.LanguageModelOption) (int, error) {
	return 0, llm.ErrUnsupportedTokenCount
}
func (l *pendingBatchLLM) InputTokenLimit() int  { return 100000 }
func (l *pendingBatchLLM) OutputTokenLimit() int { return 8192 }

func connectToolCall(id, server string) llm.ToolCall {
	return llm.ToolCall{
		ID:        id,
		Name:      llm.ConnectMCPServerToolName,
		Arguments: json.RawMessage(`{"server":"` + server + `"}`),
	}
}

type mcpConnectFallbackRun struct {
	convID string
	batch  []llm.ToolCall
}

// TestMCPConnectFallbackEphemeral pins the plain-link fallback: when the tool
// loop stops on a pending ConnectMCPServer call, the requester gets one
// ephemeral link per server per conversation, threaded where the
// conversation lives and built from the server-side start URL.
func TestMCPConnectFallbackEphemeral(t *testing.T) {
	githubURL := "https://mm.example.com/plugins/mattermost-ai/mcp/oauth/GitHub%20(Prod)/start"
	servers := []llm.UnconnectedMCPServer{
		{Name: "Jira", AuthURL: connectFlowJiraAuthURL},
		{Name: "GitHub (Prod)", AuthURL: githubURL},
	}
	jiraLink := "To continue, connect **Jira**: [Connect Jira](" + connectFlowJiraAuthURL + ")"

	cases := []struct {
		name      string
		runs      []mcpConnectFallbackRun
		kvFails   bool
		wantLinks []string
	}{
		{
			name:      "pending connect sends one link",
			runs:      []mcpConnectFallbackRun{{convID: "conv-a", batch: []llm.ToolCall{connectToolCall("c1", "Jira")}}},
			wantLinks: []string{jiraLink},
		},
		{
			name:      "same server twice in one batch is linked once",
			runs:      []mcpConnectFallbackRun{{convID: "conv-a", batch: []llm.ToolCall{connectToolCall("c1", "Jira"), connectToolCall("c2", "Jira")}}},
			wantLinks: []string{jiraLink},
		},
		{
			name: "server already linked earlier in the conversation is not linked again",
			runs: []mcpConnectFallbackRun{
				{convID: "conv-a", batch: []llm.ToolCall{connectToolCall("c1", "Jira")}},
				{convID: "conv-a", batch: []llm.ToolCall{connectToolCall("c2", "Jira")}},
			},
			wantLinks: []string{jiraLink},
		},
		{
			name: "each conversation gets its own link",
			runs: []mcpConnectFallbackRun{
				{convID: "conv-a", batch: []llm.ToolCall{connectToolCall("c1", "Jira")}},
				{convID: "conv-b", batch: []llm.ToolCall{connectToolCall("c2", "Jira")}},
			},
			wantLinks: []string{jiraLink, jiraLink},
		},
		{
			name:      "server names are markdown-escaped and URLs cannot close the link",
			runs:      []mcpConnectFallbackRun{{convID: "conv-a", batch: []llm.ToolCall{connectToolCall("c1", "GitHub (Prod)")}}},
			wantLinks: []string{"To continue, connect **GitHub (Prod)**: [Connect GitHub (Prod)](https://mm.example.com/plugins/mattermost-ai/mcp/oauth/GitHub%20%28Prod%29/start)"},
		},
		{
			name: "other pending tools are ignored",
			runs: []mcpConnectFallbackRun{{convID: "conv-a", batch: []llm.ToolCall{
				{ID: "t1", Name: "jira__get_issue", ServerOrigin: "https://jira.example.com", Arguments: json.RawMessage(`{"server":"Jira"}`)},
			}}},
		},
		{
			name:    "no link when the once-per-conversation claim cannot be recorded",
			runs:    []mcpConnectFallbackRun{{convID: "conv-a", batch: []llm.ToolCall{connectToolCall("c1", "Jira")}}},
			kvFails: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			convStore := newLoadedStateFlowStore()
			for _, id := range []string{"conv-a", "conv-b"} {
				require.NoError(t, convStore.CreateConversation(&store.Conversation{
					ID: id, UserID: "user-id", BotID: "bot-id", ChannelID: new("channel-" + id), RootPostID: new("root-" + id),
					SystemPrompt: "system", Operation: "conversation",
				}))
			}

			var mu sync.Mutex
			kv := map[string]bool{}
			var sent []capturedEphemeral
			mmClient := mocks.NewMockClient(t)
			mmClient.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
			kvCall := mmClient.On("KVCompareAndSet", mock.Anything, nil, true).Maybe()
			if tc.kvFails {
				kvCall.Return(false, errors.New("kv unavailable"))
			} else {
				kvCall.Return(func(key string, _, _ any) bool {
					mu.Lock()
					defer mu.Unlock()
					if kv[key] {
						return false
					}
					kv[key] = true
					return true
				}, nil)
			}
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

			for _, run := range tc.runs {
				tools := llm.NewToolStore()
				tools.AddTools([]llm.Tool{mmtools.NewConnectMCPServerTool(servers), loadedStateTool()})
				llmContext := &llm.Context{
					RequestingUser: &model.User{Id: "user-id", Locale: "en"},
					Tools:          tools,
					ToolCatalog:    llm.ToolCatalogContext{UnconnectedMCPServers: servers},
				}
				result, err := c.ProcessDMRequest(context.Background(), run.convID, &pendingBatchLLM{batches: [][]llm.ToolCall{run.batch}}, llmContext, 0)
				require.NoError(t, err)
				_, err = result.Stream.ReadAll()
				require.NoError(t, err)
			}

			mu.Lock()
			defer mu.Unlock()
			links := make([]string, 0, len(sent))
			for i, s := range sent {
				convID := tc.runs[min(i, len(tc.runs)-1)].convID
				assert.Equal(t, "user-id", s.userID, "only the requester receives the link")
				assert.Equal(t, "bot-id", s.post.UserId)
				assert.Equal(t, "channel-"+convID, s.post.ChannelId)
				assert.Equal(t, "root-"+convID, s.post.RootId)
				links = append(links, s.post.Message)
			}
			assert.Equal(t, nilIfEmptyStrings(tc.wantLinks), nilIfEmptyStrings(links))
		})
	}
}

type capturedEphemeral struct {
	userID string
	post   *model.Post
}

func nilIfEmptyStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
