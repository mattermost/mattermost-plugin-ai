package ai

import "github.com/mattermost/mattermost-server/v6/model"

type PostRole int

const (
	PostRoleUser PostRole = iota
	PostRoleBot
)

type Post struct {
	Role    PostRole
	Message string
}

type BotConversation struct {
	Posts []Post
}

func GetPostRole(botID string, post *model.Post) PostRole {
	if post.UserId == botID {
		return PostRoleBot
	}
	return PostRoleUser
}

func PostToBotConversation(botID string, post *model.Post) BotConversation {
	return BotConversation{
		Posts: []Post{
			{
				Role:    GetPostRole(botID, post),
				Message: post.Message,
			},
		},
	}
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
