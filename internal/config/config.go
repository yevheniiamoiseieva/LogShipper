package config

import "time"

type ResolveConfig struct {
	Static map[string]string `yaml:"static"`
	Docker bool              `yaml:"docker"`
	Cache  CacheConfig       `yaml:"cache"`
}

type CacheConfig struct {
	TTL     string `yaml:"ttl"`
	MaxSize int    `yaml:"max_size"`
}

type GraphConfig struct {
	EventBufSize      int           `yaml:"event_buf_size"`
	EdgeTTL           time.Duration `yaml:"edge_ttl"`
	StaleScanInterval time.Duration `yaml:"stale_scan_interval"`
}

type AnomalyConfig struct {
	WindowSize      int     `yaml:"window_size"`
	Threshold       float64 `yaml:"threshold"`
	CooldownSeconds int     `yaml:"cooldown_seconds"`
	MinSamples      int     `yaml:"min_samples"`
}

type Config struct {
	Sources    map[string]SourceConfig    `yaml:"sources"`
	Transforms map[string]TransformConfig `yaml:"transforms"`
	Sinks      map[string]SinkConfig      `yaml:"sinks"`
	Resolve    ResolveConfig              `yaml:"resolve"`
	Graph      GraphConfig                `yaml:"graph"`
	Anomaly    AnomalyConfig              `yaml:"anomaly"`
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
