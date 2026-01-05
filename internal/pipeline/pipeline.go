package pipeline

import (
	"context"

	"collector/internal/event"
)

type Source interface {
	Run(ctx context.Context, out chan<- event.Event) error
}

type Sink interface {
	Run(ctx context.Context, in <-chan event.Event) error
}

type Pipeline struct {
	Source Source
	Sink   Sink
}

func (p *Pipeline) Run(ctx context.Context) error {
	events := make(chan event.Event, 100)

	go func() {
		defer close(events)
		_ = p.Source.Run(ctx, events)
	}()

	if err := p.Sink.Run(ctx, events); err != nil {
		return err
	}

	return nil
}
