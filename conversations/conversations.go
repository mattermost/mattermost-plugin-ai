// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/bots"
	"github.com/mattermost/mattermost-plugin-ai/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/i18n"
	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost-plugin-ai/streaming"
	"github.com/mattermost/mattermost-plugin-ai/threads"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const RespondingToProp = "responding_to"
const LLMRequesterUserID = "llm_requester_user_id"
const NoRegen = "no_regen"

// Constants from agents package - TODO: consolidate these
const ThreadIDProp = "referenced_thread"
const AnalysisTypeProp = "prompt_type"

// AIThread represents a user's conversation with an AI
type AIThread struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ChannelID string `json:"channel_id"`
	BotID     string `json:"bot_id"`
	UpdatedAt int64  `json:"updated_at"`
}

// LLMContextBuilderInterface is an interface for building LLM contexts
type LLMContextBuilderInterface interface {
	BuildLLMContextUserRequest(bot *bots.Bot, user *model.User, channel *model.Channel, options ...llm.ContextOption) *llm.Context
	WithLLMContextDefaultTools(bot *bots.Bot, isDM bool) llm.ContextOption
}

type Conversations struct {
	prompts          *llm.Prompts
	mmClient         mmapi.Client
	pluginAPI        *pluginapi.Client
	streamingService streaming.Service
	contextBuilder   LLMContextBuilderInterface
	bots             *bots.MMBots
	db               *sqlx.DB
	builder          sq.StatementBuilderType
	licenseChecker   *enterprise.LicenseChecker
	i18n             *i18n.Bundle
	checkUsageFunc   func(userID string, bot *bots.Bot, channel *model.Channel) error
}

func New(
	prompts *llm.Prompts,
	mmClient mmapi.Client,
	pluginAPI *pluginapi.Client,
	streamingService streaming.Service,
	contextBuilder LLMContextBuilderInterface,
	botsService *bots.MMBots,
	db *sqlx.DB,
	builder sq.StatementBuilderType,
	licenseChecker *enterprise.LicenseChecker,
	i18nBundle *i18n.Bundle,
	checkUsageFunc func(userID string, bot *bots.Bot, channel *model.Channel) error,
) *Conversations {
	return &Conversations{
		prompts:          prompts,
		mmClient:         mmClient,
		pluginAPI:        pluginAPI,
		streamingService: streamingService,
		contextBuilder:   contextBuilder,
		bots:             botsService,
		db:               db,
		builder:          builder,
		licenseChecker:   licenseChecker,
		i18n:             i18nBundle,
		checkUsageFunc:   checkUsageFunc,
	}
}

// ProcessUserRequestWithContext is an internal helper that uses an existing context to process a message
func (c *Conversations) ProcessUserRequestWithContext(bot *bots.Bot, postingUser *model.User, channel *model.Channel, post *model.Post, context *llm.Context) (*llm.TextStreamResult, error) {
	var posts []llm.Post
	if post.RootId == "" {
		// A new conversation
		prompt, err := c.prompts.Format(llm.PromptDirectMessageQuestionSystem, context)
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
		previousConversation, errThread := mmapi.GetThreadData(c.mmClient, post.Id)
		if errThread != nil {
			return nil, fmt.Errorf("failed to get previous conversation: %w", errThread)
		}
		previousConversation.CutoffBeforePostID(post.Id)

		var err error
		posts, err = c.existingConversationToLLMPosts(bot, previousConversation, context)
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
	result, err := bot.LLM().ChatCompletion(completionRequest)
	if err != nil {
		return nil, err
	}

	go func() {
		request := "Write a short title for the following request. Include only the title and nothing else, no quotations. Request:\n" + post.Message
		if err := c.GenerateTitle(bot, request, post.Id, context); err != nil {
			c.pluginAPI.Log.Error("Failed to generate title", "error", err.Error())
			return
		}
	}()

	return result, nil
}

// ProcessUserRequest processes a user request to a bot
func (c *Conversations) ProcessUserRequest(bot *bots.Bot, postingUser *model.User, channel *model.Channel, post *model.Post) (*llm.TextStreamResult, error) {
	// Create a context with default tools
	context := c.contextBuilder.BuildLLMContextUserRequest(
		bot,
		postingUser,
		channel,
		c.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.GetMMBot().UserId, channel)),
	)

	return c.ProcessUserRequestWithContext(bot, postingUser, channel, post, context)
}

func (c *Conversations) GenerateTitle(bot *bots.Bot, request string, postID string, context *llm.Context) error {
	titleRequest := llm.CompletionRequest{
		Posts:   []llm.Post{{Role: llm.PostRoleUser, Message: request}},
		Context: context,
	}

	conversationTitle, err := bot.LLM().ChatCompletionNoStream(titleRequest, llm.WithMaxGeneratedTokens(25))
	if err != nil {
		return fmt.Errorf("failed to get title: %w", err)
	}

	conversationTitle = strings.Trim(conversationTitle, "\n \"'")

	if err := c.SaveTitle(postID, conversationTitle); err != nil {
		return fmt.Errorf("failed to save title: %w", err)
	}

	return nil
}

// existingConversationToLLMPosts converts existing conversation to LLM posts format
func (c *Conversations) existingConversationToLLMPosts(bot *bots.Bot, conversation *mmapi.ThreadData, context *llm.Context) ([]llm.Post, error) {
	// Handle thread summarization requests
	originalThreadID, ok := conversation.Posts[0].GetProp(ThreadIDProp).(string)
	if ok && originalThreadID != "" && conversation.Posts[0].UserId == bot.GetMMBot().UserId {
		threadPost, err := c.pluginAPI.Post.GetPost(originalThreadID)
		if err != nil {
			return nil, err
		}
		threadChannel, err := c.pluginAPI.Channel.Get(threadPost.ChannelId)
		if err != nil {
			return nil, err
		}

		if !c.pluginAPI.User.HasPermissionToChannel(context.RequestingUser.Id, threadChannel.Id, model.PermissionReadChannel) ||
			c.checkUsageFunc(context.RequestingUser.Id, bot, threadChannel) != nil {
			T := i18n.LocalizerFunc(c.i18n, context.RequestingUser.Locale)
			responsePost := &model.Post{
				ChannelId: context.Channel.Id,
				RootId:    originalThreadID,
				Message:   T("copilot.no_longer_access_error", "Sorry, you no longer have access to the original thread."),
			}
			if err = c.BotCreateNonResponsePost(bot.GetMMBot().UserId, context.RequestingUser.Id, responsePost); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("user no longer has access to original thread")
		}

		analysisType, ok := conversation.Posts[0].GetProp(AnalysisTypeProp).(string)
		if !ok {
			return nil, fmt.Errorf("missing analysis type")
		}

		posts, err := threads.New(bot.LLM(), c.prompts, c.mmClient).FollowUpAnalyze(originalThreadID, context, analysisType)
		if err != nil {
			return nil, err
		}
		posts = append(posts, c.ThreadToLLMPosts(bot, conversation.Posts)...)
		return posts, nil
	}

	// Plain DM conversation
	prompt, err := c.prompts.Format(llm.PromptDirectMessageQuestionSystem, context)
	if err != nil {
		return nil, fmt.Errorf("failed to format prompt: %w", err)
	}
	posts := []llm.Post{
		{
			Role:    llm.PostRoleSystem,
			Message: prompt,
		},
	}
	posts = append(posts, c.ThreadToLLMPosts(bot, conversation.Posts)...)

	return posts, nil
}

// GetAIThreads gets AI conversation threads for a user
func (c *Conversations) GetAIThreads(userID string) ([]AIThread, error) {
	allBots := c.bots.GetAllBots()

	dmChannelIDs := []string{}
	for _, bot := range allBots {
		channelName := model.GetDMNameFromIds(userID, bot.GetMMBot().UserId)
		botDMChannel, err := c.pluginAPI.Channel.GetByName("", channelName, false)
		if err != nil {
			if errors.Is(err, pluginapi.ErrNotFound) {
				// Channel doesn't exist yet, so we'll skip it
				continue
			}
			c.pluginAPI.Log.Error("unable to get DM channel for bot", "error", err, "bot_id", bot.GetMMBot().UserId)
			continue
		}

		// Extra permissions checks are not totally necessary since a user should always have permission to read their own DMs
		if !c.pluginAPI.User.HasPermissionToChannel(userID, botDMChannel.Id, model.PermissionReadChannel) {
			c.pluginAPI.Log.Debug("user doesn't have permission to read channel", "user_id", userID, "channel_id", botDMChannel.Id, "bot_id", bot.GetMMBot().UserId)
			continue
		}

		dmChannelIDs = append(dmChannelIDs, botDMChannel.Id)
	}

	return c.getAIThreads(dmChannelIDs)
}

// CheckUsageRestrictions checks if a user can use a bot in a channel
func (c *Conversations) CheckUsageRestrictions(userID string, bot *bots.Bot, channel *model.Channel) error {
	if c.checkUsageFunc == nil {
		return nil
	}
	return c.checkUsageFunc(userID, bot, channel)
}

// IsBasicsLicensed checks if the plugin has the required license
func (c *Conversations) IsBasicsLicensed() bool {
	return c.licenseChecker.IsBasicsLicensed()
}

// GetI18nBundle returns the i18n bundle
func (c *Conversations) GetI18nBundle() *i18n.Bundle {
	return c.i18n
}

// StreamToNewDM streams an LLM result to a new DM
func (c *Conversations) StreamToNewDM(ctx context.Context, botID string, stream *llm.TextStreamResult, userID string, post *model.Post, respondingToPostID string) error {
	if c.streamingService == nil {
		return fmt.Errorf("streaming service not initialized")
	}
	return c.streamingService.StreamToNewDM(ctx, botID, stream, userID, post, respondingToPostID)
}

// StopPostStreaming stops streaming to a post
func (c *Conversations) StopPostStreaming(postID string) {
	if c.streamingService != nil {
		c.streamingService.StopStreaming(postID)
	}
}

// SetStreamingService updates the streaming service (used during initialization)
func (c *Conversations) SetStreamingService(service streaming.Service) {
	c.streamingService = service
}
