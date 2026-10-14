// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package evals

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/bifrost"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/prompts"
)

// Default models for each provider. Update these when bumping model versions,
// together with the matching fallbacks in .github/workflows/ci.yml.
const (
	DefaultOpenAIModel    = "gpt-5.6-terra"
	DefaultAnthropicModel = "claude-sonnet-5-5"
	DefaultAzureModel     = "gpt-5.6-terra"
	DefaultMistralModel   = "mistral-large-latest"
	DefaultBedrockModel   = "global.anthropic.claude-sonnet-5-5"
	DefaultCohereModel    = "command-a-plus-05-2026"

	// DefaultGraderModel is pinned separately from DefaultOpenAIModel so that
	// bumping the model under test does not silently change the judge.
	DefaultGraderModel = "gpt-5.6-terra"
)

type EvalT struct {
	*testing.T
	*Eval

	// recorded counts graded results in the current run; lastFailure holds
	// the most recent assertion message, reported if nothing was graded.
	recorded    int
	lastFailure string
}

// Errorf captures assertion failures (testify reports through it) so a run
// that fails before grading can be recorded with its cause.
func (e *EvalT) Errorf(format string, args ...any) {
	e.Helper()
	e.lastFailure = fmt.Sprintf(format, args...)
	e.T.Errorf(format, args...)
}

type Eval struct {
	LLM       llm.LanguageModel
	GraderLLM llm.LanguageModel
	Prompts   *llm.Prompts

	runNumber int
}

// simpleProviders describes providers that need only an API key and a model.
// Providers with extra requirements (azure, openaicompatible, bedrock) are
// handled as explicit branches in serviceConfigFor.
var simpleProviders = map[string]struct {
	serviceType  string
	keyEnv       string
	modelEnv     string
	defaultModel string
}{
	"openai":    {llm.ServiceTypeOpenAI, "OPENAI_API_KEY", "OPENAI_MODEL", DefaultOpenAIModel},
	"anthropic": {llm.ServiceTypeAnthropic, "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL", DefaultAnthropicModel},
	"mistral":   {llm.ServiceTypeMistral, "MISTRAL_API_KEY", "MISTRAL_MODEL", DefaultMistralModel},
	"cohere":    {llm.ServiceTypeCohere, "COHERE_API_KEY", "COHERE_MODEL", DefaultCohereModel},
}

// agentUnderTest mirrors the defaults of an agent created in the admin UI, so
// evals exercise the same provider settings customers get out of the box.
var agentUnderTest = llm.BotConfig{ReasoningEnabled: true}

// graderAgent keeps grading cheap and stable; reasoning adds latency and
// variance without improving single-criterion rubric checks.
var graderAgent = llm.BotConfig{ReasoningEnabled: false}

// resolveModel picks the model to use: the explicit override, then the
// provider's model environment variable, then the provider default.
func resolveModel(override, modelEnv, defaultModel string) string {
	if override != "" {
		return override
	}
	if model := os.Getenv(modelEnv); model != "" {
		return model
	}
	return defaultModel
}

// serviceConfigFor builds the service configuration an admin would create for
// the provider, reading credentials from environment variables.
func serviceConfigFor(providerName string, modelOverride string) (llm.ServiceConfig, error) {
	name := strings.ToLower(providerName)

	if p, ok := simpleProviders[name]; ok {
		apiKey := os.Getenv(p.keyEnv)
		if apiKey == "" {
			return llm.ServiceConfig{}, fmt.Errorf("%s environment variable is not set", p.keyEnv)
		}
		return llm.ServiceConfig{
			ID:           name,
			Type:         p.serviceType,
			APIKey:       apiKey,
			DefaultModel: resolveModel(modelOverride, p.modelEnv, p.defaultModel),
		}, nil
	}

	switch name {
	case "azure":
		apiKey := os.Getenv("AZURE_OPENAI_API_KEY")
		if apiKey == "" {
			return llm.ServiceConfig{}, errors.New("AZURE_OPENAI_API_KEY environment variable is not set")
		}
		apiURL := os.Getenv("AZURE_OPENAI_ENDPOINT")
		if apiURL == "" {
			return llm.ServiceConfig{}, errors.New("AZURE_OPENAI_ENDPOINT environment variable is not set")
		}
		return llm.ServiceConfig{
			ID:           name,
			Type:         llm.ServiceTypeAzure,
			APIKey:       apiKey,
			APIURL:       apiURL,
			DefaultModel: resolveModel(modelOverride, "AZURE_OPENAI_MODEL", DefaultAzureModel),
		}, nil

	case "openaicompatible":
		apiURL := os.Getenv("OPENAI_COMPATIBLE_API_URL")
		if apiURL == "" {
			return llm.ServiceConfig{}, errors.New("OPENAI_COMPATIBLE_API_URL environment variable is not set")
		}
		model := resolveModel(modelOverride, "OPENAI_COMPATIBLE_MODEL", "")
		if model == "" {
			return llm.ServiceConfig{}, errors.New("OPENAI_COMPATIBLE_MODEL environment variable is not set")
		}
		return llm.ServiceConfig{
			ID:   name,
			Type: llm.ServiceTypeOpenAICompatible,
			// API key is optional for local LLMs
			APIKey:       os.Getenv("OPENAI_COMPATIBLE_API_KEY"),
			APIURL:       apiURL,
			DefaultModel: model,
		}, nil

	case "bedrock":
		region := os.Getenv("AWS_BEDROCK_REGION")
		if region == "" {
			return llm.ServiceConfig{}, errors.New("AWS_BEDROCK_REGION environment variable is not set")
		}
		return llm.ServiceConfig{
			ID:                 name,
			Type:               llm.ServiceTypeBedrock,
			Region:             region,
			AWSAccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			AWSSecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			DefaultModel:       resolveModel(modelOverride, "AWS_BEDROCK_MODEL", DefaultBedrockModel),
		}, nil

	default:
		return llm.ServiceConfig{}, fmt.Errorf("unknown provider: %s", providerName)
	}
}

// createProvider builds the provider through the same bifrost service and
// agent construction the plugin uses for configured agents.
func createProvider(providerName string, modelOverride string, agent llm.BotConfig) (llm.LanguageModel, error) {
	serviceConfig, err := serviceConfigFor(providerName, modelOverride)
	if err != nil {
		return nil, err
	}

	client, err := bifrost.NewServiceClient(serviceConfig, nil)
	if err != nil {
		return nil, err
	}

	agentLLM, err := client.ForAgent(agent)
	if err != nil {
		client.Shutdown()
		return nil, err
	}
	return agentLLM, nil
}

func NewEval() (*Eval, error) {
	// Default to OpenAI for backward compatibility
	return NewEvalWithProvider("openai")
}

// NewEvalWithProvider creates an Eval instance with a specific provider
func NewEvalWithProvider(providerName string) (*Eval, error) {
	// Setup prompts
	prompts, err := llm.NewPrompts(prompts.PromptsFolder)
	if err != nil {
		return nil, err
	}

	// Create provider (uses environment variables)
	provider, err := createProvider(providerName, "", agentUnderTest)
	if err != nil {
		return nil, err
	}

	// Setup grader LLM (separate from main LLM)
	graderLLM, err := createGraderLLM()
	if err != nil {
		return nil, fmt.Errorf("failed to create grader LLM: %w", err)
	}

	return &Eval{
		Prompts:   prompts,
		LLM:       provider,
		GraderLLM: graderLLM,
	}, nil
}

// createGraderLLM creates a separate LLM for grading based on environment variables.
// Defaults to OpenAI if not specified.
func createGraderLLM() (llm.LanguageModel, error) {
	graderProvider := os.Getenv("GRADER_LLM_PROVIDER")
	if graderProvider == "" {
		graderProvider = "openai"
	}

	graderModel := os.Getenv("GRADER_LLM_MODEL")
	if graderModel == "" && graderProvider == "openai" {
		graderModel = DefaultGraderModel
	}

	return createProvider(graderProvider, graderModel, graderAgent)
}

func NumEvalsOrSkip(t *testing.T) int {
	t.Helper()
	numEvals, err := strconv.Atoi(os.Getenv("GOEVALS"))
	if err != nil || numEvals < 1 {
		t.Skip("Skipping evals. Use GOEVALS=1 flag to run.")
	}

	return numEvals
}

func Run(t *testing.T, name string, f func(e *EvalT)) {
	numEvals := NumEvalsOrSkip(t)

	// Get list of providers to test
	providers := getProvidersToTest()

	// Run evaluations for each provider
	for _, providerName := range providers {
		// Try to create eval for this provider
		eval, err := NewEvalWithProvider(providerName)
		if err != nil {
			t.Logf("Skipping %s provider: %v", providerName, err)
			continue
		}

		e := &EvalT{T: t, Eval: eval}

		// Prefix test name with provider
		testName := fmt.Sprintf("[%s] %s", providerName, name)

		t.Run(testName, func(t *testing.T) {
			e.T = t
			for i := range numEvals {
				e.runNumber = i
				runOnce(e, f)
			}
		})
	}
}

// runOnce runs a single trial. The deferred check also runs when a require
// assertion stops the trial early via runtime.Goexit.
func runOnce(e *EvalT, f func(e *EvalT)) {
	e.recorded = 0
	e.lastFailure = ""
	defer func() {
		if e.recorded == 0 && e.lastFailure != "" {
			recordError(e, e.lastFailure)
		}
	}()
	f(e)
}

// getProvidersToTest returns the list of providers to test based on LLM_PROVIDER env var
func getProvidersToTest() []string {
	providerEnv := os.Getenv("LLM_PROVIDER")
	if providerEnv == "" {
		providerEnv = "all"
	}

	providerEnv = strings.ToLower(strings.TrimSpace(providerEnv))

	// Handle "all" case
	if providerEnv == "all" {
		return []string{"openai", "anthropic", "azure", "mistral", "bedrock", "cohere"}
	}

	// Handle comma-separated list (a single provider is a one-element list)
	providers := strings.Split(providerEnv, ",")
	result := make([]string, 0, len(providers))
	for _, p := range providers {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
