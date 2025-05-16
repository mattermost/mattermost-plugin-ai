// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/mattermost/mattermost-plugin-ai/llm"
	"github.com/mattermost/mattermost-plugin-ai/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
)

// HandleToolCall handles tool call approval/rejection
func (p *AgentsService) HandleToolCall(userID string, post *model.Post, channel *model.Channel, acceptedToolIDs []string) error {
	bot := p.GetBotByID(post.UserId)
	if bot == nil {
		return fmt.Errorf("unable to get bot")
	}

	user, err := p.pluginAPI.User.Get(userID)
	if err != nil {
		return err
	}

	toolsJSON := post.GetProp(ToolCallProp)
	if toolsJSON == nil {
		return errors.New("post missing pending tool calls")
	}

	var tools []llm.ToolCall
	unmarshalErr := json.Unmarshal([]byte(toolsJSON.(string)), &tools)
	if unmarshalErr != nil {
		return errors.New("post pending tool calls not valid JSON")
	}

	context := p.contextBuilder.BuildLLMContextUserRequest(
		bot,
		user,
		channel,
		p.contextBuilder.WithLLMContextDefaultTools(bot, mmapi.IsDMWith(bot.mmBot.UserId, channel)),
	)

	for i := range tools {
		if slices.Contains(acceptedToolIDs, tools[i].ID) {
			result, resolveErr := context.Tools.ResolveTool(tools[i].Name, func(args any) error {
				return json.Unmarshal(tools[i].Arguments, args)
			}, context)
			if resolveErr != nil {
				// Maybe in the future we can return this to the user and have a retry. For now just tell the LLM it failed.
				tools[i].Result = "Tool call failed"
				tools[i].Status = llm.ToolCallStatusError
				continue
			}
			tools[i].Result = result
			tools[i].Status = llm.ToolCallStatusSuccess
		} else {
			tools[i].Result = "Tool call rejected by user"
			tools[i].Status = llm.ToolCallStatusRejected
		}
	}

	responseRootID := post.Id
	if post.RootId != "" {
		responseRootID = post.RootId
	}

	// Update post with the tool call results
	resolvedToolsJSON, err := json.Marshal(tools)
	if err != nil {
		return fmt.Errorf("failed to marshal tool call results: %w", err)
	}
	post.AddProp(ToolCallProp, string(resolvedToolsJSON))

	if updateErr := p.pluginAPI.Post.UpdatePost(post); updateErr != nil {
		return fmt.Errorf("failed to update post with tool call results: %w", updateErr)
	}

	// Only continue if at lest one tool call was successful
	if !slices.ContainsFunc(tools, func(tc llm.ToolCall) bool {
		return tc.Status == llm.ToolCallStatusSuccess
	}) {
		return nil
	}

	previousConversation, err := p.getThreadAndMeta(responseRootID)
	if err != nil {
		return fmt.Errorf("failed to get previous conversation: %w", err)
	}
	previousConversation.cutoffBeforePostID(post.Id)
	previousConversation.Posts = append(previousConversation.Posts, post)

	posts, err := p.existingConversationToLLMPosts(bot, previousConversation, context)
	if err != nil {
		return fmt.Errorf("failed to convert existing conversation to LLM posts: %w", err)
	}

	completionRequest := llm.CompletionRequest{
		Posts:   posts,
		Context: context,
	}
	result, err := p.GetLLM(bot.cfg).ChatCompletion(completionRequest)
	if err != nil {
		return fmt.Errorf("failed to get chat completion: %w", err)
	}

	responsePost := &model.Post{
		ChannelId: channel.Id,
		RootId:    responseRootID,
	}
	if err := p.streamResultToNewPost(bot.mmBot.UserId, user.Id, result, responsePost, post.Id); err != nil {
		return fmt.Errorf("failed to stream result to new post: %w", err)
	}

	return nil
}
