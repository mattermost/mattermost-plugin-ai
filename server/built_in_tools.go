// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"errors"

	"github.com/andygrunwald/go-jira"
	"github.com/google/go-github/v41/github"
	"github.com/mattermost/mattermost-plugin-ai/server/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type LookupMattermostUserArgs struct {
	Username string `jsonschema_description:"The username of the user to lookup without a leading '@'. Example: 'firstname.lastname'"`
}

func (p *Plugin) toolResolveLookupMattermostUser(context llm.ConversationContext, argsGetter llm.ToolArgumentGetter) (string, error) {
	var args LookupMattermostUserArgs
	err := argsGetter(&args)
	if err != nil {
		return "invalid parameters to function", fmt.Errorf("failed to get arguments for tool LookupMattermostUser: %w", err)
	}

	if !model.IsValidUsername(args.Username) {
		return "invalid username", errors.New("invalid username")
	}

	// Fail for guests.
	if !p.pluginAPI.User.HasPermissionTo(context.RequestingUser.Id, model.PermissionViewMembers) {
		return "user doesn't have permissions", errors.New("user doesn't have permission to lookup users")
	}

	user, err := p.pluginAPI.User.GetByUsername(args.Username)
	if err != nil {
		if errors.Is(err, pluginapi.ErrNotFound) {
			return "user not found", nil
		}
		return "failed to lookup user", fmt.Errorf("failed to lookup user: %w", err)
	}

	userStatus, err := p.pluginAPI.User.GetStatus(user.Id)
	if err != nil {
		return "failed to lookup user", fmt.Errorf("failed to get user status: %w", err)
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
	// Exclude manual statuses because they could be prompt injections
	if userStatus.Status != "" && !userStatus.Manual {
		result += fmt.Sprintf("\nStatus: %s", userStatus.Status)
	}

	return result, nil
}

type GetChannelPosts struct {
	ChannelName string `jsonschema_description:"The name of the channel to get posts from. Should be the channel name without the leading '~'. Example: 'town-square'"`
	NumberPosts int    `jsonschema_description:"The number of most recent posts to get. Example: '30'"`
}

func (p *Plugin) toolResolveGetChannelPosts(context llm.ConversationContext, argsGetter llm.ToolArgumentGetter, bot *Bot) (string, error) {
	var args GetChannelPosts
	err := argsGetter(&args)
	if err != nil {
		return "invalid parameters to function", fmt.Errorf("failed to get arguments for tool GetChannelPosts: %w", err)
	}

	if !model.IsValidChannelIdentifier(args.ChannelName) {
		return "invalid channel name", errors.New("invalid channel name")
	}

	if args.NumberPosts < 1 || args.NumberPosts > 100 {
		return "invalid number of posts. only 100 supported at a time", errors.New("invalid number of posts")
	}

	if context.Channel == nil || context.Channel.TeamId == "" {
		//TODO: support DMs. This will require some way to disambiguate between channels with the same name on different teams.
		return "Error: Ambiguous channel lookup. Unable to what channel the user is referring to because DMs do not belong to specific teams. Tell the user to ask outside a DM channel.", errors.New("ambiguous channel lookup")
	}

	channel, err := p.pluginAPI.Channel.GetByName(context.Channel.TeamId, args.ChannelName, false)
	if err != nil {
		return "internal failure", fmt.Errorf("failed to lookup channel by name, may not exist: %w", err)
	}

	if err = p.checkUsageRestrictionsForChannel(bot, channel); err != nil {
		return "user asked for a channel that is blocked by usage restrictions", fmt.Errorf("usage restrictions during channel lookup: %w", err)
	}

	if !p.pluginAPI.User.HasPermissionToChannel(context.RequestingUser.Id, channel.Id, model.PermissionReadChannel) {
		return "user doesn't have permissions to read requested channel", errors.New("user doesn't have permission to read channel")
	}

	posts, err := p.pluginAPI.Post.GetPostsForChannel(channel.Id, 0, args.NumberPosts)
	if err != nil {
		return "internal failure", fmt.Errorf("failed to get posts for channel: %w", err)
	}

	postsData, err := p.getMetadataForPosts(posts)
	if err != nil {
		return "internal failure", fmt.Errorf("failed to get metadata for posts: %w", err)
	}

	return formatThread(postsData), nil
}

type GetGithubIssueArgs struct {
	RepoOwner string `jsonschema_description:"The owner of the repository to get issues from. Example: 'mattermost'"`
	RepoName  string `jsonschema_description:"The name of the repository to get issues from. Example: 'mattermost-plugin-ai'"`
	Number    int    `jsonschema_description:"The issue number to get. Example: '1'"`
}

func formatGithubIssue(issue *github.Issue) string {
	return fmt.Sprintf("Title: %s\nNumber: %d\nState: %s\nSubmitter: %s\nIs Pull Request: %v\nBody: %s", issue.GetTitle(), issue.GetNumber(), issue.GetState(), issue.User.GetLogin(), issue.IsPullRequest(), issue.GetBody())
}

var validGithubRepoName = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

func (p *Plugin) toolGetGithubIssue(context llm.ConversationContext, argsGetter llm.ToolArgumentGetter) (string, error) {
	var args GetGithubIssueArgs
	err := argsGetter(&args)
	if err != nil {
		return "invalid parameters to function", fmt.Errorf("failed to get arguments for tool GetGithubIssues: %w", err)
	}

	// Fail for over length repo owner or name.
	if len(args.RepoOwner) > 39 || len(args.RepoName) > 100 {
		return "invalid parameters to function", errors.New("invalid repo owner or repo name")
	}

	// Fail if repo owner or repo name contain invalid characters.
	if !validGithubRepoName.MatchString(args.RepoOwner) || !validGithubRepoName.MatchString(args.RepoName) {
		return "invalid parameters to function", errors.New("invalid repo owner or repo name")
	}

	// Fail for bad issue numbers.
	if args.Number < 1 {
		return "invalid parameters to function", errors.New("invalid issue number")
	}

	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("/github/api/v1/issue?owner=%s&repo=%s&number=%d",
			url.QueryEscape(args.RepoOwner),
			url.QueryEscape(args.RepoName),
			args.Number,
		),
		nil,
	)
	if err != nil {
		return "internal failure", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Mattermost-User-ID", context.RequestingUser.Id)

	resp := p.pluginAPI.Plugin.HTTP(req)
	if resp == nil {
		return "Error: unable to get issue, internal failure", errors.New("failed to get issue, response was nil")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		result, _ := io.ReadAll(resp.Body)
		return "Error: unable to get issue, internal failure", fmt.Errorf("failed to get issue, status code: %v\n body: %v", resp.Status, string(result))
	}

	var issue github.Issue
	err = json.NewDecoder(resp.Body).Decode(&issue)
	if err != nil {
		return "internal failure", fmt.Errorf("failed to decode response: %w", err)
	}

	return formatGithubIssue(&issue), nil
}

type GetJiraIssueArgs struct {
	InstanceURL string   `jsonschema_description:"The URL of the Jira instance to get the issue from. Example: 'https://mattermost.atlassian.net'"`
	IssueKeys   []string `jsonschema_description:"The issue keys of the Jira issues to get. Example: 'MM-1234'"`
}

var validJiraIssueKey = regexp.MustCompile(`^([[:alnum:]]+)-([[:digit:]]+)$`)

func formatJiraIssue(issue *jira.Issue) string {
	result := strings.Builder{}
	result.WriteString("Issue Key: ")
	result.WriteString(issue.Key)
	result.WriteRune('\n')

	if issue.Fields != nil {
		result.WriteString("Summary: ")
		result.WriteString(issue.Fields.Summary)
		result.WriteRune('\n')

		result.WriteString("Description: ")
		result.WriteString(issue.Fields.Description)
		result.WriteRune('\n')

		result.WriteString("Status: ")
		if issue.Fields.Status != nil {
			result.WriteString(issue.Fields.Status.Name)
		} else {
			result.WriteString("Unknown")
		}
		result.WriteRune('\n')

		result.WriteString("Assignee: ")
		if issue.Fields.Assignee != nil {
			result.WriteString(issue.Fields.Assignee.DisplayName)
		} else {
			result.WriteString("Unassigned")
		}
		result.WriteRune('\n')

		result.WriteString("Created: ")
		result.WriteString(time.Time(issue.Fields.Created).Format(time.RFC1123))
		result.WriteRune('\n')

		result.WriteString("Updated: ")
		result.WriteString(time.Time(issue.Fields.Updated).Format(time.RFC1123))
		result.WriteRune('\n')

		if issue.Fields.Type.Name != "" {
			result.WriteString("Type: ")
			result.WriteString(issue.Fields.Type.Name)
			result.WriteRune('\n')
		}

		if issue.Fields.Labels != nil {
			result.WriteString("Labels: ")
			result.WriteString(strings.Join(issue.Fields.Labels, ", "))
			result.WriteRune('\n')
		}

		if issue.Fields.Reporter != nil {
			result.WriteString("Reporter: ")
			result.WriteString(issue.Fields.Reporter.DisplayName)
			result.WriteRune('\n')
		} else if issue.Fields.Creator != nil {
			result.WriteString("Creator: ")
			result.WriteString(issue.Fields.Creator.DisplayName)
			result.WriteRune('\n')
		}

		if issue.Fields.Priority != nil {
			result.WriteString("Priority: ")
			result.WriteString(issue.Fields.Priority.Name)
			result.WriteRune('\n')
		}

		if !time.Time(issue.Fields.Duedate).IsZero() {
			result.WriteString("Due Date: ")
			result.WriteString(time.Time(issue.Fields.Duedate).Format(time.RFC1123))
			result.WriteRune('\n')
		}

		if issue.Fields.TimeTracking != nil {
			if issue.Fields.TimeTracking.OriginalEstimate != "" {
				result.WriteString("Original Estimate: ")
				result.WriteString(issue.Fields.TimeTracking.OriginalEstimate)
				result.WriteRune('\n')
			}
			if issue.Fields.TimeTracking.TimeSpent != "" {
				result.WriteString("Time Spent: ")
				result.WriteString(issue.Fields.TimeTracking.TimeSpent)
				result.WriteRune('\n')
			}
			if issue.Fields.TimeTracking.RemainingEstimate != "" {
				result.WriteString("Remaining Estimate: ")
				result.WriteString(issue.Fields.TimeTracking.RemainingEstimate)
				result.WriteRune('\n')
			}
		}

		if issue.Fields.Comments != nil {
			for _, comment := range issue.Fields.Comments.Comments {
				result.WriteString(fmt.Sprintf("Comment from %s at %s: %s\n", comment.Author.DisplayName, comment.Created, comment.Body))
			}
		}
	}

	return result.String()
}

var fetchedFields = []string{
	"summary",
	"description",
	"status",
	"assignee",
	"created",
	"updated",
	"issuetype",
	"labels",
	"reporter",
	"creator",
	"priority",
	"duedate",
	"timetracking",
	"comment",
}

func (p *Plugin) getPublicJiraIssues(instanceURL string, issueKeys []string) ([]jira.Issue, error) {
	httpClient := p.createExternalHTTPClient()
	client, err := jira.NewClient(httpClient, instanceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira client: %w", err)
	}
	jql := fmt.Sprintf("key in (%s)", strings.Join(issueKeys, ","))
	issues, _, err := client.Issue.Search(jql, &jira.SearchOptions{Fields: fetchedFields})
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}
	if issues == nil {
		return nil, fmt.Errorf("failed to get issue: issue not found")
	}

	return issues, nil
}

/*func (p *Plugin) getJiraIssueFromPlugin(instanceURL, issueKey, requestingUserID string) (*jira.Issue, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("/jira/api/v2/get-issue-by-key?instance_id=%s&issue_key=%s", instanceURL, issueKey), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Mattermost-User-ID", requestingUserID)

	resp := p.pluginAPI.Plugin.HTTP(req)
	if resp == nil {
		return nil, errors.New("failed to get issue, response was nil")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		result, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get issue, status code: %v\n body: %v", resp.Status, string(result))
	}

	var issue jira.Issue
	err = json.NewDecoder(resp.Body).Decode(&issue)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &issue, nil
}*/

func (p *Plugin) toolGetJiraIssue(context llm.ConversationContext, argsGetter llm.ToolArgumentGetter) (string, error) {
	var args GetJiraIssueArgs
	err := argsGetter(&args)
	if err != nil {
		return "invalid parameters to function", fmt.Errorf("failed to get arguments for tool GetJiraIssue: %w", err)
	}

	// Fail for over-length issue key. or doesn't look like an issue key
	for _, issueKey := range args.IssueKeys {
		if len(issueKey) > 50 || !validJiraIssueKey.MatchString(issueKey) {
			return "invalid parameters to function", errors.New("invalid issue key")
		}
	}

	issues, err := p.getPublicJiraIssues(args.InstanceURL, args.IssueKeys)
	if err != nil {
		return "internal failure", err
	}

	result := strings.Builder{}
	for i := range issues {
		result.WriteString(formatJiraIssue(&issues[i]))
		result.WriteString("------\n")
	}

	return result.String(), nil
}

// getBuiltInTools returns the built-in tools that are available to all users.
// isDM is true if the response will be in a DM with the user. More tools are available in DMs because of security properties.
func (p *Plugin) getBuiltInTools(isDM bool, bot *Bot) []llm.Tool {
	builtInTools := []llm.Tool{}

	if isDM {
		builtInTools = append(builtInTools, llm.Tool{
			Name:        "GetChannelPosts",
			Description: "Get the most recent posts from a Mattermost channel. Returns posts in the format 'username: message'",
			Schema:      GetChannelPosts{},
			Resolver: func(context llm.ConversationContext, argsGetter llm.ToolArgumentGetter) (string, error) {
				return p.toolResolveGetChannelPosts(context, argsGetter, bot)
			},
		})

		builtInTools = append(builtInTools, llm.Tool{
			Name:        "LookupMattermostUser",
			Description: "Lookup a Mattermost user by their username. Available information includes: username, full name, email, nickname, position, locale, timezone, last activity, and status.",
			Schema:      LookupMattermostUserArgs{},
			Resolver:    p.toolResolveLookupMattermostUser,
		})

		// GitHub plugin tools
		status, err := p.pluginAPI.Plugin.GetPluginStatus("github")
		if err != nil && !errors.Is(err, pluginapi.ErrNotFound) {
			p.API.LogError("failed to get github plugin status", "error", err.Error())
		} else if status != nil && status.State == model.PluginStateRunning {
			builtInTools = append(builtInTools, llm.Tool{
				Name:        "GetGithubIssue",
				Description: "Retrieve a single GitHub issue by owner, repo, and issue number.",
				Schema:      GetGithubIssueArgs{},
				Resolver:    p.toolGetGithubIssue,
			})
		}

		// Jira plugin tools
		builtInTools = append(builtInTools, llm.Tool{
			Name:        "GetJiraIssue",
			Description: "Retrieve a single Jira issue by issue key.",
			Schema:      GetJiraIssueArgs{},
			Resolver:    p.toolGetJiraIssue,
		})
	}

	return builtInTools
}

func (p *Plugin) getDefaultToolsStore(bot *Bot, isDM bool) llm.ToolStore {
	if bot == nil || bot.cfg.DisableTools {
		return llm.NewNoTools()
	}
	store := llm.NewToolStore(&p.pluginAPI.Log, p.getConfiguration().EnableLLMTrace)
	store.AddTools(p.getBuiltInTools(isDM, bot))
	return store
}
