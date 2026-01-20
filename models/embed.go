package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ollama/ollama/api"
)

const ModelTypeOllama = "ollama"

type BaseEmbeddingModel interface {
	// Embed Convert text to embeddings
	Embed(ctx context.Context, text []string) ([][]float32, error)
}

func LoadEmbeddingModel(modelType string, config map[string]interface{}) (BaseEmbeddingModel, error) {
	if modelType == ModelTypeOllama {
		jsonData, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		ollamaModelInfo := OllamaModelInfo{}
		err = json.Unmarshal(jsonData, &ollamaModelInfo)
		if err != nil {
			return nil, err
		}
		return NewOllamaModel(ollamaModelInfo)
	}
	return nil, fmt.Errorf("unknown model type: %s", modelType)
}

type OllamaModelInfo struct {
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	Endpoint   string `json:"endpoint" default:"http://localhost:11434"`
}

type OllamaModel struct {
	Info   OllamaModelInfo
	client *api.Client
}

func NewOllamaModel(info OllamaModelInfo) (*OllamaModel, error) {
	ollamaUrl, err := url.Parse(info.Endpoint)
	if err != nil {
		return nil, err
	}
	return &OllamaModel{
		Info:   info,
		client: api.NewClient(ollamaUrl, http.DefaultClient),
	}, nil
}

func (o OllamaModel) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	req := api.EmbedRequest{
		Model: o.Info.Model,
		Input: texts,
	}
	if o.Info.Dimensions > 0 {
		req.Dimensions = o.Info.Dimensions
	}
	embeds, err := o.client.Embed(ctx, &req)
	if err != nil {
		return nil, err
	}
	return embeds.Embeddings, nil
}
