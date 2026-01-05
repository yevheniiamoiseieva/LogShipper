package config

type Config struct {
	Source SourceConfig `yaml:"source"`
	Sink   SinkConfig   `yaml:"sink"`
}

type SourceConfig struct {
	Type    string `yaml:"type"`
	Service string `yaml:"service"`
}

type SinkConfig struct {
	Type   string `yaml:"type"`
	Pretty bool   `yaml:"pretty"`
}
