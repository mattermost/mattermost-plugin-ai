package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBotConfig_IsValid(t *testing.T) {
	type fields struct {
		ID                 string
		Name               string
		DisplayName        string
		CustomInstructions string
		Service            ServiceConfig
		EnableVision       bool
		DisableTools       bool
		ChannelAccessLevel ChannelAccessLevel
		ChannelIDs         []string
		UserAccessLevel    UserAccessLevel
		UserIDs            []string
		TeamIDs            []string
		MaxFileSize        int64
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "Valid 0",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openai",
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelAll,
				UserAccessLevel:    UserAccessLevelAll,
			},
			want: true,
		},
		{
			name: "Valid 1",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openai",
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelNone,
				UserAccessLevel:    UserAccessLevelAll,
			},
			want: true,
		},
		{
			name: "Invalid name",
			fields: fields{
				ID:                 "xxx",
				Name:               "", // bad
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openai",
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelAll,
				UserAccessLevel:    UserAccessLevelAll,
			},
			want: false,
		},
		{
			name: "Invalid display name",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "", // bad
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openai",
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelAll,
				UserAccessLevel:    UserAccessLevelAll,
			},
			want: false,
		},
		{
			name: "Invalid service type",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "mattermostllm", // bad
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelAll,
				UserAccessLevel:    UserAccessLevelAll,
			},
			want: false,
		},
		{
			name: "Invalid channel access level < ChannelAccessLevelAll",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openai",
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelAll - 1, // bad
				UserAccessLevel:    UserAccessLevelNone,
			},
			want: false,
		},
		{
			name: "Invalid channel access level > ChannelAccessLevelNone",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openai",
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelNone + 1, // bad
				UserAccessLevel:    UserAccessLevelNone,
			},
			want: false,
		},
		{
			name: "Invalid user access level < UserAccessLevelAll",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openai",
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelAll,
				UserAccessLevel:    UserAccessLevelAll - 1, // bad
			},
			want: false,
		},
		{
			name: "Invalid user access level > UserAccessLevelNone",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openai",
					APIKey:                  "sk-xyz",
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelAll,
				UserAccessLevel:    UserAccessLevelNone + 1, // bad
			},
			want: false,
		},
		{
			name: "OpenAI compatible required API URL",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "openaicompatible",
					APIKey:                  "sk-xyz",
					APIURL:                  "", // bad
					OrgID:                   "org-xyz",
					DefaultModel:            "gpt-40",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelNone + 1,
				UserAccessLevel:    UserAccessLevelNone,
			},
			want: false,
		},
		{
			name: "Ask Sage requires username",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "asksage",
					Username:                "", // bad
					Password:                "topsecret",
					DefaultModel:            "xxx",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelNone + 1,
				UserAccessLevel:    UserAccessLevelNone,
			},
			want: false,
		},
		{
			name: "Ask Sage requires password",
			fields: fields{
				ID:                 "xxx",
				Name:               "xxx",
				DisplayName:        "xxx",
				CustomInstructions: "",
				Service: ServiceConfig{
					Name:                    "Copilot",
					Type:                    "asksage",
					Username:                "myuser",
					Password:                "", // bad
					DefaultModel:            "xxx",
					TokenLimit:              100,
					StreamingTimeoutSeconds: 60,
				},
				ChannelAccessLevel: ChannelAccessLevelNone + 1,
				UserAccessLevel:    UserAccessLevelNone,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &BotConfig{
				ID:                 tt.fields.ID,
				Name:               tt.fields.Name,
				DisplayName:        tt.fields.DisplayName,
				CustomInstructions: tt.fields.CustomInstructions,
				Service:            tt.fields.Service,
				EnableVision:       tt.fields.EnableVision,
				DisableTools:       tt.fields.DisableTools,
				ChannelAccessLevel: tt.fields.ChannelAccessLevel,
				ChannelIDs:         tt.fields.ChannelIDs,
				UserAccessLevel:    tt.fields.UserAccessLevel,
				UserIDs:            tt.fields.UserIDs,
				TeamIDs:            tt.fields.TeamIDs,
				MaxFileSize:        tt.fields.MaxFileSize,
			}
			assert.Equalf(t, tt.want, c.IsValid(), "IsValid()")
		})
	}
}
