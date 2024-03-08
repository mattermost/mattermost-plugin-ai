package ai

type ServiceConfig struct {
	Name                    string `json:"name"`
	ServiceName             string `json:"serviceName"`
	APIKey                  string `json:"apiKey"`
	OrgID                   string `json:"orgId"`
	DefaultModel            string `json:"defaultModel"`
	URL                     string `json:"url"`
	Username                string `json:"username"`
	Password                string `json:"password"`
	TokenLimit              int    `json:"tokenLimit"`
	StreamingTimeoutSeconds int    `json:"streamingTimeoutSeconds"`
}
