package sources

import (
	"context"
	"testing"
	"time"

	"collector/internal/event"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestDockerSource_WithTestcontainers(t *testing.T) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: "alpine",
		Cmd:   []string{"sh", "-c", "echo 'test-docker-log'"},
		WaitingFor: wait.ForLog("test-docker-log"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	defer container.Terminate(ctx)

	containerID := container.GetContainerID()

	src := &DockerSource{
		ContainerID: containerID,
		Service:     "test-docker-service",
	}

	out := make(chan event.Event, 1)
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	go src.Run(runCtx, out)

	select {
	case evt := <-out:
		if evt.Message != "test-docker-log" {
			t.Errorf("expected 'test-docker-log', got '%s'", evt.Message)
		}
		if evt.Source != "docker" {
			t.Errorf("expected source 'docker', got '%s'", evt.Source)
		}
	case <-runCtx.Done():
		t.Fatal("timed out waiting for docker logs")
	}
}