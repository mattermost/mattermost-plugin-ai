package ai

type ServiceConfig struct {
	Name                    string `json:"name"`
	Type                    string `json:"type"`
	APIKey                  string `json:"apiKey"`
	OrgID                   string `json:"orgId"`
	DefaultModel            string `json:"defaultModel"`
	APIURL                  string `json:"apiURL"`
	Username                string `json:"username"`
	Password                string `json:"password"`
	TokenLimit              int    `json:"tokenLimit"`
	StreamingTimeoutSeconds int    `json:"streamingTimeoutSeconds"`
}

type BotConfig struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	DisplayName        string        `json:"displayName"`
	CustomInstructions string        `json:"customInstructions"`
	Service            ServiceConfig `json:"service"`
}

func (c *BotConfig) IsValid() bool {
	isInvalid := c.Name == "" ||
		c.DisplayName == "" ||
		c.Service.Type == "" ||
		(c.Service.Type == "openaicompatable" && c.Service.APIURL == "") ||
		(c.Service.Type != "asksage" && c.Service.APIKey == "")
	return !isInvalid
}
