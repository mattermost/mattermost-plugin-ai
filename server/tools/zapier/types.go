package zapier

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
)

// Payload represents the overall JSON structure
type ExposedFunctionsResponse struct {
	Results           []ExposedFunction `json:"results"`
	ConfigurationLink string            `json:"configuration_link"`
}

// Result represents a single action result
type ExposedFunction struct {
	ID          string                        `json:"id"`
	Description string                        `json:"description"`
	Params      ExposedFunctionResponseParams `json:"params"`
}

type ExposedFunctionResponseParams map[string]interface{}

func (efrp ExposedFunctionResponseParams) ToExposedFunctionParams() ExposedFunctionParams {
	efp := ExposedFunctionParams{
		Type:       "object",
		Nullable:   false,
		Required:   []string{},
		Properties: map[string]Parameter{},
	}

	for key, _ := range efrp {
		efp.Required = append(efp.Required, key)
		parameter := Parameter{
			Type:     "string",
			Nullable: false,
			Title:    key,
		}
		efp.Properties[key] = parameter
	}

	fmt.Println(fmt.Sprintf("%+v", efp))
	return efp
}

type ExposedFunctionParams struct {
	Type       string               `json:"type"`
	Required   []string             `json:"required"`
	Properties map[string]Parameter `json:"properties"`
	Nullable   bool                 `json:"nullable"`
}

type Parameter struct {
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Title    string `json:"title"`
}

func (f *ExposedFunction) ToMattermostAITool() ai.Tool {
	return ai.Tool{
		Name:        f.ID,
		Description: f.Description,
		Schema:      f.Params,
	}
}

// Result represents the details of a calendar event
type ExecuteResult struct {
	Kind      string    `json:"kind"`
	Etag      string    `json:"etag"`
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	HtmlLink  string    `json:"htmlLink"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Summary   string    `json:"summary"`
	Creator   Creator   `json:"creator"`
	Organizer Organizer `json:"organizer"`
	Start     EventTime `json:"start"`
	End       EventTime `json:"end"`
	ICalUID   string    `json:"iCalUID"`
	Sequence  int       `json:"sequence"`
	Reminders Reminders `json:"reminders"`
	EventType string    `json:"eventType"`
	// ... Add other fields as needed based on complete 'Result' object
}

// Creator represents the event creator's email
type Creator struct {
	Email string `json:"email"`
}

// Organizer represents the event organizer
type Organizer struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Self        bool   `json:"self"`
}

// EventTime represents dateTime and timeZone information
type EventTime struct {
	DateTime       time.Time `json:"dateTime"`
	TimeZone       string    `json:"timeZone"`
	DateTimePretty string    `json:"dateTime_pretty"`
	DatePretty     string    `json:"date_pretty"`
	TimePretty     string    `json:"time_pretty"`
}

// Reminders represents reminder settings
type Reminders struct {
	UseDefault bool `json:"useDefault"`
}

// Payload represents the overall JSON structure
type ExecuteResponse struct {
	ID                string                 `json:"id"`
	ActionUsed        string                 `json:"action_used"`
	InputParams       map[string]string      `json:"input_params"`
	ReviewURL         string                 `json:"review_url"`
	Result            ExecuteResult          `json:"result"`
	AdditionalResults []interface{}          `json:"additional_results"` // Using interface{} for flexibility
	ResultFieldLabels map[string]interface{} `json:"result_field_labels"`
	Status            string                 `json:"status"`
	Error             any                    `json:"error"` // 'null' can be of any type
	AssistantHint     any                    `json:"assistant_hint"`
}

func (e *ExecuteResponse) ToString() (string, error) {
	jsonData, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}
