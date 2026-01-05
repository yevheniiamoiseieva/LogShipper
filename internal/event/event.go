package event

import "time"

type Event struct {
	Timestamp time.Time      `json:"ts"`
	Source    string         `json:"source"`
	Service   string         `json:"service"`
	Level     string         `json:"level,omitempty"`
	Message   string         `json:"message"`
	Attrs     map[string]any `json:"attrs,omitempty"`
}
