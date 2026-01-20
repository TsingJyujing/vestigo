package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ollama/ollama/api"
)

type SummarizationModel interface {
	// Summarize generates a summary for the given text
	Summarize(ctx context.Context, texts []string) (string, error)
}

func NewSummarizationModel(modelType string, config map[string]interface{}) (SummarizationModel, error) {
	if modelType == "ollama" {
		ollamaModelInfo := OllamaSummarizationModelInfo{}
		jsonData, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonData, &ollamaModelInfo)
		if err != nil {
			return nil, err
		}
		return NewOllamaSummarizationModel(ollamaModelInfo)
	}
	return nil, fmt.Errorf("unknown summarization model type: %s", modelType)
}

type OllamaSummarizationModelInfo struct {
	Model    string `json:"model"`
	Endpoint string `json:"endpoint" default:"http://localhost:11434"`
}

type OllamaSummarizationModel struct {
	Info   OllamaSummarizationModelInfo
	client api.Client
}

func NewOllamaSummarizationModel(info OllamaSummarizationModelInfo) (*OllamaSummarizationModel, error) {
	ollamaUrl, err := url.Parse(info.Endpoint)
	if err != nil {
		return nil, err
	}
	return &OllamaSummarizationModel{
		Info:   info,
		client: *api.NewClient(ollamaUrl, http.DefaultClient),
	}, nil
}

func (o OllamaSummarizationModel) Summarize(ctx context.Context, texts []string) (string, error) {
	useStream := false
	req := api.ChatRequest{
		Model: o.Info.Model,
		Messages: []api.Message{
			{
				Role:    "system",
				Content: "You're a helpful assistant to summarize the extracted text from web page for search engine in webpage's language.",
			},
			{
				Role:    "user",
				Content: strings.Join(texts, "\n\n"),
			},
		},
		Stream: &useStream,
	}
	var respString *string = nil
	err := o.client.Chat(ctx, &req, func(resp api.ChatResponse) error {
		respString = &resp.Message.Content
		return nil
	})
	if err != nil {
		return "", err
	}
	if respString == nil {
		return "", fmt.Errorf("no response from summarization model")
	}
	return *respString, nil
}
