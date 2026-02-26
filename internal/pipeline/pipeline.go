package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"

	"collector/internal/event"
	"collector/internal/parse"
)

type Source interface {
	Run(ctx context.Context, out chan<- event.Event) error
}

// normalizedSink is the preferred sink interface, consuming *event.NormalizedEvent.
type NormalizedSink interface {
	Run(ctx context.Context, in <-chan *event.NormalizedEvent) error
}

// sink is the legacy interface, kept for backward compatibility.
type Sink interface {
	Run(ctx context.Context, in <-chan event.Event) error
}

type Transformer interface {
	Run(ctx context.Context, in <-chan event.Event, out chan<- event.Event) error
}

type Pipeline struct {
	Sources        []Source
	Transform      Transformer
	Sink           Sink           // legacy
	NormalizedSink NormalizedSink // preferred
}

func (p *Pipeline) Run(ctx context.Context) error {
	if len(p.Sources) == 0 {
		return fmt.Errorf("pipeline: no sources provided")
	}
	if p.Sink == nil && p.NormalizedSink == nil {
		return fmt.Errorf("pipeline: no sink provided")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sourceChan := make(chan event.Event, 100)
	parsedChan := make(chan event.Event, 100)
	normalChan := make(chan *event.NormalizedEvent, 100)
	legacyChan := make(chan event.Event, 100)

	errCh := make(chan error, 8)
	var wg sync.WaitGroup

	// sources
	for _, src := range p.Sources {
		s := src
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Run(ctx, sourceChan); err != nil && err != context.Canceled {
				select {
				case errCh <- err:
				default:
				}
				cancel()
			}
		}()
	}
	go func() {
		wg.Wait()
		close(sourceChan)
	}()

	// parse
	go func() {
		defer close(parsedChan)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-sourceChan:
				if !ok {
					return
				}
				parse.ParseEvent(&evt)
				select {
				case parsedChan <- evt:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// transform (optional)
	transformedChan := parsedChan
	if p.Transform != nil {
		tc := make(chan event.Event, 100)
		go func() {
			defer close(tc)
			if err := p.Transform.Run(ctx, parsedChan, tc); err != nil && err != context.Canceled {
				select {
				case errCh <- err:
				default:
				}
				cancel()
			}
		}()
		transformedChan = tc
	}

	// normalize
	go func() {
		defer close(normalChan)
		if p.NormalizedSink == nil {
			defer close(legacyChan)
			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-transformedChan:
					if !ok {
						return
					}
					select {
					case legacyChan <- evt:
					case <-ctx.Done():
						return
					}
				}
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-transformedChan:
				if !ok {
					return
				}
				select {
				case normalChan <- event.Normalize(&evt):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// sink
	var sinkErr error
	if p.NormalizedSink != nil {
		sinkErr = p.NormalizedSink.Run(ctx, normalChan)
	} else {
		sinkErr = p.Sink.Run(ctx, legacyChan)
	}

	if sinkErr != nil && sinkErr != context.Canceled {
		select {
		case errCh <- sinkErr:
		default:
		}
		cancel()
	}

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			log.Printf("pipeline stopped with error: %v", err)
			return err
		}
	default:
	}
	return sinkErr
}