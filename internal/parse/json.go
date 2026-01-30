package parse

import "collector/internal/event"

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

	for k, v := range raw {
		if k == "ts" || k == "time" || k == "@timestamp" || k == "message" || k == "msg" {
			continue
		}
		evt.Attrs[k] = v
	}
}
