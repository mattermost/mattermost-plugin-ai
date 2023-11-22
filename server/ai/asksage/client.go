package asksage

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

const (
	ServerBaseURL = "https://server-nginx.asksage.ai"
	AuthBaseURL   = "https://user-server-cac-gov.asksage.ai"
	RoleUser      = "me"
	RoleGPT       = "gpt"
)

type Client struct {
	AuthToken  string
	HTTPClient *http.Client
}

type Message struct {
	User    string `json:"user"`
	Message string `json:"message"`
}

type QueryParams struct {
	Message         []Message `json:"message"`
	Persona         string    `json:"persona,omitempty"`
	SystemPrompt    string    `json:"system_prompt,omitempty"`
	Dataset         string    `json:"dataset,omitempty"`
	LimitReferences int       `json:"limit_references,omitempty"`
	Temperature     float64   `json:"temperature,omitempty"`
	Live            int       `json:"live,omitempty"`
	Model           string    `json:"model,omitempty"`
}

type FollowUpParams struct {
	Message string `json:"message"`
}

type TokenizerParams struct {
	Content string `json:"content"`
	Model   string `json:"model,omitempty"`
}

type GetTokenParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CompletionResponse struct {
	Response   string `json:"response"`
	Message    string `json:"message"`
	References string `json:"references"`
}

type Persona struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
}

type Dataset string

func NewClient(authToken string) *Client {
	return &Client{
		AuthToken:  authToken,
		HTTPClient: &http.Client{},
	}
}

func (c *Client) Login(params GetTokenParams) error {
	var response struct {
		Response struct {
			AccessToken string `json:"access_token"`
		}
	}
	err := c.doAuth("POST", "/get-token", &params, &response)
	if err != nil {
		return err
	}
	c.AuthToken = response.Response.AccessToken

	return nil
}

func (c *Client) Query(params QueryParams) (*CompletionResponse, error) {
	response := &CompletionResponse{}
	if err := c.doServer("POST", "/query", &params, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) FollowUpQuestions(params FollowUpParams) (*CompletionResponse, error) {
	response := &CompletionResponse{}
	if err := c.doServer("POST", "/follow-up-questions", &params, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) GetPersonas() ([]Persona, error) {
	var response struct {
		Response []Persona `json:"response"`
	}
	if err := c.doServer("POST", "/get-personas", nil, &response); err != nil {
		return nil, err
	}
	return response.Response, nil
}

func (c *Client) GetDatasets() ([]Dataset, error) {
	var response struct {
		Response []Dataset `json:"dataset"`
	}
	if err := c.doServer("POST", "/get-datasets", nil, &response); err != nil {
		return nil, err
	}
	return response.Response, nil
}

func (c *Client) doServer(method, path string, body, result interface{}) error {
	fullUrl := ServerBaseURL + path
	return c.do(method, fullUrl, body, result)
}

func (c *Client) doAuth(method, path string, body, result interface{}) error {
	fullUrl := AuthBaseURL + path
	return c.do(method, fullUrl, body, result)
}

func (c *Client) do(method, path string, body interface{}, result interface{}) error {
	var req *http.Request
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyBuffer := bytes.NewBuffer(jsonBody)

		req, err = http.NewRequest(method, path, bodyBuffer)
		if err != nil {
			return err
		}
	} else {
		var err error
		req, err = http.NewRequest(method, path, nil)
		if err != nil {
			return err
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-tokens", c.AuthToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "unable to read response body on error: "+resp.Status)
		}

		return errors.New("non 200 response from asksage: " + resp.Status + "\nBody:\n" + string(body))
	}

	// Decode response body into specified struct
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return err
	}

	return nil
}
