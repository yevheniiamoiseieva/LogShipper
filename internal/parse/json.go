package parse

import (
	"encoding/json"
	"time"

	"collector/internal/event"
)

func ParseJSON(evt *event.Event) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(evt.Message), &raw); err != nil {
		return
	}

	if evt.Attrs == nil {
		evt.Attrs = make(map[string]any)
	}

	if v, ok := raw["ts"]; ok {
		if tsStr, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				evt.Timestamp = t
			}
		}
	}
	if v, ok := raw["time"]; ok {
		if tsStr, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				evt.Timestamp = t
			}
		}
	}

	metricName, _ := raw["metric"].(string)
	valueAny, hasValue := raw["value"]

	if metricName != "" && hasValue {
		if val, ok := valueAny.(float64); ok {
			evt.Type = event.TypeMetric
			evt.Metric = metricName
			evt.Value = val

			evt.Message = ""

			for k, v := range raw {
				if k == "ts" || k == "time" || k == "metric" || k == "value" || k == "message" || k == "msg" {
					continue
				}
				evt.Attrs[k] = v
			}
			return
		}
	}

	if msgStr, ok := raw["message"].(string); ok && msgStr != "" {
		evt.Message = msgStr
	} else if msgStr, ok := raw["msg"].(string); ok && msgStr != "" {
		evt.Message = msgStr
	}

	for k, v := range raw {
		if k == "ts" || k == "time" || k == "message" || k == "msg" {
			continue
		}
		evt.Attrs[k] = v
	}
}
