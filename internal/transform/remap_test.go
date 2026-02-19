package transform

import (
	"context"
	"testing"
	"collector/internal/event"
)

func TestRemapTransform_Run(t *testing.T) {
	tests := []struct {
		name     string
		caseType string
		input    string
		expected string
	}{
		{"Lower Case", "lower", "HELLO WORLD", "hello world"},
		{"Upper Case", "upper", "hello world", "HELLO WORLD"},
		{"Snake Case", "snake", "Hello World Test", "hello_world_test"},
		{"Camel Case", "camel", "hello_world_test", "helloWorldTest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &RemapTransform{
				Case:      tt.caseType,
				AddFields: map[string]string{"env": "test"},
			}

			in := make(chan event.Event, 1)
			out := make(chan event.Event, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			in <- event.Event{Message: tt.input}
			close(in)

			go tr.Run(ctx, in, out)

			result := <-out
			if result.Message != tt.expected {
				t.Errorf("Case [%s]: expected '%s', got '%s'", tt.caseType, tt.expected, result.Message)
			}

			if result.Attrs["env"] != "test" {
				t.Errorf("Expected attr 'env' to be 'test', got '%v'", result.Attrs["env"])
			}
		})
	}
}