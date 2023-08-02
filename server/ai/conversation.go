package ai

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/mattermost/mattermost-server/v6/model"
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

func NewConversationContext(reqeustingUser *model.User, channel *model.Channel, post *model.Post) ConversationContext {
	// Get current time and date formated nicely with the user's locale
	now := time.Now()
	nowString := now.Format(time.RFC1123)
	if reqeustingUser != nil {
		tz := reqeustingUser.GetPreferredTimezone()
		loc, err := time.LoadLocation(tz)
		if err != nil || loc == nil {
			loc = time.UTC
		}
		nowString = now.In(loc).Format(time.RFC1123)
	}
	return ConversationContext{
		Time:           nowString,
		RequestingUser: reqeustingUser,
		Channel:        channel,
		Post:           post,
	}
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
	result.WriteString("--- Conversation ---\n")
	for _, post := range b.Posts {
		switch post.Role {
		case PostRoleUser:
			result.WriteString("User: ")
		case PostRoleBot:
			result.WriteString("Bot: ")
		case PostRoleSystem:
			result.WriteString("System: ")
		default:
			result.WriteString("<unknown>: ")
		}
		result.WriteString(post.Message)
		result.WriteString("\n---------\n")
	}
	result.WriteString("--- Tools ---\n")
	for _, tool := range b.Tools.GetTools() {
		result.WriteString(tool.Name)
		result.WriteString(": ")
		result.WriteString(tool.Description)
		result.WriteString("\n")
		result.WriteString(fmt.Sprintf("%+v\n", tool.Schema))
	}
	result.WriteString("--- Context ---\n")
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
