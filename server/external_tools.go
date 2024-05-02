package main

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
	"github.com/mattermost/mattermost-plugin-ai/server/tools/superface"
	"github.com/mattermost/mattermost-plugin-ai/server/tools/zapier"
)

func (p *Plugin) getThirdPartyTools(isDM bool) []ai.Tool {
	thirdPartyTools := []ai.Tool{}

	config := p.getConfiguration()

	if len(config.ExternalTools) == 0 {
		return thirdPartyTools
	}

	for _, tool := range config.ExternalTools {
		switch tool.Provider {
		case "superface":
			getter := superface.New(tool.URL, tool.AuthToken)
			tools, err := getter.ListTools("")
			if err != nil {
				// handle
				fmt.Println(fmt.Errorf("error occurred fetching tools from superface"))
			}
			thirdPartyTools = append(thirdPartyTools, tools...)
		case "zapier":
			getter := zapier.New(tool.URL, tool.AuthToken)
			tools, err := getter.ListTools("")
			if err != nil {
				// handle
				fmt.Println(fmt.Errorf("error occurred fetching tools from zapier"))
			}
			thirdPartyTools = append(thirdPartyTools, tools...)
		}
	}

	return thirdPartyTools
}
