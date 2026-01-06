package sources

import (
	"bufio"
	"context"
	"io"
	"log"
	"time"

	"collector/internal/event"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerSource struct {
	Service     string
	ContainerID string
}

func (ds *DockerSource) Run(ctx context.Context, out chan<- event.Event) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	go func() {
		log.Printf("docker source started for container: %s", ds.ContainerID)

		options := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: false,
		}

		reader, err := cli.ContainerLogs(ctx, ds.ContainerID, options)
		if err != nil {
			log.Printf("docker logs error: %v", err)
			return
		}
		defer reader.Close()

		stdoutReader, stdoutWriter := io.Pipe()

		go func() {
			_, err := stdcopy.StdCopy(stdoutWriter, stdoutWriter, reader)
			stdoutWriter.CloseWithError(err)
		}()

		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				msg := scanner.Text()
				if msg == "" {
					continue
				}

				out <- event.Event{
					Timestamp: time.Now(),
					Source:    "docker",
					Service:   ds.Service,
					Message:   msg,
					Level:     "info",
					Attrs:     map[string]any{"container_id": ds.ContainerID},
				}
			}
		}

		if err := scanner.Err(); err != nil && err != context.Canceled {
			log.Printf("docker scanner error: %v", err)
		}
	}()

	return nil
}