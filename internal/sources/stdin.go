package sources

import (
	"bufio"
	"context"
	"os"
	"time"

	"collector/internal/event"
)

type StdinSource struct {
	Service string
}

func (s *StdinSource) Run(ctx context.Context, out chan<- event.Event) error {
	reader := bufio.NewScanner(os.Stdin)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if !reader.Scan() {
			return nil
		}

		line := reader.Text()

		evt := event.Event{
			Timestamp: time.Now().UTC(),
			Source:    "stdin",
			Service:   s.Service,
			Message:   line,
		}

		select {
		case out <- evt:
		case <-ctx.Done():
			return nil
		}
	}
}
