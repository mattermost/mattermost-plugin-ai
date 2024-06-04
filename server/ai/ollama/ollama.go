package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Ollama struct {
	client  *http.Client
	baseURL string
}

func New(baseURL string, client *http.Client) *Ollama {
	return &Ollama{client, baseURL}
}

func (o *Ollama) Embed(text string) ([]float32, error) {

	var embeddRequest struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	embeddRequest.Prompt = text
	embeddRequest.Model = "nomic-embed-text"

	jsonEmbeddRequest, err := json.Marshal(embeddRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedd request: %w", err)
	}

	path, err := url.JoinPath(o.baseURL, "api/embeddings")
	if err != nil {
		return nil, fmt.Errorf("failed to join path: %w", err)
	}

	resp, err := o.client.Post(path, "application/json", bytes.NewReader(jsonEmbeddRequest))
	if err != nil {
		return nil, fmt.Errorf("failed embed request: %w", err)
	}
	defer resp.Body.Close()

	var embeddResponse struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&embeddResponse); err != nil {
		return nil, fmt.Errorf("failed to decode embedd response: %w", err)
	}

	return embeddResponse.Embedding, nil
}
