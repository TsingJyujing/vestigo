package config

type Envelope struct {
	Server Server `yaml:"server"`
}
type Server struct {
	Address  string `yaml:"address"`
	Database string `yaml:"database"`
}
