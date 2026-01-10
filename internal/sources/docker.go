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

	log.Printf("docker source started for container: %s", ds.ContainerID)

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	}

	reader, err := cli.ContainerLogs(ctx, ds.ContainerID, options)
	if err != nil {
		return err
	}
	defer reader.Close()

	pipeR, pipeW := io.Pipe()
	defer pipeR.Close()

	go func() {
		_, err := stdcopy.StdCopy(pipeW, pipeW, reader)
		_ = pipeW.CloseWithError(err)
	}()

	scanner := bufio.NewScanner(pipeR)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		msg := scanner.Text()
		if msg == "" {
			continue
		}

		e := event.Event{
			Timestamp: time.Now().UTC(),
			Source:    "docker",
			Service:   ds.Service,
			Message:   msg,
			Level:     "info",
			Attrs:     map[string]any{"container_id": ds.ContainerID},
		}

		select {
		case out <- e:
		case <-ctx.Done():
			log.Printf("docker source stopping for container: %s", ds.ContainerID)
			return nil
		}
	}

	if err := scanner.Err(); err != nil && err != context.Canceled {
		return err
	}

	return nil
}
