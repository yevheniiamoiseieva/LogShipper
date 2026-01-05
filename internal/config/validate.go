package config

import "fmt"

func (c *Config) Validate() error {
	if c.Source.Type == "" {
		return fmt.Errorf("source.type is required")
	}

	if c.Sink.Type == "" {
		return fmt.Errorf("sink.type is required")
	}

	return nil
}
