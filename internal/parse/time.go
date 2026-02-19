package parse

import (
	"time"

	"collector/internal/event"
)

func applyTimestamp(evt *event.Event, raw map[string]any) {
	if v, ok := raw["ts"]; ok {
		if tsStr, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				evt.Timestamp = t
				return
			}
		}
	}
	if v, ok := raw["time"]; ok {
		if tsStr, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				evt.Timestamp = t
				return
			}
		}
	}
	if v, ok := raw["@timestamp"]; ok {
		if tsStr, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				evt.Timestamp = t
				return
			}
		}
	}
}
