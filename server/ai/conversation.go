package ai

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/mattermost/mattermost/server/public/model"
)

type PostRole int

const (
	PostRoleUser PostRole = iota
	PostRoleBot
	PostRoleSystem
)

type Post struct {
	Role    PostRole
	Message string
}

type ConversationContext struct {
	Time             string
	ServerName       string
	CompanyName      string
	RequestingUser   *model.User
	Channel          *model.Channel
	Team             *model.Team
	Post             *model.Post
	PromptParameters map[string]string
}

func NewConversationContext(requestingUser *model.User, channel *model.Channel, post *model.Post) ConversationContext {
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
	}
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

func (b *BotConversation) AddUserPost(post *model.Post) {
	b.Posts = append(b.Posts, Post{
		Role:    PostRoleUser,
		Message: post.Message,
	})
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

func GetPostRole(botID string, post *model.Post) PostRole {
	if post.UserId == botID {
		return PostRoleBot
	}
	return PostRoleUser
}

func ThreadToBotConversation(botID string, posts []*model.Post) BotConversation {
	result := BotConversation{
		Posts: make([]Post, 0, len(posts)),
	}

	for _, post := range posts {
		result.Posts = append(result.Posts, Post{
			Role:    GetPostRole(botID, post),
			Message: post.Message,
		})
	}

	return result
}
