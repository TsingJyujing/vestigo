package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const DefaultSystemPrompt = "You're a helpful assistant to summarize the extracted text from web page for search engine in webpage's language."

type GenerationModel interface {
	// Generate generates a summary for the given text
	Generate(ctx context.Context, texts []string) (string, error)
}

func NewGenerationModel(modelType string, config map[string]interface{}) (GenerationModel, error) {
	switch modelType {
	case "ollama":
		ollamaModelInfo := OllamaGenerationModelInfo{}
		jsonData, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonData, &ollamaModelInfo)
		if err != nil {
			return nil, err
		}
		return NewOllamaGenerationModel(ollamaModelInfo)
	case "openai":
		openAIModelInfo := OpenAIGenerationModelInfo{}
		jsonData, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonData, &openAIModelInfo)
		if err != nil {
			return nil, err
		}
		return NewOpenAIGenerationModel(openAIModelInfo)
	}
	return nil, fmt.Errorf("unknown summarization model type: %s", modelType)
}

type OllamaGenerationModelInfo struct {
	Model        string `json:"model"`
	Endpoint     string `json:"endpoint" default:"http://localhost:11434"`
	SystemPrompt string `json:"system_prompt,omitempty"`
}

type OllamaGenerationModel struct {
	Info   OllamaGenerationModelInfo
	client api.Client
}

func NewOllamaGenerationModel(info OllamaGenerationModelInfo) (*OllamaGenerationModel, error) {
	ollamaUrl, err := url.Parse(info.Endpoint)
	if err != nil {
		return nil, err
	}
	if info.SystemPrompt == "" {
		info.SystemPrompt = DefaultSystemPrompt
	}
	return &OllamaGenerationModel{
		Info:   info,
		client: *api.NewClient(ollamaUrl, http.DefaultClient),
	}, nil
}

func (o OllamaGenerationModel) Generate(ctx context.Context, texts []string) (string, error) {
	useStream := false
	req := api.ChatRequest{
		Model: o.Info.Model,
		Messages: []api.Message{
			{
				Role:    "system",
				Content: o.Info.SystemPrompt,
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

type OpenAIGenerationModelInfo struct {
	Model        string `json:"model"`
	Endpoint     string `json:"endpoint"`
	Token        string `json:"token"`
	SystemPrompt string `json:"system_prompt,omitempty"`
}

type OpenAIGenerationModel struct {
	Info   OpenAIGenerationModelInfo
	client openai.Client
}

func NewOpenAIGenerationModel(info OpenAIGenerationModelInfo) (*OpenAIGenerationModel, error) {
	options := make([]option.RequestOption, 0)
	if info.Token != "" {
		options = append(options, option.WithAPIKey(info.Token))
	}
	if info.Endpoint != "" {
		options = append(options, option.WithBaseURL(info.Endpoint))
	}
	if info.SystemPrompt == "" {
		info.SystemPrompt = DefaultSystemPrompt
	}
	return &OpenAIGenerationModel{
		Info:   info,
		client: openai.NewClient(options...),
	}, nil
}

func (o OpenAIGenerationModel) Generate(ctx context.Context, texts []string) (string, error) {
	chatCompletion, err := o.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(o.Info.SystemPrompt),
			openai.UserMessage(strings.Join(texts, "\n\n")),
		},
		Model: o.Info.Model,
	})
	if err != nil {
		return "", err
	}
	return chatCompletion.Choices[0].Message.Content, nil
}
