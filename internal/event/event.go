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

// normalizedEvent is the unified event type consumed by all downstream modules.
// Required fields: Timestamp, SrcService.
type NormalizedEvent struct {
	// Identification
	TraceID   string    `json:"trace_id,omitempty"`
	SpanID    string    `json:"span_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`

	// Services
	SrcService string `json:"src_service"`
	DstService string `json:"dst_service,omitempty"`

	// Operation
	Operation  string `json:"operation,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`

	// Metrics
	Latency   time.Duration `json:"latency,omitempty"`
	ErrorRate float64       `json:"error_rate,omitempty"`

	// Meta
	Level      string         `json:"level,omitempty"`
	Format     string         `json:"format,omitempty"`
	SourceName string         `json:"source_name,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}
