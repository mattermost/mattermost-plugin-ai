package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

const (
	WhisperAPILimit           = 25 * 1000 * 1000 // 25 MB
	ContextTokenMargin        = 1000
	defaultSpellcheckLanguage = "English"
)

func (p *Plugin) processUserRequestToBot(context ai.ConversationContext) error {
	if context.Post.RootId == "" {
		return p.newConversation(context)
	}

	return p.continueConversation(context)
}

func (p *Plugin) newConversation(context ai.ConversationContext) error {
	conversation, err := p.prompts.ChatCompletion(ai.PromptDirectMessageQuestion, context)
	if err != nil {
		return err
	}
	conversation.AddUserPost(context.Post)

	result, err := p.getLLM().ChatCompletion(conversation)
	if err != nil {
		return err
	}

	responsePost := &model.Post{
		ChannelId: context.Channel.Id,
		RootId:    context.Post.Id,
	}
	if err := p.streamResultToNewPost(context.RequestingUser.Id, result, responsePost); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) continueConversation(context ai.ConversationContext) error {
	threadData, err := p.getThreadAndMeta(context.Post.RootId)
	if err != nil {
		return err
	}

	// Special handing for threads started by the bot in responce to a summarization request.
	var result *ai.TextStreamResult
	originalThreadID, ok := threadData.Posts[0].GetProp(ThreadIDProp).(string)
	if ok && originalThreadID != "" {
		threadPost, err := p.pluginAPI.Post.GetPost(originalThreadID)
		if err != nil {
			return err
		}
		threadChannel, err := p.pluginAPI.Channel.Get(threadPost.ChannelId)
		if err != nil {
			return err
		}

		if !p.pluginAPI.User.HasPermissionToChannel(context.Post.UserId, threadChannel.Id, model.PermissionReadChannel) ||
			p.checkUsageRestrictions(context.Post.UserId, threadChannel) != nil {
			responsePost := &model.Post{
				ChannelId: context.Channel.Id,
				RootId:    context.Post.RootId,
				Message:   "Sorry, you no longer have access to the original thread.",
			}
			if err := p.botCreatePost(context.RequestingUser.Id, responsePost); err != nil {
				return err
			}
			return nil
		}

		result, err = p.continueThreadConversation(threadData, originalThreadID, context)
		if err != nil {
			return err
		}
	} else {
		prompt, err := p.prompts.ChatCompletion(ai.PromptDirectMessageQuestion, context)
		if err != nil {
			return err
		}
		prompt.AppendConversation(ai.ThreadToBotConversation(p.botid, threadData.Posts))

		result, err = p.getLLM().ChatCompletion(prompt)
		if err != nil {
			return err
		}
	}

	responsePost := &model.Post{
		ChannelId: context.Channel.Id,
		RootId:    context.Post.RootId,
	}
	if err := p.streamResultToNewPost(context.RequestingUser.Id, result, responsePost); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) continueThreadConversation(questionThreadData *ThreadData, originalThreadID string, context ai.ConversationContext) (*ai.TextStreamResult, error) {
	originalThreadData, err := p.getThreadAndMeta(originalThreadID)
	if err != nil {
		return nil, err
	}
	originalThread := formatThread(originalThreadData)

	context.PromptParameters = map[string]string{"Thread": originalThread}
	prompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeThread, context)
	if err != nil {
		return nil, err
	}
	prompt.AppendConversation(ai.ThreadToBotConversation(p.botid, questionThreadData.Posts))

	result, err := p.getLLM().ChatCompletion(prompt)
	if err != nil {
		return nil, err
	}

	return result, nil
}

const ThreadIDProp = "referenced_thread"

// DM the user with a standard message. Run the inferance
func (p *Plugin) startNewSummaryThread(postIDToSummarize string, context ai.ConversationContext) (*model.Post, error) {
	threadData, err := p.getThreadAndMeta(postIDToSummarize)
	if err != nil {
		return nil, err
	}

	formattedThread := formatThread(threadData)

	context.PromptParameters = map[string]string{"Thread": formattedThread}
	prompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeThread, context)
	if err != nil {
		return nil, err
	}
	summaryStream, err := p.getLLM().ChatCompletion(prompt)
	if err != nil {
		return nil, err
	}

	post := &model.Post{
		Message: fmt.Sprintf("A summary of [this thread](/_redirect/pl/%s):\n", postIDToSummarize),
	}
	post.AddProp(ThreadIDProp, postIDToSummarize)

	if err := p.streamResultToNewDM(summaryStream, context.RequestingUser.Id, post); err != nil {
		return nil, err
	}

	return post, nil
}

func (p *Plugin) selectEmoji(postToReact *model.Post, context ai.ConversationContext) error {
	context.PromptParameters = map[string]string{"Message": postToReact.Message}
	prompt, err := p.prompts.ChatCompletion(ai.PromptEmojiSelect, context)
	if err != nil {
		return err
	}

	emojiName, err := p.getLLM().ChatCompletionNoStream(prompt, ai.WithmaxTokens(25))
	if err != nil {
		return err
	}

	// Do some emoji post processing to hopfully make this an actual emoji.
	emojiName = strings.Trim(strings.TrimSpace(emojiName), ":")

	if _, found := model.GetSystemEmojiId(emojiName); !found {
		p.pluginAPI.Post.AddReaction(&model.Reaction{
			EmojiName: "large_red_square",
			UserId:    p.botid,
			PostId:    postToReact.Id,
		})
		return errors.New("LLM returned somthing other than emoji: " + emojiName)
	}

	if err := p.pluginAPI.Post.AddReaction(&model.Reaction{
		EmojiName: emojiName,
		UserId:    p.botid,
		PostId:    postToReact.Id,
	}); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) spellcheckMessage(message string) (*string, error) {
	context := ai.NewConversationContextParametersOnly(map[string]string{
		"Message":  message,
		"Language": defaultSpellcheckLanguage,
	})
	prompt, err := p.prompts.ChatCompletion(ai.PromptSpellcheck, context)
	if err != nil {
		return nil, err
	}

	result, err := p.getLLM().ChatCompletionNoStream(prompt, ai.WithmaxTokens(128))
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *Plugin) changeTone(tone, message string) (*string, error) {
	context := ai.NewConversationContextParametersOnly(map[string]string{
		"Tone":    tone,
		"Message": message,
	})
	prompt, err := p.prompts.ChatCompletion(ai.PromptChangeTone, context)
	if err != nil {
		return nil, err
	}

	result, err := p.getLLM().ChatCompletionNoStream(prompt, ai.WithmaxTokens(128))
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *Plugin) simplifyText(message string) (*string, error) {
	context := ai.NewConversationContextParametersOnly(map[string]string{
		"Message": message,
	})
	prompt, err := p.prompts.ChatCompletion(ai.PromptSimplifyText, context)
	if err != nil {
		return nil, err
	}

	result, err := p.getLLM().ChatCompletionNoStream(prompt, ai.WithmaxTokens(128))
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *Plugin) aiChangeText(ask, message string) (*string, error) {
	context := ai.NewConversationContextParametersOnly(map[string]string{
		"Ask":     ask,
		"Message": message,
	})
	prompt, err := p.prompts.ChatCompletion(ai.PromptAIChangeText, context)
	if err != nil {
		return nil, err
	}

	result, err := p.getLLM().ChatCompletionNoStream(prompt, ai.WithmaxTokens(128))
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *Plugin) summarizeChannelSince(requestingUser *model.User, channel *model.Channel, since int64) (string, error) {
	posts, err := p.pluginAPI.Post.GetPostsSince(channel.Id, since)
	if err != nil {
		return "", err
	}

	threadData, err := p.getMetadataForPosts(posts)
	if err != nil {
		return "", err
	}

	// Remove deleted posts
	threadData.Posts = slices.DeleteFunc(threadData.Posts, func(post *model.Post) bool {
		return post.DeleteAt != 0
	})

	formattedThread := formatThread(threadData)

	context := ai.NewConversationContext(requestingUser, channel, nil)
	context.PromptParameters = map[string]string{
		"Posts": formattedThread,
	}

	prompt, err := p.prompts.ChatCompletion(ai.PromptSummarizeChannelSince, context)
	if err != nil {
		return "", err
	}

	result, err := p.getLLM().ChatCompletionNoStream(prompt)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (p *Plugin) explainCode(message string) (*string, error) {
	context := ai.NewConversationContextParametersOnly(map[string]string{
		"Message": message,
	})
	prompt, err := p.prompts.ChatCompletion(ai.PromptExplainCode, context)
	if err != nil {
		return nil, err
	}

	result, err := p.getLLM().ChatCompletionNoStream(prompt)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *Plugin) suggestCodeImprovements(message string) (*string, error) {
	context := ai.NewConversationContextParametersOnly(map[string]string{
		"Message": message,
	})
	prompt, err := p.prompts.ChatCompletion(ai.PromptSuggestCodeImprovements, context)
	if err != nil {
		return nil, err
	}

	result, err := p.getLLM().ChatCompletionNoStream(prompt)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
