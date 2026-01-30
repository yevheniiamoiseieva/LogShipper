package sources

import (
	"context"
	"os"
	"testing"
	"collector/internal/event"
)

func TestStdinSource(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	src := &StdinSource{Service: "stdin-service"}
	out := make(chan event.Event, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go src.Run(ctx, out)

	w.WriteString("hello from stdin\n")

	evt := <-out
	if evt.Message != "hello from stdin" {
		t.Errorf("got %s", evt.Message)
	}
}