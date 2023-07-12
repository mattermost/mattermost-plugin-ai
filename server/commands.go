package main

import (
	"bytes"
	"image/png"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

func (p *Plugin) registerCommands() {
	p.API.RegisterCommand(&model.Command{
		Trigger:          "summarize",
		DisplayName:      "Summarize",
		Description:      "Summarize current context",
		AutoComplete:     true,
		AutoCompleteDesc: "Summarize current context",
	})

	p.API.RegisterCommand(&model.Command{
		Trigger:          "imagine",
		DisplayName:      "Imagine",
		Description:      "Generate a new image based on the provided text",
		AutoComplete:     true,
		AutoCompleteDesc: "Generate a new image based on the provided text",
	})
	_ = p.API.RegisterCommand(&model.Command{
		Trigger:          "spellcheck",
		DisplayName:      "Spellcheck",
		Description:      "Spellchecks a message",
		AutoComplete:     true,
		AutoCompleteDesc: "Spell check the provided message.",
		AutoCompleteHint: "[message]",
	})
	_ = p.API.RegisterCommand(&model.Command{
		Trigger:          "change_tone",
		DisplayName:      "Change tone",
		Description:      "Change the tone to a message",
		AutoComplete:     true,
		AutoCompleteDesc: "Change the tone of the provided message to the specified mood.",
		AutoCompleteHint: "[tone] [message]",
	})
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if args == nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, "", http.StatusInternalServerError)
	}

	split := strings.SplitN(strings.TrimSpace(args.Command), " ", 2)
	command := split[0]

	if command != "/summarize" && command != "/imagine" && command != "/spellcheck" && command != "/change_tone" {
		return &model.CommandResponse{}, nil
	}

	channel, err := p.pluginAPI.Channel.Get(args.ChannelId)
	if err != nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
	}

	user, err := p.pluginAPI.User.Get(args.UserId)
	if err != nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := p.checkUsageRestrictions(user.Id, channel); err != nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "Not authorized", nil, err.Error(), http.StatusUnauthorized)
	}

	// Need to verify the RootId is actually in the channel specified. The server does not enforce this.
	if args.RootId != "" {
		post, err := p.pluginAPI.Post.GetPost(args.RootId)
		if err != nil {
			return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
		}
		if post.ChannelId != channel.Id {
			return nil, model.NewAppError("Summarize.ExecuteCommand", "Not authorized", nil, "", http.StatusUnauthorized)
		}
	}

	context := p.MakeConversationContext(user, channel, nil)

	if command == "/summarize" {
		var response *model.CommandResponse
		var err error
		response, err = p.summarizeCurrentContext(c, args, context)

		if err != nil {
			return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
		}
		return response, nil
	}

	if command == "/imagine" {
		prompt := strings.Join(split[1:], " ")
		if err := p.imagine(c, args, prompt); err != nil {
			return nil, model.NewAppError("Imagine.ExecuteCommand", "app.imagine.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
		}
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Generating image, please wait.",
			ChannelId:    args.ChannelId,
		}, nil
	}

	if command == "/spellcheck" {
		message := strings.Join(split[1:], " ")
		result, err := p.spellcheckMessage(message)
		if err != nil {
			return nil, model.NewAppError("Imagine.ExecuteCommand", "app.spellcheck.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
		}
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         *result,
			ChannelId:    args.ChannelId,
		}, nil
	}

	if command == "/change_tone" {
		parts := strings.SplitN(split[1], " ", 2)
		tone := strings.ToLower(parts[0])
		result, err := p.changeTone(tone, parts[1])
		if err != nil {
			return nil, model.NewAppError("Imagine.ExecuteCommand", "app.change_tone.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
		}
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         *result,
			ChannelId:    args.ChannelId,
		}, nil
	}

	return &model.CommandResponse{}, nil
}

func (p *Plugin) summarizeCurrentContext(c *plugin.Context, args *model.CommandArgs, context ai.ConversationContext) (*model.CommandResponse, error) {
	if args.RootId != "" {
		postid, err := p.startNewSummaryThread(args.RootId, context)
		if err != nil {
			return nil, err
		}
		return &model.CommandResponse{
			GotoLocation: "/_redirect/pl/" + postid,
		}, nil
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "Channel summarization not implmented",
		ChannelId:    args.ChannelId,
	}, nil
}

func (p *Plugin) imagine(c *plugin.Context, args *model.CommandArgs, prompt string) error {
	go func() {
		imgBytes, err := p.getImageGenerator().GenerateImage(prompt)
		if err != nil {
			p.API.LogError("Unable to generate the new image", "error", err)
			return
		}

		buf := new(bytes.Buffer)
		if err := png.Encode(buf, imgBytes); err != nil {
			p.API.LogError("Unable to parse image", "error", err)
			return
		}

		fileInfo, appErr := p.API.UploadFile(buf.Bytes(), args.ChannelId, "generated-image.png")
		if appErr != nil {
			p.API.LogError("Unable to upload the attachment", "error", appErr)
			return
		}

		_, appErr = p.API.CreatePost(&model.Post{
			Message:   "Image generated by the AI from the text: " + prompt,
			ChannelId: args.ChannelId,
			UserId:    args.UserId,
			FileIds:   []string{fileInfo.Id},
		})
		if appErr != nil {
			p.API.LogError("Unable to post the new message", "error", appErr)
			return
		}
	}()

	return nil
}
