package ai

import (
	"io/fs"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

type Prompts struct {
	templates *template.Template
}

const PromptExtension = "tmpl"
const SystemSubTemplateName = ".system"
const UserSubTemplateName = ".user"

// Conviance vars for the filenames in ai/prompts/
const (
	PromptSummarizeThread       = "summarize_thread"
	PromptDirectMessageQuestion = "direct_message_question"
	PromptEmojiSelect           = "emoji_select"
	PromptMeetingSummary        = "meeting_summary"
	PromptMeetingSummaryOnly    = "summary_only"
	PromptMeetingKeyPoints      = "meeting_key_points"
)

func NewPrompts(input fs.FS) (*Prompts, error) {
	templates, err := template.ParseFS(input, "ai/prompts/*")
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse prompt templates")
	}

	return &Prompts{
		templates: templates,
	}, nil

}

func withPromptExtension(filename string) string {
	return filename + "." + PromptExtension
}

func (p *Prompts) ChatCompletion(templateName string, data any) (BotConversation, error) {
	conversation := BotConversation{
		Posts: []Post{},
	}

	template := p.templates.Lookup(withPromptExtension(templateName))
	if template == nil {
		return conversation, errors.New("main template not found")
	}

	if systemTemplate := template.Lookup(templateName + SystemSubTemplateName); systemTemplate != nil {
		systemMessage, err := p.Execute(systemTemplate, data)
		if err != nil {
			return conversation, err
		}

		conversation.Posts = append(conversation.Posts, Post{
			Role:    PostRoleSystem,
			Message: systemMessage,
		})
	}

	if userTemplate := template.Lookup(templateName + UserSubTemplateName); userTemplate != nil {
		userMessage, err := p.Execute(userTemplate, data)
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

func (p *Prompts) Execute(template *template.Template, data any) (string, error) {
	out := &strings.Builder{}
	if err := template.Execute(out, data); err != nil {
		return "", errors.Wrap(err, "unable to execute template")
	}
	return strings.TrimSpace(out.String()), nil
}
