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
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			evt.Timestamp = t
		} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
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

	if svc := ecsService(raw); svc != "" {
		evt.Service = svc
	}

	applyTimestamp(evt, raw)

	for k, v := range raw {
		switch k {
		case "ts", "time", "@timestamp", "message", "msg", "log.level", "log":
			continue
		}
		evt.Attrs[k] = v
	}
}

// ParseECSNormalized maps ECS fields into a NormalizedEvent.
func ParseECSNormalized(raw map[string]any, sourceName string) *event.NormalizedEvent {
	n := &event.NormalizedEvent{
		Format:     "ecs_json",
		SourceName: sourceName,
		Raw:        raw,
	}

	if ts, ok := raw["@timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			n.Timestamp = t.UTC()
		} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
			n.Timestamp = t.UTC()
		}
	}
	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now().UTC()
	}

	if logObj, ok := raw["log"].(map[string]any); ok {
		if lvl, ok := logObj["level"].(string); ok {
			n.Level = strings.ToLower(lvl)
		}
	}
	if n.Level == "" {
		if lvl, ok := raw["log.level"].(string); ok {
			n.Level = strings.ToLower(lvl)
		}
	}

	n.SrcService = ecsService(raw)

	if traceObj, ok := raw["trace"].(map[string]any); ok {
		n.TraceID, _ = traceObj["id"].(string)
	}
	if spanObj, ok := raw["span"].(map[string]any); ok {
		n.SpanID, _ = spanObj["id"].(string)
	}

	if httpObj, ok := raw["http"].(map[string]any); ok {
		if respObj, ok := httpObj["response"].(map[string]any); ok {
			if code, ok := respObj["status_code"].(float64); ok {
				n.StatusCode = int(code)
			}
		}
	}

	if evtObj, ok := raw["event"].(map[string]any); ok {
		if ns, ok := evtObj["duration"].(float64); ok && ns > 0 {
			n.Latency = time.Duration(int64(ns))
		}
	}

	var method, urlPath string
	if httpObj, ok := raw["http"].(map[string]any); ok {
		if reqObj, ok := httpObj["request"].(map[string]any); ok {
			method, _ = reqObj["method"].(string)
		}
	}
	if urlObj, ok := raw["url"].(map[string]any); ok {
		urlPath, _ = urlObj["path"].(string)
		if urlPath == "" {
			urlPath, _ = urlObj["full"].(string)
		}
	}
	if method != "" && urlPath != "" {
		n.Operation = strings.ToUpper(method) + " " + urlPath
	} else if method != "" {
		n.Operation = strings.ToUpper(method)
	}

	if dstObj, ok := raw["destination"].(map[string]any); ok {
		n.DstService, _ = dstObj["address"].(string)
	}
	if n.DstService == "" {
		if srvObj, ok := raw["server"].(map[string]any); ok {
			n.DstService, _ = srvObj["address"].(string)
		}
	}

	return n
}

func ecsService(raw map[string]any) string {
	if svcObj, ok := raw["service"].(map[string]any); ok {
		if name, ok := svcObj["name"].(string); ok && name != "" {
			return name
		}
	}
	return ""
}