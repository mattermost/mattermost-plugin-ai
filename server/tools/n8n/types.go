package n8n

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-ai/server/ai"
)

type N8NListResponse struct {
	Tools []DataItem `json:"data"`
}

type DataItem struct {
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	Active      bool        `json:"active"`
	Nodes       []Node      `json:"nodes"`
	Connections Connections `json:"connections"`
	Settings    Settings    `json:"settings"`
	StaticData  interface{} `json:"staticData"` // Potentially change this type
	Meta        Meta        `json:"meta"`
	PinData     interface{} `json:"pinData"` // Potentially change this type
	VersionId   string      `json:"versionId"`
	Tags        []string    `json:"tags"`
}

type Node struct {
	Parameters  NodeParameters  `json:"parameters"`
	Id          string          `json:"id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	ExecuteOnce bool            `json:"executeOnce"`
	NotesInFlow bool            `json:"notesInFlow"`
	Credentials NodeCredentials `json:"credentials"`
	Notes       string          `json:"notes"`
}

type NodeParameters struct {
	// Add fields here based on the contents of 'parameters' in your JSON
	Operation          string      `json:"operation"`
	Filters            interface{} `json:"filters"` // Potentially change this type
	Path               string      `json:"path"`
	HTTPMethod         string      `json:"httpMethod"`
	ResponseMode       string      `json:"responseMode"`
	ResponseData       string      `json:"responseData"`
	Options            interface{} `json:"options"` // Potentially change this type
	Method             string      `json:"method"`
	Url                string      `json:"url"`
	Authentication     string      `json:"authentication"`
	NodeCredentialType string      `json:"nodeCredentialType"`
	SendBody           bool        `json:"sendBody"`
	BodyParameters     struct {
		Parameters []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"parameters"`
	} `json:"bodyParameters"`
}

type NodeCredentials struct {
	TodoistApi struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"todoistApi"`
}

type Connections struct {
	Webhook struct {
		Main [][]struct {
			Node  string `json:"node"`
			Type  string `json:"type"`
			Index int    `json:"index"`
		} `json:"main"`
	} `json:"Webhook"`
}

type Settings struct {
	ExecutionOrder string `json:"executionOrder"`
}

type Meta struct {
	TemplateCredsSetupCompleted bool `json:"templateCredsSetupCompleted"`
}

func generateJSONSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	properties := schema["properties"].(map[string]interface{})

	for key, value := range data {
		switch v := value.(type) {
		case string:
			properties[key] = map[string]interface{}{"type": "string"}
		case float64:
			properties[key] = map[string]interface{}{"type": "number"}
		case bool:
			properties[key] = map[string]interface{}{"type": "boolean"}
		default:
			fmt.Println("Unsupported type:", v)
		}
	}

	return schema
}

func (d DataItem) ToMattermostAITool() ai.Tool {
	tool := ai.Tool{}

	tool.Name = d.Nodes[len(d.Nodes)-1].Parameters.Path
	tool.Description = d.Nodes[len(d.Nodes)-1].Notes
	tool.HTTPMethod = d.Nodes[len(d.Nodes)-1].Parameters.HTTPMethod

	if tool.Name == "" {
		for _, node := range d.Nodes {
			if node.Parameters.Path != "" {
				tool.Name = node.Parameters.Path
				break
			}
		}
	}

	if tool.Description == "" {
		for _, node := range d.Nodes {
			if node.Notes != "" {
				tool.Description = node.Notes
				// We don't break
			}
		}
	}

	if tool.HTTPMethod == "" {
		for _, node := range d.Nodes {
			if node.Parameters.HTTPMethod != "" {
				tool.HTTPMethod = node.Parameters.HTTPMethod
				break
			}
		}
	}

	if tool.HTTPMethod == "" {
		tool.HTTPMethod = http.MethodGet
	}

	var data map[string]interface{}
	if d.Nodes[0].Notes != "" {
		reader := strings.NewReader(d.Nodes[0].Notes) // Replace with your API response bytes
		if err := json.NewDecoder(reader).Decode(&data); err != nil {
			fmt.Println("Error parsing JSON:", err)
			return tool
		}
		tool.Schema = generateJSONSchema(data)
	}
	tool.IsRawMessage = true

	return tool
}

func (p *PerformResponse) ToString() (string, error) {
	jsonData, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}
