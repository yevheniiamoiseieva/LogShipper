package parse

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// ── JSON parser ───────────────────────────────────────────────────────────────

func TestParseJSONNormalized_TraceID(t *testing.T) {
	cases := []struct {
		name    string
		input   map[string]any
		wantID  string
	}{
		{"trace_id key",  map[string]any{"trace_id": "abc123", "service": "svc"}, "abc123"},
		{"traceId key",   map[string]any{"traceId": "def456", "service": "svc"}, "def456"},
		{"X-Trace-Id key", map[string]any{"X-Trace-Id": "ghi789", "service": "svc"}, "ghi789"},
		{"no trace",      map[string]any{"service": "svc"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := ParseJSONNormalized(tc.input, "test")
			if n.TraceID != tc.wantID {
				t.Errorf("TraceID: want %q, got %q", tc.wantID, n.TraceID)
			}
		})
	}
}

func TestParseJSONNormalized_Latency(t *testing.T) {
	cases := []struct {
		name    string
		input   map[string]any
		want    time.Duration
	}{
		{"ms float",     map[string]any{"duration_ms": float64(145)}, 145 * time.Millisecond},
		{"ms string",    map[string]any{"latency": "87ms"}, 87 * time.Millisecond},
		{"s string",     map[string]any{"response_time": "0.234s"}, 234 * time.Millisecond},
		{"µs string",    map[string]any{"elapsed": "500µs"}, 500 * time.Microsecond},
		{"bare float ms", map[string]any{"duration": float64(200)}, 200 * time.Millisecond},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := ParseJSONNormalized(tc.input, "test")
			if n.Latency != tc.want {
				t.Errorf("Latency: want %v, got %v", tc.want, n.Latency)
			}
		})
	}
}

func TestParseJSONNormalized_StatusCode(t *testing.T) {
	cases := []struct {
		input map[string]any
		want  int
	}{
		{map[string]any{"status_code": float64(200)}, 200},
		{map[string]any{"status": float64(404)}, 404},
		{map[string]any{"http.status": float64(500)}, 500},
		{map[string]any{"status": "201"}, 201},
	}

	for _, tc := range cases {
		n := ParseJSONNormalized(tc.input, "test")
		if n.StatusCode != tc.want {
			t.Errorf("StatusCode: want %d, got %d (input %v)", tc.want, n.StatusCode, tc.input)
		}
	}
}

func TestParseJSONNormalized_Operation(t *testing.T) {
	cases := []struct {
		name  string
		input map[string]any
		want  string
	}{
		{"explicit operation", map[string]any{"operation": "UserService.GetUser"}, "UserService.GetUser"},
		{"method+url",         map[string]any{"method": "GET", "url": "/api/users"}, "GET /api/users"},
		{"rpc.method",         map[string]any{"rpc.method": "SayHello"}, "SayHello"},
		{"url only",           map[string]any{"path": "/health"}, "/health"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := ParseJSONNormalized(tc.input, "test")
			if n.Operation != tc.want {
				t.Errorf("Operation: want %q, got %q", tc.want, n.Operation)
			}
		})
	}
}

func TestParseJSONNormalized_DstService(t *testing.T) {
	cases := []struct {
		input map[string]any
		want  string
	}{
		{map[string]any{"upstream": "user-db"}, "user-db"},
		{map[string]any{"remote_service": "stripe"}, "stripe"},
		{map[string]any{"peer.service": "redis"}, "redis"},
	}

	for _, tc := range cases {
		n := ParseJSONNormalized(tc.input, "test")
		if n.DstService != tc.want {
			t.Errorf("DstService: want %q, got %q (input %v)", tc.want, n.DstService, tc.input)
		}
	}
}

// ── ECS parser ────────────────────────────────────────────────────────────────

func TestParseECSNormalized_FullEvent(t *testing.T) {
	raw := map[string]any{
		"@timestamp": "2024-02-10T13:55:36.123Z",
		"message":    "POST /login responded 401",
		"log":        map[string]any{"level": "warn"},
		"service":    map[string]any{"name": "api-gateway"},
		"trace":      map[string]any{"id": "4bf92f3577b34da6a3ce929d0e0e4736"},
		"span":       map[string]any{"id": "00f067aa0ba902b7"},
		"http": map[string]any{
			"request":  map[string]any{"method": "POST"},
			"response": map[string]any{"status_code": float64(401)},
		},
		"url":         map[string]any{"path": "/api/v2/auth/login"},
		"event":       map[string]any{"duration": float64(234_000_000)},
		"destination": map[string]any{"address": "auth-service"},
	}

	n := ParseECSNormalized(raw, "test-source")

	if n.SrcService != "api-gateway" {
		t.Errorf("SrcService: want api-gateway, got %s", n.SrcService)
	}
	if n.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("TraceID: want 4bf92f..., got %s", n.TraceID)
	}
	if n.SpanID != "00f067aa0ba902b7" {
		t.Errorf("SpanID: want 00f067..., got %s", n.SpanID)
	}
	if n.StatusCode != 401 {
		t.Errorf("StatusCode: want 401, got %d", n.StatusCode)
	}
	if n.Latency != 234*time.Millisecond {
		t.Errorf("Latency: want 234ms, got %v", n.Latency)
	}
	if n.Operation != "POST /api/v2/auth/login" {
		t.Errorf("Operation: want 'POST /api/v2/auth/login', got %s", n.Operation)
	}
	if n.DstService != "auth-service" {
		t.Errorf("DstService: want auth-service, got %s", n.DstService)
	}
	if n.Level != "warn" {
		t.Errorf("Level: want warn, got %s", n.Level)
	}
	if n.Format != "ecs_json" {
		t.Errorf("Format: want ecs_json, got %s", n.Format)
	}
}

// ── template parser ───────────────────────────────────────────────────────────

func TestTemplateParser_NginxCombined(t *testing.T) {
	tmpl := `$remote_addr - $remote_user [$time_local] "$method $request $protocol" $status $body_bytes_sent "$http_referer" "$http_user_agent" $request_time $request_id`

	p, err := NewTemplateParser(tmpl)
	if err != nil {
		t.Fatalf("NewTemplateParser: %v", err)
	}

	line := `192.168.1.42 - john [10/Feb/2024:13:55:36 +0300] "GET /api/users HTTP/1.1" 200 1543 "https://example.com" "Mozilla/5.0" 0.087 a1b2c3d4`

	n := p.ParseNormalized(line, "nginx-file")
	if n == nil {
		t.Fatal("ParseNormalized returned nil")
	}
	if n.StatusCode != 200 {
		t.Errorf("StatusCode: want 200, got %d", n.StatusCode)
	}
	if n.Operation != "GET /api/users" {
		t.Errorf("Operation: want 'GET /api/users', got %q", n.Operation)
	}
	if n.Latency != 87*time.Millisecond {
		t.Errorf("Latency: want 87ms, got %v", n.Latency)
	}
	if n.TraceID != "a1b2c3d4" {
		t.Errorf("TraceID: want a1b2c3d4, got %q", n.TraceID)
	}
}

func TestTemplateParser_NoMatch(t *testing.T) {
	p, _ := NewTemplateParser(`$remote_addr [$time_local] $status`)
	n := p.ParseNormalized("this does not match at all", "src")
	if n != nil {
		t.Errorf("expected nil for non-matching line, got %+v", n)
	}
}

// ── testdata integration ──────────────────────────────────────────────────────

func TestParseNormalized_JSONTestdata(t *testing.T) {
	f, err := os.Open("testdata/json_logs.ndjson")
	if err != nil {
		t.Skipf("testdata not found: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Text()
		n := ParseNormalized(raw, "test")
		if n == nil {
			t.Errorf("line %d: ParseNormalized returned nil", line)
			continue
		}
		if n.Timestamp.IsZero() {
			t.Errorf("line %d: Timestamp is zero", line)
		}
	}
}

func TestParseNormalized_ECSTestdata(t *testing.T) {
	f, err := os.Open("testdata/ecs_logs.ndjson")
	if err != nil {
		t.Skipf("testdata not found: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()

		var raw map[string]any
		if err := json.Unmarshal([]byte(text), &raw); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", line, err)
		}

		n := ParseECSNormalized(raw, "test")
		if n.SrcService == "" {
			t.Errorf("line %d: SrcService is empty", line)
		}
		if n.TraceID == "" {
			t.Logf("line %d: no TraceID (optional)", line)
		}
	}
}

// ── dispatcher ────────────────────────────────────────────────────────────────

func TestParseNormalized_Routing(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantFormat string
	}{
		{
			"ECS",
			`{"@timestamp":"2024-01-01T00:00:00Z","log":{"level":"info"},"service":{"name":"svc"}}`,
			"ecs_json",
		},
		{
			"JSON",
			`{"timestamp":"2024-01-01T00:00:00Z","level":"info","service":"svc","message":"hello"}`,
			"json",
		},
		{
			"plain",
			`just a plain text log line`,
			"plain",
		},
		{
			"empty",
			``,
			"empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := ParseNormalized(tc.input, "test")
			if n.Format != tc.wantFormat {
				t.Errorf("Format: want %q, got %q", tc.wantFormat, n.Format)
			}
		})
	}
}