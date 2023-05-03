package main

import (
	"net/http"
	"strings"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
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

	summarizer Summarizer
}

type Summarizer interface {
	SummarizeThread(thread string) (string, error)
	AnswerQuestionOnThread(thread, question string) (string, error)
}

func (p *Plugin) OnActivate() error {
	p.pluginAPI = pluginapi.NewClient(p.API, p.Driver)

	botID, err := p.pluginAPI.Bot.EnsureBot(&model.Bot{
		Username:    "llmbot",
		DisplayName: "LLM Bot",
		Description: "Testing...",
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

	p.summarizer = NewOpenAISummarizer(p.getConfiguration().OpenAIAPIKey)

	return nil
}

func (p *Plugin) registerCommands() {
	p.API.RegisterCommand(&model.Command{
		Trigger:          "summarize",
		DisplayName:      "Summarize",
		Description:      "Summarize current context",
		AutoComplete:     true,
		AutoCompleteDesc: "Summarize current context",
	})
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := gin.Default()
	router.GET("/summarize", p.handleSummarize)
	router.ServeHTTP(w, r)
}

func (p *Plugin) handleSummarize(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"result": "this is the summary",
	})
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if args == nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, "", http.StatusInternalServerError)
	}

	if !strings.Contains(p.getConfiguration().AllowedUserIDs, args.UserId) {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "User not authorized", nil, "", http.StatusUnauthorized)
	}

	if !strings.Contains(p.getConfiguration().AllowedTeamIDs, args.TeamId) {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "Can't work on this team.", nil, "", http.StatusUnauthorized)
	}

	if !p.getConfiguration().AllowPrivateChannels {
		channel, err := p.pluginAPI.Channel.Get(args.ChannelId)
		if err != nil {
			return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
		}

		if channel.Type != model.ChannelTypeOpen {
			return nil, model.NewAppError("Summarize.ExecuteCommand", "Can't work on private channels.", nil, "", http.StatusUnauthorized)
		}
	}

	split := strings.SplitN(strings.TrimSpace(args.Command), " ", 2)
	command := split[0]
	/*parameters := []string{}
	cmd := ""
	if len(split) > 1 {
		cmd = split[1]
	}
	if len(split) > 2 {
		parameters = split[2:]
	}*/

	if command != "/summarize" {
		return &model.CommandResponse{}, nil
	}

	var response *model.CommandResponse
	var err error
	if len(split) == 1 {
		response, err = p.summarizeCurrentContext(c, args)
	} else {
		question := split[1]
		response, err = p.askThreadQuestion(c, args, question)
	}

	if err != nil {
		return nil, model.NewAppError("Summarize.ExecuteCommand", "app.command.execute.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return response, nil
}

func (p *Plugin) askThreadQuestion(c *plugin.Context, args *model.CommandArgs, question string) (*model.CommandResponse, error) {
	if args.RootId != "" {
		threadData, err := p.getThreadAndMeta(args.RootId)
		if err != nil {
			return nil, err
		}

		formattedThread := formatThread(threadData)
		summary, err := p.summarizer.AnswerQuestionOnThread(formattedThread, question)
		if err != nil {
			return nil, err
		}

		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         summary,
			ChannelId:    args.ChannelId,
		}, nil
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "Channel questions not implmented",
		ChannelId:    args.ChannelId,
	}, nil
}

func (p *Plugin) summarizeCurrentContext(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, error) {
	if args.RootId != "" {
		threadData, err := p.getThreadAndMeta(args.RootId)
		if err != nil {
			return nil, err
		}

		formattedThread := formatThread(threadData)
		summary, err := p.summarizer.SummarizeThread(formattedThread)
		if err != nil {
			return nil, err
		}

		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         summary,
			ChannelId:    args.ChannelId,
		}, nil
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "Channel summarization not implmented",
		ChannelId:    args.ChannelId,
	}, nil
}
