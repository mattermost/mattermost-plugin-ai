package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) MakeConversationContext(bot *Bot, user *model.User, channel *model.Channel, post *model.Post) ai.ConversationContext {
	context := ai.NewConversationContext(bot.mmBot.UserId, user, channel, post)
	if p.pluginAPI.Configuration.GetConfig().TeamSettings.SiteName != nil {
		context.ServerName = *p.pluginAPI.Configuration.GetConfig().TeamSettings.SiteName
	}

	if license := p.pluginAPI.System.GetLicense(); license != nil && license.Customer != nil {
		context.CompanyName = license.Customer.Company
	}

	if channel != nil && (channel.Type != model.ChannelTypeDirect && channel.Type != model.ChannelTypeGroup) {
		team, err := p.pluginAPI.Team.Get(channel.TeamId)
		if err != nil {
			p.pluginAPI.Log.Error("Unable to get team for context", "error", err.Error(), "team_id", channel.TeamId)
		} else {
			context.Team = team
		}
	}

	context.CustomInstructions = bot.cfg.CustomInstructions

	if err := p.AddVectorSearchForConversationContext(&context); err != nil {
		p.API.LogError("Failed to add search results to conversation context", "error", err.Error())
	}

	return context
}

func (p *Plugin) AddSearchForConversationContext(context *ai.ConversationContext) error {
	terms := context.Post.Message
	page := 0
	perPage := 10
	results, appErr := p.API.SearchPostsInTeamForUser("", context.RequestingUser.Id, model.SearchParameter{
		Terms:   &terms,
		Page:    &page,
		PerPage: &perPage,
	})
	if appErr != nil {
		return fmt.Errorf("failed to search posts for user: %s", appErr.Error())
	}

	postsList, err := p.getMetadataForPosts(results.PostList)
	if err != nil {
		return fmt.Errorf("failed to get metadata for search posts: %s", err.Error())
	}

	// Clear out the post we are responding to
	slices.DeleteFunc(postsList.Posts, func(post *model.Post) bool {
		return post.Id == context.Post.Id
	})

	context.SearchResults = formatThread(postsList)

	return nil
}

func (p *Plugin) AddVectorSearchForConversationContext(context *ai.ConversationContext) error {
	// Embed the message
	messageEmbedding, err := p.getEmbeddingsModel().Embed(context.Post.Message)
	if err != nil {
		return err
	}
	pgEmbedding := postgresEmbeddingFormat(messageEmbedding)

	// Get similar posts
	var similarPosts []struct {
		Id         string
		Message    string
		Similarity float64
	}
	embeddingExpr := fmt.Sprintf("e.Embedding <=> '%s'", pgEmbedding)
	if err := p.doQuery(&similarPosts, p.builder.
		Select("p.id, p.message, 1-("+embeddingExpr+") AS Similarity").
		From("LLM_Post_Embeddings as e").
		Where("p.DeleteAt = 0").
		Where("p.id != ?", context.Post.Id).
		LeftJoin("Posts as p ON p.Id = e.postID").
		Where("p.ChannelID IN (SELECT id FROM Channels as c, ChannelMembers as cm WHERE c.id = cm.channelid AND c.DeleteAt = 0 AND cm.UserID = ?)", context.RequestingUser.Id).
		OrderBy(embeddingExpr).
		Limit(5),
	); err != nil {
		return fmt.Errorf("failed to query similar posts: %w", err)
	}

	result := strings.Builder{}
	for _, post := range similarPosts {
		result.WriteString(post.Id)
		result.WriteString(" : ")
		result.WriteString(post.Message)
		result.WriteString("\n")
	}

	context.SearchResults = result.String()

	return nil

}
