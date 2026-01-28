package parse

import (
	"encoding/json"
	"strings"

	"collector/internal/event"
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

func tryUnmarshalJSON(s string, raw *map[string]any) bool {
	if !(s[0] == '{' || s[0] == '[') {
		return false
	}
	return json.Unmarshal([]byte(s), raw) == nil
}

func ensureAttrs(evt *event.Event) {
	if evt.Attrs == nil {
		evt.Attrs = make(map[string]any)
	}
}
