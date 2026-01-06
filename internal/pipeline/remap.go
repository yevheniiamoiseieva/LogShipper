package pipeline

import (
	"context"
	"collector/internal/event"
)

type RemapTransform struct {
	AddFields map[string]string
}

func (t *RemapTransform) Run(ctx context.Context, in <-chan event.Event, out chan<- event.Event) error {
	for evt := range in {
		if evt.Attrs == nil {
			evt.Attrs = make(map[string]any)
		}
		for k, v := range t.AddFields {
			evt.Attrs[k] = v
		}
		out <- evt
	}
	return nil
}