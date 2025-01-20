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

const (
	ServiceTypeOpenAI           = "openai"
	ServiceTypeOpenAICompatible = "openaicompatible"
	ServiceTypeAzure            = "azure"
	ServiceTypeAskSage          = "asksage"
	ServiceTypeAnthropic        = "anthropic"
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
	// Basic validation
	if c.Name == "" || c.DisplayName == "" || c.Service.Type == "" {
		return false
	}

	// Validate access levels are within bounds
	if c.ChannelAccessLevel < ChannelAccessLevelAll || c.ChannelAccessLevel > ChannelAccessLevelNone {
		return false
	}
	if c.UserAccessLevel < UserAccessLevelAll || c.UserAccessLevel > UserAccessLevelNone {
		return false
	}

	// Service-specific validation
	switch c.Service.Type {
	case ServiceTypeOpenAI:
		return c.Service.APIKey != ""
	case ServiceTypeOpenAICompatible, ServiceTypeAzure:
		return c.Service.APIKey != "" && c.Service.APIURL != ""
	case ServiceTypeAnthropic:
		return c.Service.APIKey != ""
	case ServiceTypeAskSage:
		return c.Service.Username != "" && c.Service.Password != ""
	default:
		return false
	}
}
