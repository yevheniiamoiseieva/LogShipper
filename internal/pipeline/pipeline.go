package pipeline

import (
	"context"
	"sync"

	"collector/internal/event"
	"collector/internal/parse"
)

type Source interface {
	Run(ctx context.Context, out chan<- event.Event) error
}

type Sink interface {
	Run(ctx context.Context, in <-chan event.Event) error
}

type Transformer interface {
	Run(ctx context.Context, in <-chan event.Event, out chan<- event.Event) error
}

type Pipeline struct {
	Sources   []Source
	Transform Transformer
	Sink      Sink
}

func (p *Pipeline) Run(ctx context.Context) error {
	sourceChan := make(chan event.Event, 100)
	parsedChan := make(chan event.Event, 100)
	sinkChan := make(chan event.Event, 100)

	var wg sync.WaitGroup
	for _, src := range p.Sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			s.Run(ctx, sourceChan)
		}(src)
	}

	go func() {
		wg.Wait()
		close(sourceChan)
	}()

	go func() {
		for evt := range sourceChan {
			parse.ParseJSON(&evt)
			parsedChan <- evt
		}
		close(parsedChan)
	}()

	go func() {
		if p.Transform != nil {
			p.Transform.Run(ctx, parsedChan, sinkChan)
		} else {
			for evt := range parsedChan {
				sinkChan <- evt
			}
		}
		close(sinkChan)
	}()

	return p.Sink.Run(ctx, sinkChan)
}