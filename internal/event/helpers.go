package event

import (
	"fmt"
	"time"
)

// NewFromRaw builds a NormalizedEvent from a raw map, trying known field aliases.
func NewFromRaw(raw map[string]any, sourceName string) (*NormalizedEvent, error) {
	e := &NormalizedEvent{SourceName: sourceName, Raw: raw}

	for _, key := range []string{"timestamp", "ts", "@timestamp", "time"} {
		if v, ok := raw[key]; ok {
			switch t := v.(type) {
			case time.Time:
				e.Timestamp = t
			case string:
				if p, err := time.Parse(time.RFC3339Nano, t); err == nil {
					e.Timestamp = p
				} else if p, err := time.Parse(time.RFC3339, t); err == nil {
					e.Timestamp = p
				}
			}
			if !e.Timestamp.IsZero() {
				break
			}
		}
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	for _, key := range []string{"src_service", "service", "service.name", "source"} {
		if s, ok := stringVal(raw, key); ok {
			e.SrcService = s
			break
		}
	}
	for _, key := range []string{"dst_service", "destination", "peer.service"} {
		if s, ok := stringVal(raw, key); ok {
			e.DstService = s
			break
		}
	}

	e.TraceID, _ = stringVal(raw, "trace_id")
	e.SpanID, _ = stringVal(raw, "span_id")

	for _, key := range []string{"operation", "method", "http.method", "request"} {
		if s, ok := stringVal(raw, key); ok {
			e.Operation = s
			break
		}
	}

	for _, key := range []string{"status_code", "status", "http.status_code", "code"} {
		if v, ok := raw[key]; ok {
			switch n := v.(type) {
			case int:
				e.StatusCode = n
			case int64:
				e.StatusCode = int(n)
			case float64:
				e.StatusCode = int(n)
			}
			if e.StatusCode != 0 {
				break
			}
		}
	}

	for _, key := range []string{"latency", "duration", "elapsed", "response_time"} {
		if v, ok := raw[key]; ok {
			switch n := v.(type) {
			case time.Duration:
				e.Latency = n
			case float64:
				e.Latency = time.Duration(n * float64(time.Millisecond))
			case int:
				e.Latency = time.Duration(n) * time.Millisecond
			case int64:
				e.Latency = time.Duration(n) * time.Millisecond
			}
			if e.Latency != 0 {
				break
			}
		}
	}

	for _, key := range []string{"level", "log.level", "severity"} {
		if s, ok := stringVal(raw, key); ok {
			e.Level = s
			break
		}
	}
	e.Format, _ = stringVal(raw, "format")

	if err := e.Validate(); err != nil {
		return nil, err
	}
	return e, nil
}

// normalize converts a parse-layer Event into a NormalizedEvent.
func Normalize(evt *Event) *NormalizedEvent {
	n := &NormalizedEvent{
		Timestamp:  evt.Timestamp,
		SrcService: evt.Service,
		Level:      evt.Level,
		SourceName: evt.Source,
		Raw:        make(map[string]any),
	}
	for k, v := range evt.Attrs {
		n.Raw[k] = v
	}
	n.Format, _ = stringVal(evt.Attrs, "format")
	n.TraceID, _ = stringVal(evt.Attrs, "trace_id")
	n.SpanID, _ = stringVal(evt.Attrs, "span_id")
	n.DstService, _ = stringVal(evt.Attrs, "dst_service")
	n.Operation, _ = stringVal(evt.Attrs, "operation")

	if evt.Type == TypeMetric && evt.Metric != "" {
		n.Operation = evt.Metric
		n.Raw["metric_value"] = evt.Value
	}
	if evt.Message != "" {
		n.Raw["message"] = evt.Message
	}
	return n
}

// CorrelationKey returns TraceID if set, otherwise "src->dst:operation".
func (e *NormalizedEvent) CorrelationKey() string {
	if e.TraceID != "" {
		return e.TraceID
	}
	return fmt.Sprintf("%s->%s:%s", e.SrcService, e.DstService, e.Operation)
}

// RiskScore is a placeholder; full implementation tracked in Issue #5.
func (e *NormalizedEvent) RiskScore() float64 {
	return 0.0
}

func stringVal(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}