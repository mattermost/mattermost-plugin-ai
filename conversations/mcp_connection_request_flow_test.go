// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/conversation"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/llmcontext"
	"github.com/mattermost/mattermost-plugin-agents/v2/mcp"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmapi/mocks"
	"github.com/mattermost/mattermost-plugin-agents/v2/store"
	"github.com/mattermost/mattermost-plugin-agents/v2/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const connectionTestJiraOrigin = "https://jira.example.com"

// connectionTestMCPProvider serves the catalog a user sees after (or still
// before) connecting Jira.
type connectionTestMCPProvider struct {
	connected bool
}

func (p *connectionTestMCPProvider) GetToolsWithSelection(context.Context, mcp.CatalogRequest, mcp.ToolSelection) ([]llm.Tool, *mcp.Errors) {
	if p.connected {
		return []llm.Tool{loadedStateTool()}, nil
	}
	return nil, &mcp.Errors{ToolAuthErrors: []llm.ToolAuthError{{
		ServerName:   "Jira",
		ServerOrigin: connectionTestJiraOrigin,
		AuthURL:      "https://mm.example.com/plugins/mattermost-ai/mcp/oauth/Jira/start",
	}}}
}

func (p *connectionTestMCPProvider) DescribeUnconnectedMCPServers(authErrors []llm.ToolAuthError) []llm.UnconnectedMCPServer {
	var out []llm.UnconnectedMCPServer
	for _, authErr := range authErrors {
		out = append(out, llm.UnconnectedMCPServer{Name: authErr.ServerName, ServerOrigin: authErr.ServerOrigin})
	}
	return out
}

func (p *connectionTestMCPProvider) MCPServerOriginsByName() map[string]string {
	return map[string]string{"Jira": connectionTestJiraOrigin}
}

// TestHandleToolCallResolvesConnectionRequest covers the connect round-trip:
// Continue reports whether the rebuilt catalog now has the server's tools,
// Skip records a decline, and every outcome is terminal and continues the
// conversation without blaming the user.
func TestHandleToolCallResolvesConnectionRequest(t *testing.T) {
	requestInput := json.RawMessage(`{"server":"Jira","reason":"Look up your open issues"}`)
	dmChannel := &model.Channel{Id: "dm-channel", Type: model.ChannelTypeDirect, Name: "bot-id__user-id"}
	openChannel := &model.Channel{Id: "channel-id", TeamId: "team-id", Type: model.ChannelTypeOpen}

	cases := []struct {
		name              string
		channel           *model.Channel
		connected         bool
		accepted          bool
		wantStatus        string
		wantResultErr     bool
		wantConnected     bool
		wantTools         []string
		wantResultSnippet string
	}{
		{
			name:          "continue after connecting reports the server's tools",
			channel:       dmChannel,
			connected:     true,
			accepted:      true,
			wantStatus:    conversation.StatusSuccess,
			wantConnected: true,
			wantTools:     []string{"jira__get_issue"},
		},
		{
			name:              "continue without connecting reports the server is still unavailable",
			channel:           dmChannel,
			accepted:          true,
			wantStatus:        conversation.StatusSuccess,
			wantResultSnippet: "Jira is still not connected",
		},
		{
			name:              "skip in a channel records the decline and continues",
			channel:           openChannel,
			wantStatus:        conversation.StatusRejected,
			wantResultErr:     true,
			wantResultSnippet: "The user chose not to connect Jira.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			convStore, conv := loadedStateConversationStore()
			blocks := []conversation.ContentBlock{{
				Type:            conversation.BlockTypeToolUse,
				ID:              "connect-1",
				Name:            mcp.RequestConnectionToolName,
				Input:           requestInput,
				Status:          conversation.StatusPending,
				UserInteraction: llm.UserInteractionConnect,
				Shared:          new(false),
			}}
			content, err := json.Marshal(blocks)
			require.NoError(t, err)
			approvalPostID := "approval-post-id"
			require.NoError(t, convStore.CreateTurn(&store.Turn{
				ID:             "assistant-turn",
				ConversationID: conv.ID,
				PostID:         &approvalPostID,
				Role:           "assistant",
				Content:        content,
				Sequence:       1,
			}))

			mockAPI := &plugintest.API{}
			mockLicenseState(mockAPI, true)
			mockAPI.On("GetTeam", "team-id").Return(&model.Team{Id: "team-id", Name: "team"}, nil).Maybe()
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
			mmClient.On("GetPost", mock.Anything).Maybe().Return(nil, &model.AppError{Message: "not found"})
			mmClient.On("GetPostThread", mock.Anything).Maybe().Return(nil, &model.AppError{Message: "not found"})

			streamingService := &loadedStateStreamingService{}
			c := &Conversations{
				mmClient: mmClient,
				contextBuilder: llmcontext.NewLLMContextBuilder(
					pluginAPI,
					&channelFollowUpTestToolProvider{},
					&connectionTestMCPProvider{connected: tc.connected},
					&channelFollowUpTestConfig{},
				),
				bots:             botsService,
				convService:      conversation.NewService(convStore, nil, nil, nil),
				streamingService: streamingService,
				licenseChecker:   licenseChecker,
			}

			approvalPost := &model.Post{Id: approvalPostID, UserId: "bot-id"}
			approvalPost.AddProp(streaming.ConversationIDProp, conv.ID)

			var acceptedIDs []string
			if tc.accepted {
				acceptedIDs = []string{"connect-1"}
			}
			require.NoError(t, c.HandleToolCall(context.Background(), "user-id", approvalPost, tc.channel, acceptedIDs, nil))
			streamingService.waitForStreaming()

			turns, err := convStore.GetTurnsForConversation(conv.ID)
			require.NoError(t, err)
			require.Len(t, turns, 2)

			var updatedBlocks []conversation.ContentBlock
			require.NoError(t, json.Unmarshal(turns[0].Content, &updatedBlocks))
			assert.Equal(t, tc.wantStatus, updatedBlocks[0].Status)

			var resultBlocks []conversation.ContentBlock
			require.NoError(t, json.Unmarshal(turns[1].Content, &resultBlocks))
			require.Len(t, resultBlocks, 1)
			result := resultBlocks[0]
			assert.NotNil(t, result.DecidedAt, "connection decisions are terminal")
			require.NotNil(t, result.Shared)
			assert.True(t, *result.Shared)

			if tc.wantResultErr {
				assert.Equal(t, conversation.StatusError, result.Status)
				assert.Contains(t, result.Content, tc.wantResultSnippet)
			} else {
				assert.Equal(t, conversation.StatusSuccess, result.Status)
				var parsed mcp.RequestConnectionResult
				require.NoError(t, json.Unmarshal([]byte(result.Content), &parsed))
				assert.Equal(t, tc.wantConnected, parsed.Connected)
				assert.Equal(t, tc.wantTools, parsed.Tools)
				assert.Contains(t, parsed.Message, tc.wantResultSnippet)
			}

			require.Len(t, lm.requests, 1, "expected a follow-up LLM request")
			assert.Zero(t, countUserMessagesContaining(lm.requests[0].Posts, toolRejectionGuidance),
				"connection decisions must not receive tool-rejection guidance")

			followUpTools := lm.requests[0].Context.Tools
			if tc.connected {
				assert.Nil(t, followUpTools.GetTool(mcp.RequestConnectionToolName),
					"a connected server must not be offered again")
			} else {
				assert.NotNil(t, followUpTools.GetTool(mcp.RequestConnectionToolName),
					"the server stays connectable if the user changes their mind")
			}
		})
	}
}
