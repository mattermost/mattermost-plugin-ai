package main

import (
	"strings"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/crspeller/mattermost-plugin-summarize/server/ai"
	"github.com/crspeller/mattermost-plugin-summarize/server/ai/mattermostai"
	"github.com/crspeller/mattermost-plugin-summarize/server/ai/openai"
	"github.com/jmoiron/sqlx"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

const (
	BotUsername = "llmbot"
)

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

	summarizer      ai.Summarizer
	threadAnswerer  ai.ThreadAnswerer
	genericAnswerer ai.GenericAnswerer
	emojiSelector   ai.EmojiSelector
	imageGenerator  ai.ImageGenerator

	openai *openai.OpenAI
}

func (p *Plugin) OnActivate() error {
	p.pluginAPI = pluginapi.NewClient(p.API, p.Driver)

	botID, err := p.pluginAPI.Bot.EnsureBot(&model.Bot{
		Username:    BotUsername,
		DisplayName: "LLM Bot",
		Description: "LLM Bot",
	})
	if err != nil {
		return errors.Wrapf(err, "failed to ensure bot")
	}
	p.botid = botID

	origDB, err := p.pluginAPI.Store.GetMasterDB()
	if err != nil {
		return err
	}
	p.db = sqlx.NewDb(origDB, p.pluginAPI.Store.DriverName())

	builder := sq.StatementBuilder.PlaceholderFormat(sq.Question)
	if p.pluginAPI.Store.DriverName() == model.DatabaseDriverPostgres {
		builder = builder.PlaceholderFormat(sq.Dollar)
	}

	if p.pluginAPI.Store.DriverName() == model.DatabaseDriverMysql {
		p.db.MapperFunc(func(s string) string { return s })
	}

	p.registerCommands()

	openAI := openai.New(p.getConfiguration().OpenAIAPIKey)
	mattermostAI := mattermostai.New(p.getConfiguration().MattermostAIUrl, p.getConfiguration().MattermostAISecret)

	switch p.getConfiguration().Summarizer {
	case "openai":
		p.summarizer = openAI
	case "mattermostai":
		p.summarizer = mattermostAI
	}

	switch p.getConfiguration().ThreadAnswerer {
	case "openai":
		p.threadAnswerer = openAI
	case "mattermostai":
		p.threadAnswerer = mattermostAI
	}

	switch p.getConfiguration().GenericAnswerer {
	case "openai":
		p.genericAnswerer = openAI
	case "mattermostai":
		p.genericAnswerer = mattermostAI
	}

	switch p.getConfiguration().EmojiSelector {
	case "openai":
		p.emojiSelector = openAI
	case "mattermostai":
		p.emojiSelector = mattermostAI
	}

	switch p.getConfiguration().ImageGenerator {
	case "openai":
		p.imageGenerator = openAI
	case "mattermostai":
		p.imageGenerator = mattermostAI
	}

	p.openai = openAI

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
		err = p.processUserRequestToBot(post, channel)
		if err != nil {
			p.pluginAPI.Log.Error("Unable to process bot reqeust: " + err.Error())
			return
		}
	}

	// We are mentioned
	if strings.Contains(post.Message, "@"+BotUsername) {
		err = p.processUserRequestToBot(post, channel)
		if err != nil {
			p.pluginAPI.Log.Error("Unable to process bot mention: " + err.Error())
			return
		}
	}
}
