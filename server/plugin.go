package main

import (
	"embed"
	"strings"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/openai"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

const (
	BotUsername = "ai"
)

//go:embed ai/prompts
var promptsFolder embed.FS

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	pluginAPI *pluginapi.Client

	botid string

	db      *sqlx.DB
	builder sq.StatementBuilderType

	imageGenerator ai.ImageGenerator

	llm ai.LanguageModel

	prompts *ai.Prompts
}

func (p *Plugin) OnActivate() error {
	p.pluginAPI = pluginapi.NewClient(p.API, p.Driver)

	botID, err := p.pluginAPI.Bot.EnsureBot(&model.Bot{
		Username:    BotUsername,
		DisplayName: "AI Assistant",
		Description: "Your helpful assistant within Mattermost",
	},
		pluginapi.ProfileImagePath("assets/bot_icon.png"),
	)
	if err != nil {
		return errors.Wrapf(err, "failed to ensure bot")
	}
	p.botid = botID

	if err := p.SetupDB(); err != nil {
		return err
	}

	p.prompts, err = ai.NewPrompts(promptsFolder)
	if err != nil {
		return err
	}

	p.registerCommands()

	openAI := openai.New(p.getConfiguration().OpenAIAPIKey)
	openAICompatible := openai.NewCompatible(p.getConfiguration().OpenAICompatibleKey, p.getConfiguration().OpenAICompatibleUrl, p.getConfiguration().OpenAICompatibleModel)

	switch p.getConfiguration().ImageGenerator {
	case "openai":
		p.imageGenerator = openAI
	case "openaicompatible":
		p.imageGenerator = openAICompatible
	}

	p.llm = openAI

	return nil
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	// Don't respond to ouselves
	if post.UserId == p.botid {
		return
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		p.pluginAPI.Log.Error(err.Error())
		return
	}

	// Check if this is post in the DM channel with the bot
	if channel.Type == model.ChannelTypeDirect && strings.Contains(channel.Name, p.botid) {
		if p.getConfiguration().EnableUseRestrictions {
			if !p.pluginAPI.User.HasPermissionToTeam(post.UserId, p.getConfiguration().OnlyUsersOnTeam, model.PermissionViewTeam) {
				p.pluginAPI.Log.Error("User not on allowed team.")
				return
			}
		}
		err = p.processUserRequestToBot(post, channel)
		if err != nil {
			p.pluginAPI.Log.Error("Unable to process bot reqeust: " + err.Error())
			return
		}
	}

	// We are mentioned
	if strings.Contains(post.Message, "@"+BotUsername) {
		if err := p.checkUsageRestrictions(post.UserId, channel); err != nil {
			p.pluginAPI.Log.Error(err.Error())
			return
		}

		err = p.processUserRequestToBot(post, channel)
		if err != nil {
			p.pluginAPI.Log.Error("Unable to process bot mention: " + err.Error())
			return
		}
	}
}
