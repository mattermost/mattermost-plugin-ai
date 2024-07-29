package ai

import (
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"errors"
)

type Prompts struct {
	templates *template.Template
}

const PromptExtension = "tmpl"
const SystemSubTemplateName = ".system"
const UserSubTemplateName = ".user"

// Convenience vars for the filenames in ai/prompts/
const (
	PromptSummarizeThread         = "summarize_thread"
	PromptDirectMessageQuestion   = "direct_message_question"
	PromptEmojiSelect             = "emoji_select"
	PromptMeetingSummary          = "meeting_summary"
	PromptMeetingSummaryOnly      = "summary_only"
	PromptMeetingKeyPoints        = "meeting_key_points"
	PromptSpellcheck              = "spellcheck"
	PromptChangeTone              = "change_tone"
	PromptSimplifyText            = "simplify_text"
	PromptAIChangeText            = "ai_change_text"
	PromptSummarizeChannelSince   = "summarize_channel_since"
	PromptSummarizeChunk          = "summarize_chunk"
	PromptExplainCode             = "explain_code"
	PromptSuggestCodeImprovements = "suggest_code_improvements"
	PromptFindActionItemsSince    = "find_action_items_since"
	PromptFindOpenQuestionsSince  = "find_open_questions_since"
)

func NewPrompts(input fs.FS) (*Prompts, error) {
	templates, err := template.ParseFS(input, "ai/prompts/*")
	if err != nil {
		return nil, fmt.Errorf("unable to parse prompt templates: %w", err)
	}

	return &Prompts{
		templates: templates,
	}, nil
}

func withPromptExtension(filename string) string {
	return filename + "." + PromptExtension
}

func (p *Prompts) ChatCompletion(templateName string, context ConversationContext, tools ToolStore) (BotConversation, error) {
	conversation := BotConversation{
		Posts:   []Post{},
		Context: context,
		Tools:   tools,
	}

	tmpl := p.templates.Lookup(withPromptExtension(templateName))
	if tmpl == nil {
		return conversation, errors.New("main template not found")
	}

	if systemTemplate := tmpl.Lookup(templateName + SystemSubTemplateName); systemTemplate != nil {
		systemMessage, err := p.execute(systemTemplate, context)
		if err != nil {
			return conversation, err
		}

		conversation.Posts = append(conversation.Posts, Post{
			Role:    PostRoleSystem,
			Message: systemMessage,
		})
	}

	if userTemplate := tmpl.Lookup(templateName + UserSubTemplateName); userTemplate != nil {
		userMessage, err := p.execute(userTemplate, context)
		if err != nil {
			return conversation, err
		}

		conversation.Posts = append(conversation.Posts, Post{
			Role:    PostRoleUser,
			Message: userMessage,
		})
	}

	return conversation, nil
}

func (p *Prompts) execute(template *template.Template, data ConversationContext) (string, error) {
	out := &strings.Builder{}
	if err := template.Execute(out, data); err != nil {
		return "", fmt.Errorf("unable to execute template: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}
