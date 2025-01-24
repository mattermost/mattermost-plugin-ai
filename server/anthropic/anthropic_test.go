// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package anthropic

import (
	"bytes"
	"testing"

	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/stretchr/testify/assert"
)

func TestConversationToMessages(t *testing.T) {
	tests := []struct {
		name         string
		conversation llm.BotConversation
		wantSystem   string
		wantMessages []anthropicSDK.MessageParam
	}{
		{
			name: "basic conversation with system message",
			conversation: llm.BotConversation{
				Posts: []llm.Post{
					{Role: llm.PostRoleSystem, Message: "You are a helpful assistant"},
					{Role: llm.PostRoleUser, Message: "Hello"},
					{Role: llm.PostRoleBot, Message: "Hi there!"},
				},
			},
			wantSystem: "You are a helpful assistant",
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Hello"),
						},
					}),
				},
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleAssistant),
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
			conversation: llm.BotConversation{
				Posts: []llm.Post{
					{Role: llm.PostRoleUser, Message: "First message"},
					{Role: llm.PostRoleUser, Message: "Second message"},
					{Role: llm.PostRoleBot, Message: "First response"},
					{Role: llm.PostRoleBot, Message: "Second response"},
				},
			},
			wantSystem: "",
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
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
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleAssistant),
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
			conversation: llm.BotConversation{
				Posts: []llm.Post{
					{Role: llm.PostRoleUser, Message: "Look at this:",
						Files: []llm.File{
							{
								MimeType: "image/jpeg",
								Reader:   bytes.NewReader([]byte("fake-image-data")),
							},
						}},
					{Role: llm.PostRoleBot, Message: "I see the image"},
				},
			},
			wantSystem: "",
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
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
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleAssistant),
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
			conversation: llm.BotConversation{
				Posts: []llm.Post{
					{Role: llm.PostRoleUser, Files: []llm.File{
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
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
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
			conversation: llm.BotConversation{
				Posts: []llm.Post{
					{Role: llm.PostRoleUser, Message: "First question"},
					{Role: llm.PostRoleBot, Message: "First answer"},
					{Role: llm.PostRoleUser, Message: "Follow up 1"},
					{Role: llm.PostRoleUser, Message: "Follow up 2"},
					{Role: llm.PostRoleUser, Message: "Follow up 3"},
					{Role: llm.PostRoleBot, Message: "Response 1"},
					{Role: llm.PostRoleBot, Message: "Response 2"},
					{Role: llm.PostRoleBot, Message: "Response 3"},
					{Role: llm.PostRoleUser, Message: "Final question"},
				},
			},
			wantSystem: "",
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("First question"),
						},
					}),
				},
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleAssistant),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("First answer"),
						},
					}),
				},
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Follow up 1"),
						},
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Follow up 2"),
						},
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Follow up 3"),
						},
					}),
				},
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleAssistant),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Response 1"),
						},
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Response 2"),
						},
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Response 3"),
						},
					}),
				},
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Final question"),
						},
					}),
				},
			},
		},
		{
			name: "multiple roles with multiple images",
			conversation: llm.BotConversation{
				Posts: []llm.Post{
					{Role: llm.PostRoleUser, Message: "Look at these images:",
						Files: []llm.File{
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
					{Role: llm.PostRoleBot, Message: "I see them"},
					{Role: llm.PostRoleUser, Message: "Here are more:",
						Files: []llm.File{
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
			wantMessages: []anthropicSDK.MessageParam{
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Look at these images:"),
						},
						anthropicSDK.ImageBlockParam{
							Type: anthropicSDK.F(anthropicSDK.ImageBlockParamTypeImage),
							Source: anthropicSDK.F(anthropicSDK.ImageBlockParamSource{
								Type:      anthropicSDK.F(anthropicSDK.ImageBlockParamSourceTypeBase64),
								MediaType: anthropicSDK.F(anthropicSDK.ImageBlockParamSourceMediaType("image/jpeg")),
								Data:      anthropicSDK.F("aW1hZ2UtMQ=="),
							}),
						},
						anthropicSDK.ImageBlockParam{
							Type: anthropicSDK.F(anthropicSDK.ImageBlockParamTypeImage),
							Source: anthropicSDK.F(anthropicSDK.ImageBlockParamSource{
								Type:      anthropicSDK.F(anthropicSDK.ImageBlockParamSourceTypeBase64),
								MediaType: anthropicSDK.F(anthropicSDK.ImageBlockParamSourceMediaType("image/png")),
								Data:      anthropicSDK.F("aW1hZ2UtMg=="),
							}),
						},
					}),
				},
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleAssistant),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("I see them"),
						},
					}),
				},
				{
					Role: anthropicSDK.F(anthropicSDK.MessageParamRoleUser),
					Content: anthropicSDK.F([]anthropicSDK.ContentBlockParamUnion{
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("Here are more:"),
						},
						anthropicSDK.ImageBlockParam{
							Type: anthropicSDK.F(anthropicSDK.ImageBlockParamTypeImage),
							Source: anthropicSDK.F(anthropicSDK.ImageBlockParamSource{
								Type:      anthropicSDK.F(anthropicSDK.ImageBlockParamSourceTypeBase64),
								MediaType: anthropicSDK.F(anthropicSDK.ImageBlockParamSourceMediaType("image/webp")),
								Data:      anthropicSDK.F("aW1hZ2UtMw=="),
							}),
						},
						anthropicSDK.TextBlockParam{
							Type: anthropicSDK.F(anthropicSDK.TextBlockParamTypeText),
							Text: anthropicSDK.F("[Unsupported image type: image/tiff]"),
						},
						anthropicSDK.ImageBlockParam{
							Type: anthropicSDK.F(anthropicSDK.ImageBlockParamTypeImage),
							Source: anthropicSDK.F(anthropicSDK.ImageBlockParamSource{
								Type:      anthropicSDK.F(anthropicSDK.ImageBlockParamSourceTypeBase64),
								MediaType: anthropicSDK.F(anthropicSDK.ImageBlockParamSourceMediaType("image/gif")),
								Data:      anthropicSDK.F("aW1hZ2UtNQ=="),
							}),
						},
					}),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSystem, gotMessages := conversationToMessages(tt.conversation.Posts)
			assert.Equal(t, tt.wantSystem, gotSystem)
			assert.Equal(t, tt.wantMessages, gotMessages)
		})
	}
}
