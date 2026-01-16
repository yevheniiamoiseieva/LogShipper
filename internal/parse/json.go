package parse

import (
	"encoding/json"
	"collector/internal/event"
	"time"
)

func ParseJSON(evt *event.Event) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(evt.Message), &raw); err != nil {
		return
	}

	if evt.Attrs == nil {
		evt.Attrs = make(map[string]any)
	}

	for k, v := range raw {
		switch k {
		case "ts", "time":
			if tsStr, ok := v.(string); ok {
				if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
					evt.Timestamp = t
				}
			}
		case "message", "msg":
			if msgStr, ok := v.(string); ok {
				evt.Message = msgStr
			}
		default:
			evt.Attrs[k] = v
		}
	}
}