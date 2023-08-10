package main

import (
	"embed"
	"os/exec"
	"strings"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/anthropic"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/asksage"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/openai"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

const (
	BotUsername = "ai"

	CallsRecordingPostType = "custom_calls_recording"

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

	p.prompts, err = ai.NewPrompts(promptsFolder, p.getBuiltInTools)
	if err != nil {
		return err
	}

	p.ffmpegPath = resolveffmpegPath()
	if p.ffmpegPath == "" {
		p.pluginAPI.Log.Error("ffmpeg not installed, transcriptions will be disabled.", "error", err)
	}

	p.registerCommands()

	return nil
}

func (p *Plugin) getLLM() ai.LanguageModel {
	cfg := p.getConfiguration()
	var llm ai.LanguageModel
	var llmService ServiceConfig
	for _, service := range cfg.Config.Services {
		if service.Name == cfg.Config.LLMGenerator {
			llmService = service
			break
		}
	}
	switch llmService.ServiceName {
	case "openai":
		llm = openai.New(llmService.APIKey, llmService.DefaultModel)
	case "openaicompatible":
		llm = openai.NewCompatible(llmService.APIKey, llmService.URL, llmService.DefaultModel)
	case "anthropic":
		llm = anthropic.New(llmService.APIKey, llmService.DefaultModel)
	case "asksage":
		llm = asksage.New(llmService.Username, llmService.Password, llmService.DefaultModel)
	}

	if cfg.Config.EnableLLMTrace {
		return NewLanguageModelLogWrapper(p.pluginAPI.Log, llm)
	}

	return llm
}

func (p *Plugin) getImageGenerator() ai.ImageGenerator {
	cfg := p.getConfiguration()
	var imageGeneratorService ServiceConfig
	for _, service := range cfg.Config.Services {
		if service.Name == cfg.Config.ImageGenerator {
			imageGeneratorService = service
			break
		}
	}
	switch imageGeneratorService.ServiceName {
	case "openai":
		return openai.New(imageGeneratorService.APIKey, imageGeneratorService.DefaultModel)
	case "openaicompatible":
		return openai.NewCompatible(imageGeneratorService.APIKey, imageGeneratorService.URL, imageGeneratorService.DefaultModel)
	}

	return nil
}

func (p *Plugin) getTranscribe() ai.Transcriber {
	cfg := p.getConfiguration()
	var transcriptionService ServiceConfig
	for _, service := range cfg.Config.Services {
		if service.Name == cfg.Config.TranscriptGenerator {
			transcriptionService = service
			break
		}
	}
	switch transcriptionService.ServiceName {
	case "openai":
		return openai.New(transcriptionService.APIKey, transcriptionService.DefaultModel)
	case "openaicompatible":
		return openai.NewCompatible(transcriptionService.APIKey, transcriptionService.URL, transcriptionService.DefaultModel)
	}
	return nil
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	// Don't respond to ouselves
	if post.UserId == p.botid {
		return
	}

	// Never respond to remote posts
	if post.RemoteId != nil && *post.RemoteId != "" {
		return
	}

	channel, err := p.pluginAPI.Channel.Get(post.ChannelId)
	if err != nil {
		p.pluginAPI.Log.Error(err.Error())
		return
	}

	// Check if this is post in the DM channel with the bot
	if channel.Type == model.ChannelTypeDirect && strings.Contains(channel.Name, p.botid) {
		postingUser, err := p.pluginAPI.User.Get(post.UserId)
		if err != nil {
			p.pluginAPI.Log.Error(err.Error())
			return
		}

		// We don't talk to other bots
		if postingUser.IsBot {
			return
		}

		if p.getConfiguration().Config.SecurityConfig.EnableUseRestrictions {
			if !p.pluginAPI.User.HasPermissionToTeam(postingUser.Id, p.getConfiguration().Config.SecurityConfig.OnlyUsersOnTeam, model.PermissionViewTeam) {
				p.pluginAPI.Log.Error("User not on allowed team.")
				return
			}
		}
		err = p.processUserRequestToBot(p.MakeConversationContext(postingUser, channel, post))
		if err != nil {
			p.pluginAPI.Log.Error("Unable to process bot reqeust: " + err.Error())
			return
		}
		return
	}

	// We are mentioned
	if userIsMentioned(post.Message, BotUsername) {
		postingUser, err := p.pluginAPI.User.Get(post.UserId)
		if err != nil {
			p.pluginAPI.Log.Error(err.Error())
			return
		}

		// We don't talk to other bots
		if postingUser.IsBot {
			return
		}

		if err := p.checkUsageRestrictions(postingUser.Id, channel); err != nil {
			p.pluginAPI.Log.Error(err.Error())
			return
		}

		err = p.processUserRequestToBot(p.MakeConversationContext(postingUser, channel, post))
		if err != nil {
			p.pluginAPI.Log.Error("Unable to process bot mention: " + err.Error())
			return
		}
		return
	}

	// Its a bot post from the calls plugin
	if post.Type == CallsRecordingPostType && p.getConfiguration().Config.EnableAutomaticCallsSummary {
		if p.getConfiguration().Config.SecurityConfig.EnableUseRestrictions {
			if !strings.Contains(p.getConfiguration().Config.SecurityConfig.AllowedTeamIDs, channel.TeamId) {
				return
			}

			if !p.getConfiguration().Config.SecurityConfig.AllowPrivateChannels {
				if channel.Type != model.ChannelTypeOpen {
					return
				}
			}
		}

		if err := p.handleCallRecordingPost(post, channel); err != nil {
			p.pluginAPI.Log.Error("Unable to process calls recording", "error", err)
			return
		}
		return
	}
}
