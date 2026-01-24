package transform

import (
	"context"
	"strings"
	"unicode"

	"collector/internal/event"
)

type RemapTransform struct {
	AddFields map[string]string
	Case      string
}

func (t *RemapTransform) Run(ctx context.Context, in <-chan event.Event, out chan<- event.Event) error {
	for evt := range in {

		if evt.Attrs == nil {
			evt.Attrs = make(map[string]any)
		}

		for k, v := range t.AddFields {
			evt.Attrs[k] = v
		}

		switch t.Case {
		case "upper":
			evt.Message = strings.ToUpper(evt.Message)
		case "lower":
			evt.Message = strings.ToLower(evt.Message)
		case "snake":
			evt.Message = toSnakeCase(evt.Message)
		case "camel":
			evt.Message = toCamelCase(evt.Message)
		}

		out <- evt
	}
	return nil
}

func toSnakeCase(s string) string {
	var result strings.Builder
	s = strings.TrimSpace(s)

	for i, r := range s {
		if i > 0 {
			if unicode.IsUpper(r) || unicode.IsSpace(r) || r == '-' {
				currStr := result.String()
				if len(currStr) > 0 && currStr[len(currStr)-1] != '_' {
					result.WriteRune('_')
				}
			}
		}

		if !unicode.IsSpace(r) && r != '-' {
			result.WriteRune(unicode.ToLower(r))
		}
	}
	return result.String()
}

func toCamelCase(s string) string {
	s = strings.ToLower(s)
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '_'
	})

	if len(words) == 0 {
		return s
	}

	result := words[0]
	for i := 1; i < len(words); i++ {
		w := []rune(words[i])
		w[0] = unicode.ToUpper(w[0])
		result += string(w)
	}
	return result
}