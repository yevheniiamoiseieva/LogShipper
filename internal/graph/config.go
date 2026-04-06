package graph

import "time"

type Config struct {
	EventBufSize      int
	EdgeTTL           time.Duration
	StaleScanInterval time.Duration
}

func defaultConfig() Config {
	return Config{
		EventBufSize:      256,
		EdgeTTL:           5 * time.Minute,
		StaleScanInterval: 30 * time.Second,
	}
}

func (c *Config) applyDefaults() {
	if c.EventBufSize <= 0 {
		c.EventBufSize = 256
	}
	if c.EdgeTTL <= 0 {
		c.EdgeTTL = 5 * time.Minute
	}
	if c.StaleScanInterval <= 0 {
		c.StaleScanInterval = 30 * time.Second
	}
}
