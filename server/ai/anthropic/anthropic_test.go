package anthropic

import (
	"bytes"
	"testing"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/stretchr/testify/assert"
)

func TestConversationToMessages(t *testing.T) {
	tests := []struct {
		name         string
		conversation ai.BotConversation
		wantSystem   string
		wantMessages []InputMessage
	}{
		{
			name: "basic conversation with system message",
			conversation: ai.BotConversation{
				Posts: []ai.Post{
					{Role: ai.PostRoleSystem, Message: "You are a helpful assistant"},
					{Role: ai.PostRoleUser, Message: "Hello"},
					{Role: ai.PostRoleBot, Message: "Hi there!"},
				},
			},
			wantSystem: "You are a helpful assistant",
			wantMessages: []InputMessage{
				{Role: RoleUser, Content: "Hello"},
				{Role: RoleAssistant, Content: "Hi there!"},
			},
		},
		{
			name: "multiple messages from same role",
			conversation: ai.BotConversation{
				Posts: []ai.Post{
					{Role: ai.PostRoleUser, Message: "First message"},
					{Role: ai.PostRoleUser, Message: "Second message"},
					{Role: ai.PostRoleBot, Message: "First response"},
					{Role: ai.PostRoleBot, Message: "Second response"},
				},
			},
			wantSystem: "",
			wantMessages: []InputMessage{
				{Role: RoleUser, Content: []ContentBlock{
					{Type: "text", Text: "First message"},
					{Type: "text", Text: "Second message"},
				}},
				{Role: RoleAssistant, Content: []ContentBlock{
					{Type: "text", Text: "First response"},
					{Type: "text", Text: "Second response"},
				}},
			},
		},
		{
			name: "conversation with image",
			conversation: ai.BotConversation{
				Posts: []ai.Post{
					{Role: ai.PostRoleUser, Message: "Look at this:",
						Files: []ai.File{
							{
								MimeType: "image/jpeg",
								Reader:   bytes.NewReader([]byte("fake-image-data")),
							},
						}},
					{Role: ai.PostRoleBot, Message: "I see the image"},
				},
			},
			wantSystem: "",
			wantMessages: []InputMessage{
				{Role: RoleUser, Content: []ContentBlock{
					{Type: "text", Text: "Look at this:"},
					{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: "image/jpeg",
							Data:      "ZmFrZS1pbWFnZS1kYXRh", // base64 encoded "fake-image-data"
						},
					},
				}},
				{Role: RoleAssistant, Content: "I see the image"},
			},
		},
		{
			name: "unsupported image type",
			conversation: ai.BotConversation{
				Posts: []ai.Post{
					{Role: ai.PostRoleUser, Files: []ai.File{
						{
							MimeType: "image/tiff",
							Reader:   bytes.NewReader([]byte("fake-tiff-data")),
						},
					}},
				},
			},
			wantSystem: "",
			wantMessages: []InputMessage{
				{Role: RoleUser, Content: []ContentBlock{
					{Type: "text", Text: "[Unsupported image type: image/tiff]"},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSystem, gotMessages := conversationToMessages(tt.conversation)
			assert.Equal(t, tt.wantSystem, gotSystem)
			assert.Equal(t, tt.wantMessages, gotMessages)
		})
	}
}
