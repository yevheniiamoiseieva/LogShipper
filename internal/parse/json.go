package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"collector/internal/event"
)

func ParseJSON(evt *event.Event, raw map[string]any) {
	ensureAttrs(evt)
	evt.Attrs["format"] = "json"

	if evt.Type == "" {
		evt.Type = event.TypeLog
	}

	applyTimestamp(evt, raw)

	if msg, ok := raw["message"].(string); ok && msg != "" {
		evt.Message = msg
	} else if msg, ok := raw["msg"].(string); ok && msg != "" {
		evt.Message = msg
	}

	if lvl := extractLevel(raw); lvl != "" {
		evt.Level = lvl
	}
	if svc := extractService(raw); svc != "" {
		evt.Service = svc
	}

	for k, v := range raw {
		switch k {
		case "ts", "time", "@timestamp", "message", "msg",
			"level", "severity", "lvl", "log_level",
			"service", "service_name", "app":
			continue
		}
		evt.Attrs[k] = v
	}
}

// ParseJSONNormalized parses a raw JSON map into a NormalizedEvent.
func ParseJSONNormalized(raw map[string]any, sourceName string) *event.NormalizedEvent {
	n := &event.NormalizedEvent{
		Format:     "json",
		SourceName: sourceName,
		Raw:        raw,
	}

	n.Timestamp = extractTimestamp(raw)
	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now().UTC()
	}

	n.Level = extractLevel(raw)
	n.SrcService = extractService(raw)
	n.TraceID = firstString(raw, "trace_id", "traceId", "trace.id", "X-Trace-Id", "x-trace-id")
	n.SpanID = firstString(raw, "span_id", "spanId", "span.id")
	n.DstService = firstString(raw, "upstream", "target", "remote_service", "peer.service", "dst_service")
	n.StatusCode = extractStatusCode(raw)
	n.Latency = extractLatency(raw)

	if op := firstString(raw, "operation", "event", "rpc.method"); op != "" {
		n.Operation = op
	} else {
		method := firstString(raw, "method", "http.method")
		url := firstString(raw, "url", "path", "uri", "http.url", "http.path")
		if method != "" && url != "" {
			n.Operation = method + " " + url
		} else if method != "" {
			n.Operation = method
		} else if url != "" {
			n.Operation = url
		}
	}

	return n
}

// NewFromRaw constructs a NormalizedEvent from a raw map and validates it.
func NewFromRaw(raw map[string]any, sourceName string) (*event.NormalizedEvent, error) {
	n := ParseJSONNormalized(raw, sourceName)
	if err := n.Validate(); err != nil {
		return nil, fmt.Errorf("NewFromRaw: %w", err)
	}
	return n, nil
}

func extractLevel(raw map[string]any) string {
	for _, key := range []string{"level", "severity", "lvl", "log_level"} {
		if v, ok := raw[key].(string); ok && v != "" {
			return strings.ToLower(v)
		}
	}
	return ""
}

func extractService(raw map[string]any) string {
	for _, key := range []string{"service", "service_name", "app", "application", "component"} {
		if v, ok := raw[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func extractStatusCode(raw map[string]any) int {
	for _, key := range []string{"status_code", "status", "http.status", "code", "http_status"} {
		v, ok := raw[key]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int(n)
		case string:
			if i, err := strconv.Atoi(n); err == nil {
				return i
			}
		}
	}
	return 0
}

var durationRe = regexp.MustCompile(`^([\d.]+)\s*(ms|s|µs|us|ns)?$`)

func extractLatency(raw map[string]any) time.Duration {
	for _, key := range []string{
		"latency", "duration", "elapsed", "response_time",
		"latency_ms", "duration_ms", "elapsed_ms",
		"latency_s", "duration_s",
		"request_time",
	} {
		v, ok := raw[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case float64:
			if strings.HasSuffix(key, "_s") {
				return time.Duration(val * float64(time.Second))
			}
			return time.Duration(val * float64(time.Millisecond))
		case string:
			m := durationRe.FindStringSubmatch(strings.TrimSpace(val))
			if m == nil {
				continue
			}
			n, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				continue
			}
			switch m[2] {
			case "s":
				return time.Duration(n * float64(time.Second))
			case "µs", "us":
				return time.Duration(n * float64(time.Microsecond))
			case "ns":
				return time.Duration(n)
			default:
				return time.Duration(n * float64(time.Millisecond))
			}
		}
	}
	return 0
}

func firstString(raw map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := raw[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func extractTimestamp(raw map[string]any) time.Time {
	for _, key := range []string{"ts", "time", "@timestamp", "timestamp", "datetime"} {
		v, ok := raw[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			for _, layout := range []string{
				time.RFC3339Nano, time.RFC3339,
				"2006-01-02T15:04:05.999Z",
				"2006-01-02 15:04:05",
			} {
				if t, err := time.Parse(layout, val); err == nil {
					return t.UTC()
				}
			}
		case float64:
			if val > 1e12 {
				return time.UnixMilli(int64(val)).UTC()
			}
			return time.Unix(int64(val), 0).UTC()
		}
	}
	return time.Time{}
}