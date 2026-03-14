package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"collector/internal/event"
)

// TemplateParser parses plain text log lines using nginx-style $variable templates.
type TemplateParser struct {
	template string
	re       *regexp.Regexp
	vars     []string
}

// NewTemplateParser compiles a template string into a TemplateParser.
func NewTemplateParser(template string) (*TemplateParser, error) {
	re, vars, err := compileTemplate(template)
	if err != nil {
		return nil, fmt.Errorf("TemplateParser: %w", err)
	}
	return &TemplateParser{template: template, re: re, vars: vars}, nil
}

// Parse returns a map of variable names to captured values, or nil on no match.
func (p *TemplateParser) Parse(line string) map[string]string {
	m := p.re.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(p.vars))
	for i, name := range p.vars {
		result[name] = m[i+1]
	}
	return result
}

// ParseNormalized parses line and maps fields into a NormalizedEvent.
// Returns nil if the line does not match the template.
func (p *TemplateParser) ParseNormalized(line, sourceName string) *event.NormalizedEvent {
	fields := p.Parse(line)
	if fields == nil {
		return nil
	}

	n := &event.NormalizedEvent{
		Format:     "template",
		SourceName: sourceName,
		Raw:        make(map[string]any, len(fields)),
	}
	for k, v := range fields {
		n.Raw[k] = v
	}

	mapTemplateFields(n, fields)

	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now().UTC()
	}
	return n
}

var timeLayouts = []string{
	"02/Jan/2006:15:04:05 -0700",
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05",
}

func mapTemplateFields(n *event.NormalizedEvent, f map[string]string) {
	for _, key := range []string{"time_local", "time_iso8601", "timestamp", "ts", "time"} {
		if v, ok := f[key]; ok {
			for _, layout := range timeLayouts {
				if t, err := time.Parse(layout, v); err == nil {
					n.Timestamp = t.UTC()
					break
				}
			}
			if !n.Timestamp.IsZero() {
				break
			}
		}
	}

	if v, ok := f["remote_addr"]; ok {
		n.Raw["remote_addr"] = v
	}

	method := f["method"]
	request := f["request"]
	if method != "" && request != "" {
		n.Operation = method + " " + request
	} else if request != "" {
		n.Operation = request
	}

	for _, key := range []string{"status", "status_code"} {
		if v, ok := f[key]; ok {
			if code, err := strconv.Atoi(v); err == nil {
				n.StatusCode = code
				break
			}
		}
	}

	for _, key := range []string{"request_time", "upstream_response_time"} {
		if v, ok := f[key]; ok {
			if sec, err := strconv.ParseFloat(v, 64); err == nil {
				n.Latency = time.Duration(sec * float64(time.Second))
				break
			}
		}
	}

	if n.Level == "" && n.StatusCode > 0 {
		switch {
		case n.StatusCode >= 500:
			n.Level = "error"
		case n.StatusCode >= 400:
			n.Level = "warn"
		default:
			n.Level = "info"
		}
	}

	for _, key := range []string{"request_id", "trace_id", "x_request_id"} {
		if v, ok := f[key]; ok && v != "" {
			n.TraceID = v
			break
		}
	}
}

var varPattern = regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)`)

func compileTemplate(tmpl string) (*regexp.Regexp, []string, error) {
	var vars []string
	var reStr strings.Builder

	last := 0
	for _, loc := range varPattern.FindAllStringSubmatchIndex(tmpl, -1) {
		reStr.WriteString(regexp.QuoteMeta(tmpl[last:loc[0]]))
		varName := tmpl[loc[2]:loc[3]]
		vars = append(vars, varName)
		reStr.WriteString("(?P<" + varName + ">" + varCapture(varName) + ")")
		last = loc[1]
	}
	reStr.WriteString(regexp.QuoteMeta(tmpl[last:]))

	re, err := regexp.Compile("^" + reStr.String() + "$")
	if err != nil {
		return nil, nil, err
	}
	return re, vars, nil
}

func varCapture(name string) string {
	switch name {
	case "time_local":
		return `[^\]]+`
	case "request":
		return `[^"]+`
	case "http_user_agent", "http_referer":
		return `[^"]*`
	case "status":
		return `\d{3}`
	case "body_bytes_sent", "bytes":
		return `\d+`
	case "request_time", "upstream_response_time":
		return `[\d.]+|-`
	default:
		return `\S+`
	}
}