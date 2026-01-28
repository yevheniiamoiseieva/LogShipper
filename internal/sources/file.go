package sources

import (
	"context"
	"log"
	"time"

	"collector/internal/event"
	"github.com/hpcloud/tail"
)

type FileSource struct {
	Service string
	Path    string
}

func (fs *FileSource) Run(ctx context.Context, out chan<- event.Event) error {
	t, err := tail.TailFile(fs.Path, tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: false,
	})
	if err != nil {
		return err
	}
	defer t.Stop()

	log.Printf("file source started for path: %s", fs.Path)

	for {
		select {
		case <-ctx.Done():
			log.Printf("file source stopping for path: %s", fs.Path)
			return nil

		case line, ok := <-t.Lines:
			if !ok {
				return nil
			}
			if line.Err != nil {
				log.Printf("tail line error: %v", line.Err)
				continue
			}

			e := event.Event{
				Timestamp: time.Now().UTC(),
				Source:    "file",
				Service:   fs.Service,
				Type:      event.TypeLog,

				Message: line.Text,
				Level:   "info",
				Attrs:   map[string]any{"path": fs.Path},
			}

			select {
			case out <- e:
			case <-ctx.Done():
				return nil
			}
		}
	}
}
