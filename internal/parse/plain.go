package parse

import "collector/internal/event"

func MarkPlain(evt *event.Event) {
	ensureAttrs(evt)
	evt.Attrs["format"] = "plain"
}
