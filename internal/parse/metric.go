package parse

import (
	"collector/internal/event"
)

func ParseMetric(evt *event.Event, raw map[string]any) bool {
	metricName, _ := raw["metric"].(string)
	valueAny, hasValue := raw["value"]
	if metricName == "" || !hasValue {
		return false
	}
	val, ok := valueAny.(float64)
	if !ok {
		return false
	}

	ensureAttrs(evt)
	evt.Attrs["format"] = "metric_json"
	evt.Type = event.TypeMetric
	evt.Metric = metricName
	evt.Value = val
	evt.Message = ""

	for k, v := range raw {
		if k == "ts" || k == "time" || k == "@timestamp" || k == "metric" || k == "value" || k == "message" || k == "msg" {
			continue
		}
		evt.Attrs[k] = v
	}

	applyTimestamp(evt, raw)
	return true
}
