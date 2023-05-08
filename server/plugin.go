package main

import (
	"bytes"
	"fmt"
	"image/png"
	"net/http"
	"strings"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-starter-template/server/mattermostai"
	"github.com/mattermost/mattermost-plugin-starter-template/server/openai"
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

	summarizer           Summarizer
	threadAnswerer       ThreadAnswerer
	imageGenerator       ImageGenerator
	threadConversationer ThreadConversationer
	emojiSelector        EmojiSelector
}

func (p *Plugin) OnActivate() error {
	p.pluginAPI = pluginapi.NewClient(p.API, p.Driver)

	botID, err := p.pluginAPI.Bot.EnsureBot(&model.Bot{
		Username:    "llmbot",
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

	switch p.getConfiguration().ThreadConversationer {
	case "openai":
		p.threadConversationer = openAI
	case "mattermostai":
		p.threadConversationer = mattermostAI
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

	return nil
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	// Don't respond to ouselves
	if post.UserId == p.botid {
		return
	}

	// Optimization: We only care about replies
	if post.RootId == "" {
		return
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		p.pluginAPI.Log.Error(err.Error())
		return
	}

	if channel.Type != model.ChannelTypeDirect {
		return
	}

	// Check if this DM channel is with the bot
	if !strings.Contains(channel.Name, p.botid) {
		return
	}

	nextPost, err := p.continueThreadConversation(post.RootId)
	if err != nil {
		p.pluginAPI.Log.Error(err.Error())
		return
	}

	if err := p.pluginAPI.Post.CreatePost(&model.Post{
		UserId:    p.botid,
		Message:   nextPost,
		ChannelId: channel.Id,
		RootId:    post.RootId,
	}); err != nil {
		p.pluginAPI.Log.Error(err.Error())
		return
	}
}

func (p *Plugin) continueThreadConversation(rootID string) (string, error) {
	questionThreadData, err := p.getThreadAndMeta(rootID)
	if err != nil {
		return "", err
	}

	originalThreadID := questionThreadData.Posts[0].GetProp(ThreadIDProp).(string)
	if originalThreadID == "" {
		return "", errors.New("Unable to retrive inital thread")
	}

	originalThreadData, err := p.getThreadAndMeta(originalThreadID)
	if err != nil {
		return "", err
	}

	originalThread := formatThread(originalThreadData)

	posts := []string{}
	for _, post := range questionThreadData.Posts {
		posts = append(posts, post.Message)
	}

	nextAnswer, err := p.threadConversationer.ThreadConversation(originalThread, posts)
	if err != nil {
		return "", err
	}

	return nextAnswer, nil
}

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
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := gin.Default()
	router.Use(p.ginlogger)
	router.Use(p.MattermostAuthorizationRequired)
	router.POST("/react/:postid", p.handleReact)
	router.ServeHTTP(w, r)
}

func (p *Plugin) ginlogger(c *gin.Context) {
	c.Next()

	for _, ginErr := range c.Errors {
		p.API.LogError(ginErr.Error())
	}
}

func (p *Plugin) handleReact(c *gin.Context) {
	postID := c.Param("postid")

	post, err := p.pluginAPI.Post.GetPost(postID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if !p.getConfiguration().AllowPrivateChannels {
		channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if channel.Type != model.ChannelTypeOpen {
			c.AbortWithError(http.StatusUnauthorized, errors.New("Can't operate on private channels."))
			return
		}

		if !strings.Contains(p.getConfiguration().AllowedTeamIDs, channel.TeamId) {
			c.AbortWithError(http.StatusUnauthorized, errors.New("Can't operate on this team."))
			return
		}
	}

	emojiName, err := p.emojiSelector.SelectEmoji(post.Message)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if _, found := model.GetSystemEmojiId(emojiName); !found {
		p.pluginAPI.Post.AddReaction(&model.Reaction{
			EmojiName: "large_red_square",
			UserId:    p.botid,
			PostId:    post.Id,
		})
		c.AbortWithError(http.StatusInternalServerError, errors.New("LLM returned somthing other than emoji: "+emojiName))
		return
	}

	p.pluginAPI.Post.AddReaction(&model.Reaction{
		EmojiName: emojiName,
		UserId:    p.botid,
		PostId:    post.Id,
	})

	c.Status(http.StatusOK)
}

func (p *Plugin) MattermostAuthorizationRequired(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if !strings.Contains(p.getConfiguration().AllowedUserIDs, userID) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
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

	if command != "/summarize" && command != "/imagine" {
		return &model.CommandResponse{}, nil
	}

	if command == "/summarize" {
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

	return &model.CommandResponse{}, nil
}

const ThreadIDProp = "referenced_thread"

// DM the user with a standard message. Run the inferance
func (p *Plugin) startNewSummaryThread(rootID string, userID string) (string, error) {
	threadData, err := p.getThreadAndMeta(rootID)
	if err != nil {
		return "", err
	}

	formattedThread := formatThread(threadData)
	summary, err := p.summarizer.SummarizeThread(formattedThread)
	if err != nil {
		return "", err
	}

	post := &model.Post{
		Message: fmt.Sprintf("[Original Thread](/_redirect/pl/%s)\n```\n%s\n```", rootID, summary),
	}
	post.AddProp(ThreadIDProp, rootID)

	if err := p.pluginAPI.Post.DM(p.botid, userID, post); err != nil {
		return "", err
	}

	return post.Id, nil
}

func (p *Plugin) askThreadQuestion(c *plugin.Context, args *model.CommandArgs, question string) (*model.CommandResponse, error) {
	if args.RootId != "" {
		threadData, err := p.getThreadAndMeta(args.RootId)
		if err != nil {
			return nil, err
		}

		formattedThread := formatThread(threadData)
		summary, err := p.threadAnswerer.AnswerQuestionOnThread(formattedThread, question)
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
		postid, err := p.startNewSummaryThread(args.RootId, args.UserId)
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
		imgBytes, err := p.imageGenerator.GenerateImage(prompt)
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
