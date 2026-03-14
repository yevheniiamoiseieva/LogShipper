package event_test

import (
	"testing"
	"time"

	"collector/internal/event"
)

// ----- helpers -----

func validEvent() *event.NormalizedEvent {
	return &event.NormalizedEvent{
		Timestamp:  time.Now(),
		SrcService: "auth-service",
	}
}

// ----- Validate -----

func TestValidate_OK(t *testing.T) {
	if err := validEvent().Validate(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidate_ZeroTimestamp(t *testing.T) {
	e := validEvent()
	e.Timestamp = time.Time{}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for zero Timestamp, got nil")
	}
}

func TestValidate_EmptySrcService(t *testing.T) {
	e := validEvent()
	e.SrcService = ""
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for empty SrcService, got nil")
	}
}

// ----- IsMetric -----

func TestIsMetric_WithLatency(t *testing.T) {
	e := validEvent()
	e.Latency = 42 * time.Millisecond
	if !e.IsMetric() {
		t.Fatal("expected IsMetric=true when Latency > 0")
	}
}

func TestIsMetric_WithStatusCode(t *testing.T) {
	e := validEvent()
	e.StatusCode = 200
	if !e.IsMetric() {
		t.Fatal("expected IsMetric=true when StatusCode is set")
	}
}

func TestIsMetric_PureLog(t *testing.T) {
	e := validEvent()
	e.Level = "info"
	if e.IsMetric() {
		t.Fatal("expected IsMetric=false for a plain log event")
	}
}

// ----- HasCorrelationKey -----

func TestHasCorrelationKey_TraceID(t *testing.T) {
	e := validEvent()
	e.TraceID = "abc123"
	if !e.HasCorrelationKey() {
		t.Fatal("expected HasCorrelationKey=true with TraceID")
	}
}

func TestHasCorrelationKey_SrcDst(t *testing.T) {
	e := validEvent()
	e.DstService = "payment-service"
	if !e.HasCorrelationKey() {
		t.Fatal("expected HasCorrelationKey=true with SrcService+DstService")
	}
}

func TestHasCorrelationKey_SrcOnly(t *testing.T) {
	e := validEvent()
	// DstService is empty -> only one side, not enough for a call-graph edge.
	if e.HasCorrelationKey() {
		t.Fatal("expected HasCorrelationKey=false with SrcService only")
	}
}

func TestHasCorrelationKey_None(t *testing.T) {
	e := &event.NormalizedEvent{
		Timestamp:  time.Now(),
		SrcService: "svc",
	}
	if e.HasCorrelationKey() {
		t.Fatal("expected HasCorrelationKey=false with no trace or dst")
	}
}

// ----- CorrelationKey -----

func TestCorrelationKey_PrefersTraceID(t *testing.T) {
	e := validEvent()
	e.TraceID = "trace-xyz"
	e.DstService = "other"
	e.Operation = "GET /foo"
	if got := e.CorrelationKey(); got != "trace-xyz" {
		t.Fatalf("expected trace-xyz, got %q", got)
	}
}

func TestCorrelationKey_FallbackSrcDst(t *testing.T) {
	e := validEvent()
	e.DstService = "db-service"
	e.Operation = "SELECT"
	want := "auth-service->db-service:SELECT"
	if got := e.CorrelationKey(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestCorrelationKey_EmptyOperation(t *testing.T) {
	e := validEvent()
	e.DstService = "cache"
	// Operation is empty key must still be stable.
	want := "auth-service->cache:"
	if got := e.CorrelationKey(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// ----- RiskScore -----

func TestRiskScore_Placeholder(t *testing.T) {
	e := validEvent()
	if score := e.RiskScore(); score != 0.0 {
		t.Fatalf("expected 0.0, got %v", score)
	}
}

// ----- NewFromRaw -----

func TestNewFromRaw_MinimalValid(t *testing.T) {
	raw := map[string]any{
		"service": "order-service",
	}
	e, err := event.NewFromRaw(raw, "stdin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.SrcService != "order-service" {
		t.Fatalf("SrcService mismatch: got %q", e.SrcService)
	}
	if e.SourceName != "stdin" {
		t.Fatalf("SourceName mismatch: got %q", e.SourceName)
	}
	if e.Timestamp.IsZero() {
		t.Fatal("Timestamp should be auto-filled to time.Now()")
	}
}

func TestNewFromRaw_AllFields(t *testing.T) {
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	raw := map[string]any{
		"timestamp":   ts.Format(time.RFC3339),
		"service":     "api-gateway",
		"dst_service": "user-service",
		"trace_id":    "t1",
		"span_id":     "s1",
		"operation":   "POST /login",
		"status_code": float64(200),
		"latency":     float64(42), // ms
		"level":       "info",
		"format":      "json",
	}
	e, err := event.NewFromRaw(raw, "file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !e.Timestamp.Equal(ts) {
		t.Fatalf("Timestamp: want %v, got %v", ts, e.Timestamp)
	}
	if e.SrcService != "api-gateway" {
		t.Errorf("SrcService: %q", e.SrcService)
	}
	if e.DstService != "user-service" {
		t.Errorf("DstService: %q", e.DstService)
	}
	if e.TraceID != "t1" {
		t.Errorf("TraceID: %q", e.TraceID)
	}
	if e.SpanID != "s1" {
		t.Errorf("SpanID: %q", e.SpanID)
	}
	if e.Operation != "POST /login" {
		t.Errorf("Operation: %q", e.Operation)
	}
	if e.StatusCode != 200 {
		t.Errorf("StatusCode: %d", e.StatusCode)
	}
	if e.Latency != 42*time.Millisecond {
		t.Errorf("Latency: %v", e.Latency)
	}
	if e.Level != "info" {
		t.Errorf("Level: %q", e.Level)
	}
	if e.Format != "json" {
		t.Errorf("Format: %q", e.Format)
	}
}

func TestNewFromRaw_MissingService(t *testing.T) {
	raw := map[string]any{
		"message": "hello",
	}
	// SrcService will be empty -> Validate should fail.
	_, err := event.NewFromRaw(raw, "stdin")
	if err == nil {
		t.Fatal("expected error for missing service, got nil")
	}
}

func TestNewFromRaw_TimestampFormats(t *testing.T) {
	cases := []struct {
		name string
		val  any
	}{
		{"RFC3339Nano string", "2024-01-02T15:04:05.999999999Z"},
		{"RFC3339 string", "2024-01-02T15:04:05Z"},
		{"time.Time value", time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := map[string]any{
				"service":   "svc",
				"timestamp": tc.val,
			}
			e, err := event.NewFromRaw(raw, "test")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if e.Timestamp.IsZero() {
				t.Fatal("Timestamp should be non-zero")
			}
		})
	}
}

// ----- Normalize -----

func TestNormalize_BasicFields(t *testing.T) {
	ts := time.Now()
	raw := &event.Event{
		Timestamp: ts,
		Service:   "cart-service",
		Source:    "docker",
		Level:     "warn",
		Message:   "high latency detected",
		Attrs: map[string]any{
			"format":      "json",
			"trace_id":    "trace-abc",
			"dst_service": "inventory-service",
			"operation":   "GET /items",
		},
	}
	n := event.Normalize(raw)

	if !n.Timestamp.Equal(ts) {
		t.Errorf("Timestamp mismatch")
	}
	if n.SrcService != "cart-service" {
		t.Errorf("SrcService: %q", n.SrcService)
	}
	if n.SourceName != "docker" {
		t.Errorf("SourceName: %q", n.SourceName)
	}
	if n.Level != "warn" {
		t.Errorf("Level: %q", n.Level)
	}
	if n.Format != "json" {
		t.Errorf("Format: %q", n.Format)
	}
	if n.TraceID != "trace-abc" {
		t.Errorf("TraceID: %q", n.TraceID)
	}
	if n.DstService != "inventory-service" {
		t.Errorf("DstService: %q", n.DstService)
	}
	if n.Operation != "GET /items" {
		t.Errorf("Operation: %q", n.Operation)
	}
	if n.Raw["message"] != "high latency detected" {
		t.Errorf("Raw[message]: %v", n.Raw["message"])
	}
}

func TestNormalize_MetricEvent(t *testing.T) {
	raw := &event.Event{
		Timestamp: time.Now(),
		Service:   "billing",
		Type:      event.TypeMetric,
		Metric:    "requests_total",
		Value:     42.5,
		Attrs:     map[string]any{},
	}
	n := event.Normalize(raw)
	if n.Operation != "requests_total" {
		t.Errorf("expected Operation=metric name, got %q", n.Operation)
	}
	if n.Raw["metric_value"] != 42.5 {
		t.Errorf("metric_value not preserved in Raw")
	}
}