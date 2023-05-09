package mattermostai

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"

	"github.com/crspeller/mattermost-plugin-summarize/server/ai"
)

type MattermostAI struct {
	url    string
	secret string
	model  string
}

func New(url string, secret string) *MattermostAI {
	return &MattermostAI{
		url:    url,
		secret: secret,
	}
}

type ImageQueryRequest struct {
	Prompt string `json:"prompt"`
}

type TextQueryRequest struct {
	Prompt string `json:"prompt"`
}

type TextQueryResponse struct {
	Response string `json:"response"`
}

func (s *MattermostAI) SummarizeThread(thread string) (*ai.TextStreamResult, error) {
	prompt := thread + "\nbot, summarize the conversation so far"
	requestBody, err := json.Marshal(TextQueryRequest{Prompt: prompt})
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Post(s.url+"/botQuery", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response TextQueryResponse
	json.Unmarshal(data, &response)

	return ai.NewStreamFromString(response.Response), nil
}

/*func (s *MattermostAI) AnswerQuestionOnThread(thread string, question string) (string, error) {
	prompt := thread + "\nbot, answer the question about the conversation so far: " + question
	requestBody, err := json.Marshal(TextQueryRequest{Prompt: prompt})
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Post(s.url+"/botQuery", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var response TextQueryResponse
	json.Unmarshal(data, &response)

	return response.Response, nil
}*/

func (s *MattermostAI) GenerateImage(prompt string) (image.Image, error) {
	requestBody, err := json.Marshal(ImageQueryRequest{Prompt: prompt})
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Post(s.url+"/generateImage", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	r := bytes.NewReader(data)
	imgData, err := png.Decode(r)
	if err != nil {
		return nil, err
	}

	return imgData, nil
}

func (s *MattermostAI) SelectEmoji(message string) (string, error) {
	requestBody, err := json.Marshal(TextQueryRequest{Prompt: message})
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Post(s.url+"/selectEmoji", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var response TextQueryResponse
	json.Unmarshal(data, &response)

	return response.Response, nil
}

func (s *MattermostAI) ContinueThreadInterrogation(originalThread string, conversation ai.BotConversation) (*ai.TextStreamResult, error) {
	prompt := originalThread + "\nbot, answer the question about the conversation so far: " // + strings.Join(posts, "\n")
	requestBody, err := json.Marshal(TextQueryRequest{Prompt: prompt})
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Post(s.url+"/threadConversation", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response TextQueryResponse
	json.Unmarshal(data, &response)

	return ai.NewStreamFromString(response.Response), nil
}

func (s *MattermostAI) ContinueQuestionThread(conversation ai.BotConversation) (*ai.TextStreamResult, error) {
	return nil, errors.New("not implmented")
}
