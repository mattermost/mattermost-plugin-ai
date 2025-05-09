// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const RespondingToProp = "responding_to"
const LLMRequesterUserID = "llm_requester_user_id"
const NoRegen = "no_regen"
const ToolCallProp = "pending_tool_call"

// AIThread represents a user's conversation with an AI
type AIThread struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ChannelID string `json:"channel_id"`
	BotID     string `json:"bot_id"`
	UpdatedAt int64  `json:"updated_at"`
}

// AIBotInfo contains information about an AI bot - not using the one in types.go since it has different JSON fields
type AIBotInfo struct {
	ID                 string                 `json:"id"`
	DisplayName        string                 `json:"displayName"`
	Username           string                 `json:"username"`
	LastIconUpdate     int64                  `json:"lastIconUpdate"`
	DMChannelID        string                 `json:"dmChannelID"`
	ChannelAccessLevel llm.ChannelAccessLevel `json:"channelAccessLevel"`
	ChannelIDs         []string               `json:"channelIDs"`
	UserAccessLevel    llm.UserAccessLevel    `json:"userAccessLevel"`
	UserIDs            []string               `json:"userIDs"`
}

// processUserRequestWithContext is an internal helper that uses an existing context to process a message
func (p *AgentsService) processUserRequestWithContext(bot *Bot, postingUser *model.User, channel *model.Channel, post *model.Post, context *llm.Context) (*llm.TextStreamResult, error) {
	var posts []llm.Post
	if post.RootId == "" {
		// A new conversation
		prompt, err := p.prompts.Format(llm.PromptDirectMessageQuestionSystem, context)
		if err != nil {
			return nil, fmt.Errorf("failed to format prompt: %w", err)
		}
		posts = []llm.Post{
			{
				Role:    llm.PostRoleSystem,
				Message: prompt,
			},
		}
	} else {
		// Continuing an existing conversation
		previousConversation, errThread := p.getThreadAndMeta(post.Id)
		if errThread != nil {
			return nil, fmt.Errorf("failed to get previous conversation: %w", errThread)
		}
		previousConversation.cutoffBeforePostID(post.Id)

		var err error
		posts, err = p.existingConversationToLLMPosts(bot, previousConversation, context)
		if err != nil {
			return nil, fmt.Errorf("failed to convert existing conversation to LLM posts: %w", err)
		}
	}

	posts = append(posts, llm.Post{
		Role:    llm.PostRoleUser,
		Message: post.Message,
	})

	completionRequest := llm.CompletionRequest{
		Posts:   posts,
		Context: context,
	}
	result, err := p.GetLLM(bot.cfg).ChatCompletion(completionRequest)
	if err != nil {
		return nil, err
	}

	go func() {
		request := "Write a short title for the following request. Include only the title and nothing else, no quotations. Request:\n" + post.Message
		if err := p.generateTitle(bot, request, post.Id, context); err != nil {
			p.pluginAPI.Log.Error("Failed to generate title", "error", err.Error())
			return
		}
	}()

	return result, nil
}

func (p *AgentsService) processUserRequestToBot(bot *Bot, postingUser *model.User, channel *model.Channel, post *model.Post) (*llm.TextStreamResult, error) {
	// Create a context with default tools
	context := p.contextBuilder.BuildLLMContextUserRequest(
		bot,
		postingUser,
		channel,
		p.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
	)

	return p.processUserRequestWithContext(bot, postingUser, channel, post, context)
}

func (p *AgentsService) generateTitle(bot *Bot, request string, postID string, context *llm.Context) error {
	titleRequest := llm.CompletionRequest{
		Posts:   []llm.Post{{Role: llm.PostRoleUser, Message: request}},
		Context: context,
	}

	conversationTitle, err := p.GetLLM(bot.cfg).ChatCompletionNoStream(titleRequest, llm.WithMaxGeneratedTokens(25))
	if err != nil {
		return fmt.Errorf("failed to get title: %w", err)
	}

	conversationTitle = strings.Trim(conversationTitle, "\n \"'")

	if err := p.saveTitle(postID, conversationTitle); err != nil {
		return fmt.Errorf("failed to save title: %w", err)
	}

	return nil
}

func (p *AgentsService) existingConversationToLLMPosts(bot *Bot, conversation *ThreadData, context *llm.Context) ([]llm.Post, error) {
	// Handle thread summarization requests
	originalThreadID, ok := conversation.Posts[0].GetProp(ThreadIDProp).(string)
	if ok && originalThreadID != "" && conversation.Posts[0].UserId == bot.mmBot.UserId {
		threadPost, err := p.pluginAPI.Post.GetPost(originalThreadID)
		if err != nil {
			return nil, err
		}
		threadChannel, err := p.pluginAPI.Channel.Get(threadPost.ChannelId)
		if err != nil {
			return nil, err
		}

		if !p.pluginAPI.User.HasPermissionToChannel(context.RequestingUser.Id, threadChannel.Id, model.PermissionReadChannel) ||
			p.checkUsageRestrictions(context.RequestingUser.Id, bot, threadChannel) != nil {
			T := i18n.LocalizerFunc(p.i18n, context.RequestingUser.Locale)
			responsePost := &model.Post{
				ChannelId: context.Channel.Id,
				RootId:    originalThreadID,
				Message:   T("copilot.no_longer_access_error", "Sorry, you no longer have access to the original thread."),
			}
			if err = p.botCreateNonResponsePost(bot.mmBot.UserId, context.RequestingUser.Id, responsePost); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("user no longer has access to original thread")
		}

		analysisType, ok := conversation.Posts[0].GetProp(AnalysisTypeProp).(string)
		if !ok {
			return nil, fmt.Errorf("missing analysis type")
		}

		posts, err := p.getAnalyzeThreadPosts(originalThreadID, context, analysisType)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p.ThreadToLLMPosts(bot, conversation.Posts)...)
		return posts, nil
	}

	// Plain DM conversation
	prompt, err := p.prompts.Format(llm.PromptDirectMessageQuestionSystem, context)
	if err != nil {
		return nil, fmt.Errorf("failed to format prompt: %w", err)
	}
	posts := []llm.Post{
		{
			Role:    llm.PostRoleSystem,
			Message: prompt,
		},
	}
	posts = append(posts, p.ThreadToLLMPosts(bot, conversation.Posts)...)

	return posts, nil
}

// GetAIThreads gets AI conversation threads for a user
func (p *AgentsService) GetAIThreads(userID string) ([]AIThread, error) {
	p.botsLock.RLock()
	defer p.botsLock.RUnlock()

	dmChannelIDs := []string{}
	for _, bot := range p.bots {
		channelName := model.GetDMNameFromIds(userID, bot.mmBot.UserId)
		botDMChannel, err := p.pluginAPI.Channel.GetByName("", channelName, false)
		if err != nil {
			if errors.Is(err, pluginapi.ErrNotFound) {
				// Channel doesn't exist yet, so we'll skip it
				continue
			}
			p.pluginAPI.Log.Error("unable to get DM channel for bot", "error", err, "bot_id", bot.mmBot.UserId)
			continue
		}

		// Extra permissions checks are not totally necessary since a user should always have permission to read their own DMs
		if !p.pluginAPI.User.HasPermissionToChannel(userID, botDMChannel.Id, model.PermissionReadChannel) {
			p.pluginAPI.Log.Debug("user doesn't have permission to read channel", "user_id", userID, "channel_id", botDMChannel.Id, "bot_id", bot.mmBot.UserId)
			continue
		}

		dmChannelIDs = append(dmChannelIDs, botDMChannel.Id)
	}

	return p.getAIThreads(dmChannelIDs)
}

// GetAIBots returns all AI bots available to a user
func (p *AgentsService) GetAIBots(userID string) ([]AIBotInfo, error) {
	p.botsLock.RLock()
	defer p.botsLock.RUnlock()

	// Get the info from all the bots.
	// Put the default bot first.
	bots := make([]AIBotInfo, 0, len(p.bots))
	defaultBotName := p.getConfiguration().DefaultBotName
	for i, bot := range p.bots {
		// Don't return bots the user is excluded from using.
		if p.checkUsageRestrictionsForUser(bot, userID) != nil {
			continue
		}

		// Get the bot DM channel ID. To avoid creating the channel unless nessary
		/// we return "" if the channel doesn't exist.
		dmChannelID := ""
		channelName := model.GetDMNameFromIds(userID, bot.mmBot.UserId)
		botDMChannel, err := p.pluginAPI.Channel.GetByName("", channelName, false)
		if err == nil {
			dmChannelID = botDMChannel.Id
		}

		bots = append(bots, AIBotInfo{
			ID:                 bot.mmBot.UserId,
			DisplayName:        bot.mmBot.DisplayName,
			Username:           bot.mmBot.Username,
			LastIconUpdate:     bot.mmBot.LastIconUpdate,
			DMChannelID:        dmChannelID,
			ChannelAccessLevel: bot.cfg.ChannelAccessLevel,
			ChannelIDs:         bot.cfg.ChannelIDs,
			UserAccessLevel:    bot.cfg.UserAccessLevel,
			UserIDs:            bot.cfg.UserIDs,
		})
		if bot.mmBot.Username == defaultBotName {
			bots[0], bots[i] = bots[i], bots[0]
		}
	}

	return bots, nil
}

// IsSearchEnabled returns whether search functionality is enabled
func (p *AgentsService) IsSearchEnabled() bool {
	return p.search != nil && p.getConfiguration().EmbeddingSearchConfig.Type != ""
}

// IsBasicsLicensed returns whether the basic features are licensed
func (p *AgentsService) IsBasicsLicensed() bool {
	return p.licenseChecker.IsBasicsLicensed()
}

// StopPostStreaming stops streaming to a post
func (p *AgentsService) StopPostStreaming(postID string) {
	p.stopPostStreaming(postID)
}

// CheckUsageRestrictions checks if a user can use a bot in a channel
func (p *AgentsService) CheckUsageRestrictions(userID string, bot *Bot, channel *model.Channel) error {
	return p.checkUsageRestrictions(userID, bot, channel)
}
