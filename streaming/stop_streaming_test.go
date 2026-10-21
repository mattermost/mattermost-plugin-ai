// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package streaming

import (
	"context"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/i18n"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestStreamToPostMarksUserStop(t *testing.T) {
	const channelID = "channel-id"
	const postID = "post-id"

	tests := []struct {
		name        string
		end         func(service *MMPostStreamService, cancelParent context.CancelFunc)
		wantStopped bool
	}{
		{
			name:        "requester stops the stream",
			end:         func(s *MMPostStreamService, _ context.CancelFunc) { s.StopStreaming(postID) },
			wantStopped: true,
		},
		{
			name: "run finishes without a user stop",
			end:  func(s *MMPostStreamService, _ context.CancelFunc) { s.FinishStreaming(postID) },
		},
		{
			name: "parent context is canceled",
			end:  func(_ *MMPostStreamService, cancelParent context.CancelFunc) { cancelParent() },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeStreamingClient{channels: map[string]*model.Channel{channelID: {Id: channelID, Type: model.ChannelTypeOpen}}}
			service := NewMMPostStreamService(client, i18n.Init())

			parent, cancelParent := context.WithCancel(context.Background())
			defer cancelParent()
			ctx, err := service.GetStreamingContext(parent, postID)
			require.NoError(t, err)

			stream := make(chan llm.TextStreamEvent)
			go func() {
				stream <- llm.TextStreamEvent{Type: llm.EventTypeText, Value: "partial answer"}
				tc.end(service, cancelParent)
			}()

			post := &model.Post{Id: postID, ChannelId: channelID}
			service.StreamToPost(ctx, &llm.TextStreamResult{Stream: stream}, post, "en", "requester-id")

			require.NotEmpty(t, client.updatedPosts)
			final := client.updatedPosts[len(client.updatedPosts)-1]
			require.Equal(t, "partial answer", final.Message)
			if tc.wantStopped {
				require.Equal(t, true, final.GetProp(GenerationStoppedProp))
			} else {
				require.Nil(t, final.GetProp(GenerationStoppedProp))
			}
		})
	}
}

func TestStreamToPostClearsEarlierStop(t *testing.T) {
	const channelID = "channel-id"

	client := &fakeStreamingClient{channels: map[string]*model.Channel{channelID: {Id: channelID, Type: model.ChannelTypeOpen}}}
	service := NewMMPostStreamService(client, i18n.Init())

	stream := make(chan llm.TextStreamEvent, 2)
	stream <- llm.TextStreamEvent{Type: llm.EventTypeText, Value: "regenerated answer"}
	stream <- llm.TextStreamEvent{Type: llm.EventTypeEnd}

	post := &model.Post{Id: "post-id", ChannelId: channelID}
	post.AddProp(GenerationStoppedProp, true)
	service.StreamToPost(context.Background(), &llm.TextStreamResult{Stream: stream}, post, "en", "requester-id")

	require.NotEmpty(t, client.updatedPosts)
	require.Nil(t, client.updatedPosts[len(client.updatedPosts)-1].GetProp(GenerationStoppedProp))
}
