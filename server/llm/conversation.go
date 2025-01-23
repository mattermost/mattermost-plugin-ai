// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // Needed to fill time.LoadLocation db

	"github.com/mattermost/mattermost-plugin-ai/server/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

type PostRole int

const (
	PostRoleUser PostRole = iota
	PostRoleBot
	PostRoleSystem
)

type File struct {
	MimeType string
	Size     int64
	Reader   io.Reader
}

type Post struct {
	Role    PostRole
	Message string
	Files   []File
}

type ConversationContext struct {
	BotID              string
	Time               string
	ServerName         string
	CompanyName        string
	RequestingUser     *model.User
	Channel            *model.Channel
	Team               *model.Team
	Post               *model.Post
	PromptParameters   map[string]string
	CustomInstructions string
}

func NewConversationContext(botID string, requestingUser *model.User, channel *model.Channel, post *model.Post) ConversationContext {
	// Get current time and date formatted nicely with the user's locale
	now := time.Now()
	nowString := now.Format(time.RFC1123)
	if requestingUser != nil {
		tz := requestingUser.GetPreferredTimezone()
		loc, err := time.LoadLocation(tz)
		if err != nil || loc == nil {
			loc = time.UTC
		}
		nowString = now.In(loc).Format(time.RFC1123)
	}
	return ConversationContext{
		Time:           nowString,
		RequestingUser: requestingUser,
		Channel:        channel,
		Post:           post,
		BotID:          botID,
	}
}

func (c *ConversationContext) IsDMWithBot() bool {
	return mmapi.IsDMWith(c.BotID, c.Channel)
}

func (c ConversationContext) String() string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Time: %v\nServerName: %v\nCompanyName: %v", c.Time, c.ServerName, c.CompanyName))
	if c.RequestingUser != nil {
		result.WriteString(fmt.Sprintf("\nRequestingUser: %v", c.RequestingUser.Username))
	}
	if c.Channel != nil {
		result.WriteString(fmt.Sprintf("\nChannel: %v", c.Channel.Name))
	}
	if c.Team != nil {
		result.WriteString(fmt.Sprintf("\nTeam: %v", c.Team.Name))
	}
	if c.Post != nil {
		result.WriteString(fmt.Sprintf("\nPost: %v", c.Post.Id))
	}

	result.WriteString("\nPromptParameters:")
	for key := range c.PromptParameters {
		result.WriteString(fmt.Sprintf(" %v", key))
	}

	return result.String()
}

func NewConversationContextParametersOnly(promptParameters map[string]string) ConversationContext {
	return ConversationContext{
		PromptParameters: promptParameters,
	}
}

type BotConversation struct {
	Posts   []Post
	Tools   ToolStore
	Context ConversationContext
}

func (b *BotConversation) AddPost(post Post) {
	b.Posts = append(b.Posts, post)
}

func (b *BotConversation) AppendConversation(conversation BotConversation) {
	b.Posts = append(b.Posts, conversation.Posts...)
}

func (b *BotConversation) ExtractSystemMessage() string {
	var result strings.Builder
	for _, post := range b.Posts {
		if post.Role == PostRoleSystem {
			result.WriteString(post.Message)
		}
	}
	return result.String()
}

func (b BotConversation) String() string {
	// Create a string of all the posts with their role and message
	var result strings.Builder
	result.WriteString("--- Conversation ---")
	for _, post := range b.Posts {
		switch post.Role {
		case PostRoleUser:
			result.WriteString("\n--- User ---\n")
		case PostRoleBot:
			result.WriteString("\n--- Bot ---\n")
		case PostRoleSystem:
			result.WriteString("\n--- System ---\n")
		default:
			result.WriteString("\n--- <Unknown> ---\n")
		}
		result.WriteString(post.Message)
	}
	result.WriteString("\n--- Tools ---\n")
	for _, tool := range b.Tools.GetTools() {
		result.WriteString(tool.Name)
		result.WriteString(" ")
	}
	result.WriteString("\n--- Context ---\n")
	result.WriteString(fmt.Sprintf("%+v\n", b.Context))

	return result.String()
}

func (b *BotConversation) Truncate(maxTokens int, countTokens func(string) int) bool {
	oldPosts := b.Posts
	b.Posts = make([]Post, 0, len(oldPosts))
	var totalTokens int
	for i := len(oldPosts) - 1; i >= 0; i-- {
		post := oldPosts[i]
		if totalTokens >= maxTokens {
			slices.Reverse(b.Posts)
			return true
		}
		postTokens := countTokens(post.Message)
		if (totalTokens + postTokens) > maxTokens {
			charactersToCut := (postTokens - (maxTokens - totalTokens)) * 4
			post.Message = strings.TrimSpace(post.Message[charactersToCut:])
			b.Posts = append(b.Posts, post)
			slices.Reverse(b.Posts)
			return true
		}
		totalTokens += postTokens
		b.Posts = append(b.Posts, post)
	}

	slices.Reverse(b.Posts)
	return false
}

func FormatPostBody(post *model.Post) string {
	attachments := post.Attachments()
	if len(attachments) > 0 {
		result := strings.Builder{}
		result.WriteString(post.Message)
		for _, attachment := range attachments {
			result.WriteString("\n")
			if attachment.Pretext != "" {
				result.WriteString(attachment.Pretext)
				result.WriteString("\n")
			}
			if attachment.Title != "" {
				result.WriteString(attachment.Title)
				result.WriteString("\n")
			}
			if attachment.Text != "" {
				result.WriteString(attachment.Text)
				result.WriteString("\n")
			}
			for _, field := range attachment.Fields {
				value, err := json.Marshal(field.Value)
				if err != nil {
					continue
				}
				result.WriteString(field.Title)
				result.WriteString(": ")
				result.Write(value)
				result.WriteString("\n")
			}

			if attachment.Footer != "" {
				result.WriteString(attachment.Footer)
				result.WriteString("\n")
			}
		}
		return result.String()
	}
	return post.Message
}
