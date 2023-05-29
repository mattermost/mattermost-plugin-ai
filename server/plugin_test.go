package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/ai/mattermostai"
)

var mattermostAI ai.Summarizer

func BenchmarkCreateAPI(b *testing.B) {
	p := Plugin{}
	p.setConfiguration(&configuration{
		Summarizer: "openai",
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := p.getConfiguration()
		switch config.Summarizer {
		case "openai":
			mattermostAI = mattermostai.New(config.MattermostAIUrl, config.MattermostAISecret)
		case "mattermostai":
			mattermostAI = nil
		case "openaicompatible":
			mattermostAI = nil
		}
	}
}
