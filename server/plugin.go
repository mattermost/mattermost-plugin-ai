package main

import (
	"embed"
	"fmt"
	"os/exec"
	"sync"

	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/anthropic"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/asksage"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/openai"
	"github.com/mattermost/mattermost-plugin-ai/server/enterprise"
	"github.com/mattermost/mattermost-plugin-ai/server/mmapi"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	BotUsername = "ai"

	CallsRecordingPostType = "custom_calls_recording"
	CallsBotUsername       = "calls"

	ffmpegPluginPath = "./plugins/mattermost-ai/server/dist/ffmpeg"
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

	ffmpegPath string

	db      *sqlx.DB
	builder sq.StatementBuilderType

	prompts *ai.Prompts

	streamingContexts      map[string]PostStreamContext
	streamingContextsMutex sync.Mutex

	licenseChecker *enterprise.LicenseChecker
}

func resolveffmpegPath() string {
	_, standardPathErr := exec.LookPath("ffmpeg")
	if standardPathErr != nil {
		_, pluginPathErr := exec.LookPath(ffmpegPluginPath)
		if pluginPathErr != nil {
			return ""
		}
		return ffmpegPluginPath
	}

	return "ffmpeg"
}

func (p *Plugin) EnsureMainBot() error {
	serviceName := "a third party AI Service"
	if llmConfig, err := p.getActiveLLMConfig(); err == nil {
		serviceName = llmConfig.Name
	}
	botID, err := p.pluginAPI.Bot.EnsureBot(&model.Bot{
		Username:    BotUsername,
		DisplayName: "AI Copilot",
		Description: "Powered by " + serviceName,
	},
		pluginapi.ProfileImagePath("assets/bot_icon.png"),
	)
	if err != nil {
		return fmt.Errorf("failed to ensure bot: %w", err)
	}
	p.botid = botID

	return nil
}

func (p *Plugin) OnActivate() error {
	p.pluginAPI = pluginapi.NewClient(p.API, p.Driver)

	p.licenseChecker = enterprise.NewLicenseChecker(p.pluginAPI)

	if err := p.EnsureMainBot(); err != nil {
		return err
	}

	if err := p.SetupDB(); err != nil {
		return err
	}

	var err error
	p.prompts, err = ai.NewPrompts(promptsFolder, p.getBuiltInTools, p.getThirdPartyTools)
	if err != nil {
		return err
	}

	p.ffmpegPath = resolveffmpegPath()
	if p.ffmpegPath == "" {
		p.pluginAPI.Log.Error("ffmpeg not installed, transcriptions will be disabled.", "error", err)
	}

	p.streamingContexts = map[string]PostStreamContext{}

	return nil
}

func (p *Plugin) getActiveLLMConfig() (ai.ServiceConfig, error) {
	cfg := p.getConfiguration()
	if cfg == nil || cfg.Services == nil || len(cfg.Services) == 0 {
		return ai.ServiceConfig{}, errors.New("no LLM services configured. Please configure a service in the plugin settings")
	}

	if p.licenseChecker.IsMultiLLMLicensed() {
		for _, service := range cfg.Services {
			if service.Name == cfg.LLMGenerator {
				return service, nil
			}
		}
	}

	return cfg.Services[0], nil
}

func (p *Plugin) getLLM() ai.LanguageModel {
	llmServiceConfig, err := p.getActiveLLMConfig()
	if err != nil {
		p.pluginAPI.Log.Error(err.Error())
		return nil
	}

	var llm ai.LanguageModel
	switch llmServiceConfig.ServiceName {
	case "openai":
		llm = openai.New(llmServiceConfig)
	case "openaicompatible":
		llm = openai.NewCompatible(llmServiceConfig)
	case "anthropic":
		llm = anthropic.New(llmServiceConfig)
	case "asksage":
		llm = asksage.New(llmServiceConfig)
	}

	cfg := p.getConfiguration()
	if cfg.EnableLLMTrace {
		llm = NewLanguageModelLogWrapper(p.pluginAPI.Log, llm)
	}

	llm = NewLLMTruncationWrapper(llm)

	return llm
}

func (p *Plugin) getTranscribe() ai.Transcriber {
	cfg := p.getConfiguration()
	var transcriptionService ai.ServiceConfig
	for _, service := range cfg.Services {
		if service.Name == cfg.TranscriptGenerator {
			transcriptionService = service
			break
		}
	}
	switch transcriptionService.ServiceName {
	case "openai":
		return openai.New(transcriptionService)
	case "openaicompatible":
		return openai.NewCompatible(transcriptionService)
	}
	return nil
}

var (
	// ErrNoResponse is returned when no response is posted under a normal condition.
	ErrNoResponse = errors.New("no response")
)

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if err := p.handleMessages(post); err != nil {
		if errors.Is(err, ErrNoResponse) {
			p.pluginAPI.Log.Debug(err.Error())
		} else {
			p.pluginAPI.Log.Error(err.Error())
		}
	}
}

const (
	ActivateAIProp  = "activate_ai"
	FromWebhookProp = "from_webhook"
	FromBotProp     = "from_bot"
	FromPluginProp  = "from_plugin"
)

// handleMessages Handled messages posted. Returns true if a response was posted.
func (p *Plugin) handleMessages(post *model.Post) error {
	// Don't respond to ouselves
	if post.UserId == p.botid {
		return fmt.Errorf("not responding to ourselves: %w", ErrNoResponse)
	}

	// Never respond to remote posts
	if post.RemoteId != nil && *post.RemoteId != "" {
		return fmt.Errorf("not responding to remote posts: %w", ErrNoResponse)
	}

	// Don't respond to plugins unless they ask for it
	if post.GetProp(FromPluginProp) != nil && post.GetProp(ActivateAIProp) == nil {
		return fmt.Errorf("not responding to plugin posts: %w", ErrNoResponse)
	}

	// Don't respond to webhooks
	if post.GetProp(FromWebhookProp) != nil {
		return fmt.Errorf("not responding to webhook posts: %w", ErrNoResponse)
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		return fmt.Errorf("unable to get channel: %w", err)
	}

	postingUser, err := p.pluginAPI.User.Get(post.UserId)
	if err != nil {
		return err
	}

	// Don't respond to other bots unless they ask for it
	if (postingUser.IsBot || post.GetProp(FromBotProp) != nil) && post.GetProp(ActivateAIProp) == nil {
		return fmt.Errorf("not responding to other bots: %w", ErrNoResponse)
	}

	switch {
	// Check we are mentioned like @ai
	case userIsMentionedMarkdown(post.Message, BotUsername):
		return p.handleMentions(post, postingUser, channel)

		// Check if this is post in the DM channel with the bot
	case mmapi.IsDMWith(p.botid, channel):
		return p.handleDMs(channel, postingUser, post)
	}

	return nil
}

func (p *Plugin) handleMentions(post *model.Post, postingUser *model.User, channel *model.Channel) error {
	if err := p.checkUsageRestrictions(postingUser.Id, channel); err != nil {
		return err
	}

	if err := p.processUserRequestToBot(p.MakeConversationContext(postingUser, channel, post)); err != nil {
		return fmt.Errorf("unable to process bot mention: %w", err)
	}

	return nil
}

func (p *Plugin) handleDMs(channel *model.Channel, postingUser *model.User, post *model.Post) error {
	if err := p.checkUsageRestrictionsForUser(postingUser.Id); err != nil {
		return err
	}

	if err := p.processUserRequestToBot(p.MakeConversationContext(postingUser, channel, post)); err != nil {
		return fmt.Errorf("unable to process bot DM: %w", err)
	}

	return nil
}
