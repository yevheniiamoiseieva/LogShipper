package event

import "time"

const (
	TypeLog    = "log"
	TypeMetric = "metric"
)

type Event struct {
	Timestamp time.Time      `json:"ts"`
	Source    string         `json:"source"`
	Service   string         `json:"service"`
	Type  string `json:"type"`
	Level string `json:"level,omitempty"`

	// logs
	Message string         `json:"message,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`

	// metrics
	Metric string  `json:"metric,omitempty"`
	Value  float64 `json:"value,omitempty"`
}
