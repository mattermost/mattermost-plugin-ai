// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llm

type ServiceConfig struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	APIKey       string `json:"apiKey"`
	OrgID        string `json:"orgId"`
	DefaultModel string `json:"defaultModel"`
	APIURL       string `json:"apiURL"`
	Username     string `json:"username"`
	Password     string `json:"password"`

	// Renaming the JSON field to inputTokenLimit would require a migration, leaving as is for now.
	InputTokenLimit         int  `json:"tokenLimit"`
	StreamingTimeoutSeconds int  `json:"streamingTimeoutSeconds"`
	SendUserID              bool `json:"sendUserID"`

	// Otherwise known as maxTokens
	OutputTokenLimit int `json:"outputTokenLimit"`
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
	case ServiceTypeOpenAICompatible:
		return c.Service.APIURL != ""
	case ServiceTypeAzure:
		return c.Service.APIKey != "" && c.Service.APIURL != ""
	case ServiceTypeAnthropic:
		return c.Service.APIKey != ""
	default:
		return false
	}
}
