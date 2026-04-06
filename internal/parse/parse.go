package parse

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"collector/internal/event"
)

var (
	ParseSuccessTotal atomic.Int64
	ParseErrorTotal   atomic.Int64
)

func ParseEvent(evt *event.Event) {
	ensureAttrs(evt)

	s := strings.TrimSpace(evt.Message)
	if s == "" {
		evt.Attrs["format"] = "empty"
		return
	}

	var raw map[string]any
	if !tryUnmarshalJSON(s, &raw) {
		MarkPlain(evt)
		return
	}

	if ParseMetric(evt, raw) {
		return
	}
	if IsECS(raw) {
		ParseECS(evt, raw)
		return
	}
	ParseJSON(evt, raw)
}

// ParseNormalized parses a raw log line into a NormalizedEvent.
func ParseNormalized(line, sourceName string) *event.NormalizedEvent {
	s := strings.TrimSpace(line)
	if s == "" {
		return emptyEvent(sourceName)
	}

	var raw map[string]any
	if !tryUnmarshalJSON(s, &raw) {
		ParseSuccessTotal.Add(1)
		return plainEvent(line, sourceName)
	}

	var n *event.NormalizedEvent
	switch {
	case isMetricJSON(raw):
		n = metricNormalized(raw, sourceName)
	case IsECS(raw):
		n = ParseECSNormalized(raw, sourceName)
	default:
		n = ParseJSONNormalized(raw, sourceName)
	}

	if n.SrcService == "" {
		slog.Warn("parse: SrcService is empty",
			"source", sourceName,
			"format", n.Format,
			"line_prefix", truncate(line, 120),
		)
		ParseErrorTotal.Add(1)
	} else {
		ParseSuccessTotal.Add(1)
	}

	return n
}

func tryUnmarshalJSON(s string, raw *map[string]any) bool {
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return false
	}
	return json.Unmarshal([]byte(s), raw) == nil
}

func ensureAttrs(evt *event.Event) {
	if evt.Attrs == nil {
		evt.Attrs = make(map[string]any)
	}
}

func isMetricJSON(raw map[string]any) bool {
	_, hasMetric := raw["metric"]
	_, hasValue := raw["value"]
	return hasMetric && hasValue
}

func metricNormalized(raw map[string]any, sourceName string) *event.NormalizedEvent {
	n := &event.NormalizedEvent{
		Format:     "metric_json",
		SourceName: sourceName,
		Timestamp:  time.Now().UTC(),
		Raw:        raw,
	}
	if ts := extractTimestamp(raw); !ts.IsZero() {
		n.Timestamp = ts
	}
	if svc := extractService(raw); svc != "" {
		n.SrcService = svc
	}
	if name, ok := raw["metric"].(string); ok {
		n.Operation = name
	}
	return n
}

func plainEvent(line, sourceName string) *event.NormalizedEvent {
	return &event.NormalizedEvent{
		Timestamp:  time.Now().UTC(),
		Format:     "plain",
		SourceName: sourceName,
		Raw:        map[string]any{"message": line},
	}
}

func emptyEvent(sourceName string) *event.NormalizedEvent {
	return &event.NormalizedEvent{
		Timestamp:  time.Now().UTC(),
		Format:     "empty",
		SourceName: sourceName,
		Raw:        map[string]any{},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}