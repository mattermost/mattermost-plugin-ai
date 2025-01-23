// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llm

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

//go:generate go run generate_prompt_vars.go

func NewPrompts(input fs.FS) (*Prompts, error) {
	templates, err := template.ParseFS(input, "llm/prompts/*")
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
