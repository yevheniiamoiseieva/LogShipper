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
	if len(p.Sources) == 0 {
		return fmt.Errorf("pipeline: no sources provided")
	}
	if p.Sink == nil {
		return fmt.Errorf("pipeline: no sink provided")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sourceChan := make(chan event.Event, 100)
	parsedChan := make(chan event.Event, 100)
	sinkChan := make(chan event.Event, 100)

	errCh := make(chan error, 8)
	var wg sync.WaitGroup

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

				parse.ParseJSON(&evt)

				select {
				case parsedChan <- evt:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	go func() {
		defer close(sinkChan)

		if p.Transform == nil {
			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-parsedChan:
					if !ok {
						return
					}
					select {
					case sinkChan <- evt:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		if err := p.Transform.Run(ctx, parsedChan, sinkChan); err != nil && err != context.Canceled {
			select {
			case errCh <- err:
			default:
			}
			cancel()
			return
		}
	}()

	sinkErr := p.Sink.Run(ctx, sinkChan)
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
