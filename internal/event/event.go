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
	Type      string         `json:"type"`
	Level     string         `json:"level,omitempty"`
	Message   string         `json:"message,omitempty"`
	Attrs     map[string]any `json:"attrs,omitempty"`
	Metric    string         `json:"metric,omitempty"`
	Value     float64        `json:"value,omitempty"`
}

// NormalizedEvent is the unified type consumed by all downstream modules.
// Required fields: Timestamp, SrcService.
type NormalizedEvent struct {
	TraceID    string         `json:"trace_id,omitempty"`
	SpanID     string         `json:"span_id,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	SrcService string         `json:"src_service"`
	DstService string         `json:"dst_service,omitempty"`
	Operation  string         `json:"operation,omitempty"`
	StatusCode int            `json:"status_code,omitempty"`
	Latency    time.Duration  `json:"latency,omitempty"`
	ErrorRate  float64        `json:"error_rate,omitempty"`
	Level      string         `json:"level,omitempty"`
	Format     string         `json:"format,omitempty"`
	SourceName string         `json:"source_name,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}