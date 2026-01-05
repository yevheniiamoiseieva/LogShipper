package sinks

import (
	"context"
	"encoding/json"
	"os"

	"collector/internal/event"
)

type StdoutSink struct {
	Pretty bool
}

func (s *StdoutSink) Run(ctx context.Context, in <-chan event.Event) error {
	encoder := json.NewEncoder(os.Stdout)
	if s.Pretty {
		encoder.SetIndent("", "  ")
	}

	for evt := range in {
		if err := encoder.Encode(evt); err != nil {
			return err
		}
	}

	return nil
}
