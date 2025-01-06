package anthropic

import (
	"bytes"
	"testing"

	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/stretchr/testify/assert"
)

func TestConversationToMessages(t *testing.T) {
	tests := []struct {
		name         string
		conversation ai.BotConversation
		wantSystem   string
		wantMessages []anthropicSDK.MessageParam
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
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F("user"),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Hello"),
						},
					}),
				},
				{
					Role: anthropicSDK.F("assistant"),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Hi there!"),
						},
					}),
				},
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
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F("user"),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("First message"),
						},
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Second message"),
						},
					}),
				},
				{
					Role: anthropicSDK.F("assistant"),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("First response"),
						},
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Second response"),
						},
					}),
				},
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
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F("user"),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Look at this:"),
						},
						anthropicSDK.ImageBlockParam{
							Type: anthropicSDK.F(anthropicSDK.ImageBlockParamTypeImage),
							Source: anthropicSDK.F(anthropicSDK.ImageBlockParamSource{
								Type:      anthropicSDK.F(anthropicSDK.ImageBlockParamSourceTypeBase64),
								MediaType: anthropicSDK.F(anthropicSDK.ImageBlockParamSourceMediaType("image/jpeg")),
								Data:      anthropicSDK.F("ZmFrZS1pbWFnZS1kYXRh"),
							}),
						},
					}),
				},
				{
					Role: anthropicSDK.F("assistant"),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("I see the image"),
						},
					}),
				},
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
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F("user"),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("[Unsupported image type: image/tiff]"),
						},
					}),
				},
			},
		},
		{
			name: "complex back and forth with repeated roles",
			conversation: ai.BotConversation{
				Posts: []ai.Post{
					{Role: ai.PostRoleUser, Message: "First question"},
					{Role: ai.PostRoleBot, Message: "First answer"},
					{Role: ai.PostRoleUser, Message: "Follow up 1"},
					{Role: ai.PostRoleUser, Message: "Follow up 2"},
					{Role: ai.PostRoleUser, Message: "Follow up 3"},
					{Role: ai.PostRoleBot, Message: "Response 1"},
					{Role: ai.PostRoleBot, Message: "Response 2"},
					{Role: ai.PostRoleBot, Message: "Response 3"},
					{Role: ai.PostRoleUser, Message: "Final question"},
				},
			},
			wantSystem: "",
			wantMessages: []InputMessage{
				{Role: RoleUser, Content: "First question"},
				{Role: RoleAssistant, Content: "First answer"},
				{Role: RoleUser, Content: []ContentBlock{
					{Type: "text", Text: "Follow up 1"},
					{Type: "text", Text: "Follow up 2"},
					{Type: "text", Text: "Follow up 3"},
				}},
				{Role: RoleAssistant, Content: []ContentBlock{
					{Type: "text", Text: "Response 1"},
					{Type: "text", Text: "Response 2"},
					{Type: "text", Text: "Response 3"},
				}},
				{Role: RoleUser, Content: "Final question"},
			},
		},
		{
			name: "multiple roles with multiple images",
			conversation: ai.BotConversation{
				Posts: []ai.Post{
					{Role: ai.PostRoleUser, Message: "Look at these images:",
						Files: []ai.File{
							{
								MimeType: "image/jpeg",
								Reader:   bytes.NewReader([]byte("image-1")),
							},
							{
								MimeType: "image/png",
								Reader:   bytes.NewReader([]byte("image-2")),
							},
						},
					},
					{Role: ai.PostRoleBot, Message: "I see them"},
					{Role: ai.PostRoleUser, Message: "Here are more:",
						Files: []ai.File{
							{
								MimeType: "image/webp",
								Reader:   bytes.NewReader([]byte("image-3")),
							},
							{
								MimeType: "image/tiff", // unsupported
								Reader:   bytes.NewReader([]byte("image-4")),
							},
							{
								MimeType: "image/gif",
								Reader:   bytes.NewReader([]byte("image-5")),
							},
						},
					},
				},
			},
			wantSystem: "",
			wantMessages: []InputMessage{
				{Role: RoleUser, Content: []ContentBlock{
					{Type: "text", Text: "Look at these images:"},
					{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: "image/jpeg",
							Data:      "aW1hZ2UtMQ==", // base64 encoded "image-1"
						},
					},
					{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: "image/png",
							Data:      "aW1hZ2UtMg==", // base64 encoded "image-2"
						},
					},
				}},
				{Role: RoleAssistant, Content: "I see them"},
				{Role: RoleUser, Content: []ContentBlock{
					{Type: "text", Text: "Here are more:"},
					{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: "image/webp",
							Data:      "aW1hZ2UtMw==", // base64 encoded "image-3"
						},
					},
					{Type: "text", Text: "[Unsupported image type: image/tiff]"},
					{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: "image/gif",
							Data:      "aW1hZ2UtNQ==", // base64 encoded "image-5"
						},
					},
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
