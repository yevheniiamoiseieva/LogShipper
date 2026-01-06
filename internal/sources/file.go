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

	go func() {
		log.Printf("file source started for path: %s", fs.Path)

		for {
			select {
			case <-ctx.Done():
				log.Printf("file source stopping for path: %s", fs.Path)
				t.Stop()
				return
			case line, ok := <-t.Lines:
				if !ok {
					return
				}
				if line.Err != nil {
					log.Printf("tail line error: %v", line.Err)
					continue
				}

				e := event.Event{
					Timestamp: time.Now(),
					Source:    "file",
					Service:   fs.Service,
					Message:   line.Text,
					Level:     "info",
					Attrs:     make(map[string]any),
				}

				select {
				case <-ctx.Done():
					return
				case out <- e:
				}
			}
		}
	}()

	return nil
}