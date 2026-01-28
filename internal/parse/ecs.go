package parse

import (
	"strings"
	"time"

	"collector/internal/event"
)

func IsECS(raw map[string]any) bool {
	if _, ok := raw["@timestamp"]; ok {
		return true
	}
	if _, ok := raw["ecs.version"]; ok {
		return true
	}
	if _, ok := raw["log.level"]; ok {
		return true
	}
	if logObj, ok := raw["log"].(map[string]any); ok {
		if _, ok := logObj["level"]; ok {
			return true
		}
	}
	return false
}

func ParseECS(evt *event.Event, raw map[string]any) {
	ensureAttrs(evt)
	evt.Attrs["format"] = "ecs_json"

	if evt.Type == "" {
		evt.Type = event.TypeLog
	}

	if ts, ok := raw["@timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			evt.Timestamp = t
		}
	}

	if msg, ok := raw["message"].(string); ok && msg != "" {
		evt.Message = msg
	} else if msg, ok := raw["msg"].(string); ok && msg != "" {
		evt.Message = msg
	}

	if lvl, ok := raw["log.level"].(string); ok && lvl != "" {
		evt.Level = strings.ToLower(lvl)
	} else if logObj, ok := raw["log"].(map[string]any); ok {
		if lvl, ok := logObj["level"].(string); ok && lvl != "" {
			evt.Level = strings.ToLower(lvl)
		}
	}

	applyTimestamp(evt, raw)

	for k, v := range raw {
		if k == "ts" || k == "time" || k == "@timestamp" || k == "message" || k == "msg" || k == "log.level" || k == "log" {
			continue
		}
		evt.Attrs[k] = v
	}
}
