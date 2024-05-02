package zapier

import "github.com/mattermost/mattermost-plugin-ai/server/ai"

type Zapier struct {
	superfaceURL string
	authToken    string
}

func New(superfaceURL, authToken string) *Zapier {
	return &Zapier{
		superfaceURL: superfaceURL,
		authToken:    authToken,
	}
}

func (z *Zapier) ListTools(userID string) ([]ai.Tool, error) {
	return []ai.Tool{}, nil
}

func (z *Zapier) Perform(userID, functionName string, arguments any) any {
	return nil
}
