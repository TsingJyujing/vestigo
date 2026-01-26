package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Envelope struct {
	Server            Server            `yaml:"server"`
	EmbeddingSavePath string            `yaml:"embedding_save_path"`
	EmbeddingModels   []EmbeddingModel  `yaml:"embedding_models"`
	GenerationModels  []GenerationModel `yaml:"generation_models"`
}
type Server struct {
	Address  string   `yaml:"address"`
	Database string   `yaml:"database"`
	Tokens   []string `yaml:"tokens"`
}

type EmbeddingModel struct {
	ID     string                 `yaml:"id"`
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

type GenerationModel struct {
	ID     string                 `yaml:"id"`
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

func LoadConfigFromFile(path string) (*Envelope, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	envelope := Envelope{}
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to parse pages_config: %w", err)
	}
	return &envelope, nil
}
