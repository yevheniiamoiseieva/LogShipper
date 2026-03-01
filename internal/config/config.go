package config

// ResolveConfig holds resolver configuration.
type ResolveConfig struct {
	Static map[string]string `yaml:"static"`
	Docker bool              `yaml:"docker"`
	Cache  CacheConfig       `yaml:"cache"`
}

type CacheConfig struct {
	TTL     string `yaml:"ttl"`      // e.g. "30s", "5m"
	MaxSize int    `yaml:"max_size"` // 0 = unlimited
}

type Config struct {
	Sources    map[string]SourceConfig    `yaml:"sources"`
	Transforms map[string]TransformConfig `yaml:"transforms"`
	Sinks      map[string]SinkConfig      `yaml:"sinks"`
	Resolve    ResolveConfig              `yaml:"resolve"`
}

type SourceConfig struct {
	Type        string `yaml:"type"`
	Service     string `yaml:"service"`
	Path        string `yaml:"path,omitempty"`
	ContainerID string `yaml:"container_id,omitempty"`
}

type TransformConfig struct {
	Type      string            `yaml:"type"`
	Inputs    []string          `yaml:"inputs"`
	AddFields map[string]string `yaml:"add_fields"`
	Case      string            `yaml:"case,omitempty"`
}

type SinkConfig struct {
	Type   string   `yaml:"type"`
	Inputs []string `yaml:"inputs"`
	Pretty bool     `yaml:"pretty"`
}