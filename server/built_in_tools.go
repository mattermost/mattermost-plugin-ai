package main

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
)

type LookupMattermostUserArgs struct {
	Username string `jsonschema_description:"The username of the user to lookup witout a leading '@'. Example: 'firstname.lastname'"`
}

func (p *Plugin) toolResolveLookupMattermostUser(context ai.ConversationContext, argsGetter ai.ToolArgumentGetter) (string, error) {
	var args LookupMattermostUserArgs
	err := argsGetter(&args)
	if err != nil {
		return "", errors.Wrap(err, "failed to get arguments for tool LookupMattermostUser")
	}

	// Fail for guests.
	if !p.pluginAPI.User.HasPermissionTo(context.RequestingUser.Id, model.PermissionViewMembers) {
		return "", errors.New("user doesn't have permission to lookup users")
	}

	user, err := p.pluginAPI.User.GetByUsername(args.Username)
	if err != nil {
		return "", errors.Wrap(err, "failed to lookup user")
	}

	userStatus, err := p.pluginAPI.User.GetStatus(user.Id)
	if err != nil {
		return "", errors.Wrap(err, "failed to get user status")
	}

	result := fmt.Sprintf("Username: %s", user.Username)
	if p.pluginAPI.Configuration.GetConfig().PrivacySettings.ShowFullName != nil && *p.pluginAPI.Configuration.GetConfig().PrivacySettings.ShowFullName {
		if user.FirstName != "" || user.LastName != "" {
			result += fmt.Sprintf("\nFull Name: %s %s", user.FirstName, user.LastName)
		}
	}
	if p.pluginAPI.Configuration.GetConfig().PrivacySettings.ShowEmailAddress != nil && *p.pluginAPI.Configuration.GetConfig().PrivacySettings.ShowEmailAddress {
		result += fmt.Sprintf("\nEmail: %s", user.Email)
	}
	if user.Nickname != "" {
		result += fmt.Sprintf("\nNickname: %s", user.Nickname)
	}
	if user.Position != "" {
		result += fmt.Sprintf("\nPosition: %s", user.Position)
	}
	if user.Locale != "" {
		result += fmt.Sprintf("\nLocale: %s", user.Locale)
	}
	result += fmt.Sprintf("\nTimezone: %s", model.GetPreferredTimezone(user.Timezone))
	result += fmt.Sprintf("\nLast Activity: %s", model.GetTimeForMillis(userStatus.LastActivityAt).Format("2006-01-02 15:04:05 MST"))
	result += fmt.Sprintf("\nStatus: %s", userStatus.Status)

	return result, nil
}

type GetChannelPosts struct {
	ChannelName string `jsonschema_description:"The name of the channel to get posts from. Should be the channel name without the leading '~'. Example: 'town-square'"`
	NumberPosts int    `jsonschema_description:"The number of most recent posts to get. Example: '30'"`
}

func (p *Plugin) toolResolveGetChannelPosts(context ai.ConversationContext, argsGetter ai.ToolArgumentGetter) (string, error) {
	var args GetChannelPosts
	err := argsGetter(&args)
	if err != nil {
		return "invalid parameters to function", errors.Wrap(err, "failed to get arguments for tool GetChannelPosts")
	}

	if context.Channel == nil || context.Channel.TeamId == "" {
		//TODO: support DMs. This will require some way to disabiguate between channels with the same name on different teams.
		return "Error: Ambiguous channel lookup. Unable to what channel the user is reffering to because DMs do not belong to specific teams. Tell the user to ask outside a DM channel.", errors.New("ambiguous channel lookup")
	}

	if !p.pluginAPI.User.HasPermissionToChannel(context.RequestingUser.Id, context.Channel.Id, model.PermissionReadChannel) {
		return "user doesn't have permissions to read requested channel", errors.New("user doesn't have permission to read channel")
	}

	channel, err := p.pluginAPI.Channel.GetByName(context.Channel.TeamId, args.ChannelName, false)
	if err != nil {
		return "internal failure", errors.Wrap(err, "failed to lookup channel by name, may not exist")
	}

	posts, err := p.pluginAPI.Post.GetPostsForChannel(channel.Id, 0, args.NumberPosts)
	if err != nil {
		return "internal failure", errors.Wrap(err, "failed to get posts for channel")
	}

	postsData, err := p.getMetadataForPosts(posts)
	if err != nil {
		return "internal failure", errors.Wrap(err, "failed to get metadata for posts")
	}

	return formatThread(postsData), nil
}

func (p *Plugin) getBuiltInTools() []ai.Tool {
	builtInTools := []ai.Tool{
		{
			Name:        "LookupMattermostUser",
			Description: "Lookup a Mattermost user by their username. Avalable information includes: username, full name, email, nickname, position, locale, timezone, last activity, and status.",
			Schema:      LookupMattermostUserArgs{},
			Resolver:    p.toolResolveLookupMattermostUser,
		},
		{
			Name:        "GetChannelPosts",
			Description: "Get the most recent posts from a Mattermost channel. Returns posts in the format 'username: message'",
			Schema:      GetChannelPosts{},
			Resolver:    p.toolResolveGetChannelPosts,
		},
	}
	return builtInTools
}
