package llm

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
	SendUserID              bool   `json:"sendUserID"`
}

type ChannelAccessLevel int

const (
	ChannelAccessLevelAll ChannelAccessLevel = iota
	ChannelAccessLevelAllow
	ChannelAccessLevelBlock
	ChannelAccessLevelNone
)

type UserAccessLevel int

const (
	UserAccessLevelAll UserAccessLevel = iota
	UserAccessLevelAllow
	UserAccessLevelBlock
	UserAccessLevelNone
)

type BotConfig struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	DisplayName        string             `json:"displayName"`
	CustomInstructions string             `json:"customInstructions"`
	Service            ServiceConfig      `json:"service"`
	EnableVision       bool               `json:"enableVision"`
	DisableTools       bool               `json:"disableTools"`
	ChannelAccessLevel ChannelAccessLevel `json:"channelAccessLevel"`
	ChannelIDs         []string           `json:"channelIDs"`
	UserAccessLevel    UserAccessLevel    `json:"userAccessLevel"`
	UserIDs            []string           `json:"userIDs"`
	TeamIDs            []string           `json:"teamIDs"`
	MaxFileSize        int64              `json:"maxFileSize"`
}

func (c *BotConfig) IsValid() bool {
	isInvalid := c.Name == "" ||
		c.DisplayName == "" ||
		c.Service.Type == "" ||
		((c.Service.Type == "openaicompatible" || c.Service.Type == "azure") && c.Service.APIURL == "") ||
		(c.Service.Type != "asksage" && c.Service.Type != "openaicompatible" && c.Service.Type != "azure" && c.Service.APIKey == "")
	return !isInvalid
}
