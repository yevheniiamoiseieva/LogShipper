package sources

import (
	"context"
	"os"
	"testing"
	"time"

	"collector/internal/event"
)

func TestFileSource(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_logs_*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	src := &FileSource{
		Path:    tmpFile.Name(),
		Service: "test-service",
	}

	out := make(chan event.Event, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	testMessage := "test log line\n"
	if _, err := tmpFile.WriteString(testMessage); err != nil {
		t.Fatal(err)
	}

	go src.Run(ctx, out)

	select {
	case evt := <-out:
		if evt.Message != "test log line" {
			t.Errorf("expected 'test log line', got '%s'", evt.Message)
		}
	case <-ctx.Done():
		t.Error("timed out waiting for file event")
	}
}