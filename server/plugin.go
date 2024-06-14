package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"
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
	"github.com/mattermost/mattermost-plugin-ai/server/metrics"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	BotUsername = "ai"

	CallsRecordingPostType = "custom_calls_recording"
	CallsBotUsername       = "calls"
	ZoomBotUsername        = "zoom"

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

	ffmpegPath string

	db      *sqlx.DB
	builder sq.StatementBuilderType

	prompts *ai.Prompts

	streamingContexts      map[string]PostStreamContext
	streamingContextsMutex sync.Mutex

	licenseChecker *enterprise.LicenseChecker
	metricsService metrics.Metrics
	metricsHandler http.Handler

	botsLock sync.RWMutex
	bots     []*Bot

	i18n *i18n.Bundle
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

func (p *Plugin) OnActivate() error {
	p.pluginAPI = pluginapi.NewClient(p.API, p.Driver)

	p.licenseChecker = enterprise.NewLicenseChecker(p.pluginAPI)

	p.metricsService = metrics.NewMetrics(metrics.InstanceInfo{
		InstallationID: os.Getenv("MM_CLOUD_INSTALLATION_ID"),
		PluginVersion:  manifest.Version,
	})
	p.metricsHandler = metrics.NewMetricsHandler(p.GetMetrics())

	p.i18n = i18nInit()

	if err := p.MigrateServicesToBots(); err != nil {
		p.pluginAPI.Log.Error("failed to migrate services to bots", "error", err)
		// Don't fail on migration errors
	}

	if err := p.EnsureBots(); err != nil {
		p.pluginAPI.Log.Error("Failed to ensure bots", "error", err)
		// Don't fail on ensure bots errors as this leaves the plugin in an awkward state
		// where it can't be configured from the system console.
	}

	if err := p.SetupDB(); err != nil {
		return err
	}

	var err error
	p.prompts, err = ai.NewPrompts(promptsFolder, p.getBuiltInTools)
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

func (p *Plugin) getLLM(llmBotConfig ai.BotConfig) ai.LanguageModel {
	metrics := p.metricsService.GetMetricsForAIService(llmBotConfig.Name)

	var llm ai.LanguageModel
	switch llmBotConfig.Service.Type {
	case "openai":
		llm = openai.New(llmBotConfig.Service, metrics)
	case "openaicompatible":
		llm = openai.NewCompatible(llmBotConfig.Service, metrics)
	case "anthropic":
		llm = anthropic.New(llmBotConfig.Service, metrics)
	case "asksage":
		llm = asksage.New(llmBotConfig.Service, metrics)
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
	var botConfig ai.BotConfig
	for _, bot := range cfg.Bots {
		if bot.Name == cfg.TranscriptGenerator {
			botConfig = bot
			break
		}
	}
	metrics := p.metricsService.GetMetricsForAIService(botConfig.Name)
	switch botConfig.Service.Type {
	case "openai":
		return openai.New(botConfig.Service, metrics)
	case "openaicompatible":
		return openai.NewCompatible(botConfig.Service, metrics)
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
	if p.IsAnyBot(post.UserId) {
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

	// Check we are mentioned like @ai
	if bot := p.GetBotMentioned(post.Message); bot != nil {
		return p.handleMentions(bot, post, postingUser, channel)
	}

	// Check if this is post in the DM channel with any bot
	if bot := p.GetBotForDMChannel(channel); bot != nil {
		return p.handleDMs(bot, channel, postingUser, post)
	}

	return nil
}

func (p *Plugin) handleMentions(bot *Bot, post *model.Post, postingUser *model.User, channel *model.Channel) error {
	if err := p.checkUsageRestrictions(postingUser.Id, channel); err != nil {
		return err
	}

	if err := p.processUserRequestToBot(bot, p.MakeConversationContext(bot, postingUser, channel, post)); err != nil {
		return fmt.Errorf("unable to process bot mention: %w", err)
	}

	return nil
}

func (p *Plugin) handleDMs(bot *Bot, channel *model.Channel, postingUser *model.User, post *model.Post) error {
	if err := p.checkUsageRestrictionsForUser(postingUser.Id); err != nil {
		return err
	}

	if err := p.processUserRequestToBot(bot, p.MakeConversationContext(bot, postingUser, channel, post)); err != nil {
		return fmt.Errorf("unable to process bot DM: %w", err)
	}

	return nil
}
