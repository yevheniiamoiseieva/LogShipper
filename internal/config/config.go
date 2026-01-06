package config

type Config struct {
	Sources    map[string]SourceConfig    `yaml:"sources"`
	Transforms map[string]TransformConfig `yaml:"transforms"`
	Sinks      map[string]SinkConfig      `yaml:"sinks"`
}

type SourceConfig struct {
	Type    string `yaml:"type"`
	Service string `yaml:"service"`
	Path    string `yaml:"path,omitempty"`
	ContainerID string `yaml:"container_id,omitempty"`
}

type TransformConfig struct {
	Type      string            `yaml:"type"`
	Inputs    []string          `yaml:"inputs"`
	AddFields map[string]string `yaml:"add_fields"`
}

type SinkConfig struct {
	Type   string   `yaml:"type"`
	Inputs []string `yaml:"inputs"`
	Pretty bool     `yaml:"pretty"`
}