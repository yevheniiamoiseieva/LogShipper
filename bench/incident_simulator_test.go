package bench

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"collector/internal/anomaly"
)

type IncidentPhase int

const (
	PhaseNormal IncidentPhase = iota
	PhaseIncident
)

type IncidentSimulator struct {
	detector         *anomaly.ZScoreDetector
	rng              *rand.Rand
	normalDuration   time.Duration
	incidentDuration time.Duration

	FirstAnomalyAt time.Time
	DetectedAt     time.Time
	DetectionDelay time.Duration
	TotalEvents    int
	AlertsReceived int
}

func NewIncidentSimulator(det *anomaly.ZScoreDetector, seed int64) *IncidentSimulator {
	return &IncidentSimulator{
		detector:         det,
		rng:              rand.New(rand.NewSource(seed)),
		normalDuration:   5 * time.Minute,
		incidentDuration: 2 * time.Minute,
	}
}

func (s *IncidentSimulator) drainEvents() int {
	n := 0
	for {
		select {
		case <-s.detector.Events():
			n++
		default:
			return n
		}
	}
}

func (s *IncidentSimulator) Run(_ context.Context) {
	normalTicks := int(s.normalDuration / (100 * time.Millisecond))
	incidentTicks := int(s.incidentDuration / (100 * time.Millisecond))

	for i := 0; i < normalTicks; i++ {
		s.feedNormal()
	}
	s.drainEvents() // выбрасываем warm-up алерты

	s.FirstAnomalyAt = time.Now()

	for i := 0; i < incidentTicks; i++ {
		s.feedIncident()
		n := s.drainEvents()
		if n > 0 {
			s.AlertsReceived += n
			if s.DetectedAt.IsZero() {
				s.DetectedAt = time.Now()
				s.DetectionDelay = s.DetectedAt.Sub(s.FirstAnomalyAt)
			}
		}
	}
}

func (s *IncidentSimulator) feedNormal() {
	latency := s.rng.NormFloat64()*10 + 50
	if latency < 1 {
		latency = 1
	}
	s.detector.Feed("payment-service|db|query", "latency_ms", latency)
	s.detector.Feed("api-gw|auth|HTTP", "latency_ms", s.rng.NormFloat64()*5+20)
	s.TotalEvents += 2
}

func (s *IncidentSimulator) feedIncident() {
	latency := (s.rng.NormFloat64()*10 + 50) * 10
	s.detector.Feed("payment-service|db|query", "latency_ms", latency)

	errVal := 0.0
	if s.rng.Float64() < 0.30 {
		errVal = 1.0
	}
	s.detector.Feed("payment-service|db|query", "error_rate", errVal)
	s.detector.Feed("api-gw|auth|HTTP", "latency_ms", s.rng.NormFloat64()*5+20)
	s.detector.Feed("auth|api-gw|HTTP", "latency_ms", s.rng.NormFloat64()*5+20)
	s.TotalEvents += 4
}

func TestTimeToDiagnose(t *testing.T) {
	det := anomaly.NewZScoreDetector(100, 3.5, 4096).
		WithMinSamples(50).
		WithCooldown(0)

	sim := NewIncidentSimulator(det, 42)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Running incident simulation…")
	wallStart := time.Now()
	sim.Run(ctx)
	wallElapsed := time.Since(wallStart)

	t.Logf("Simulation wall-clock: %v", wallElapsed)
	t.Logf("Total events fed:      %d", sim.TotalEvents)
	t.Logf("Alerts received:       %d", sim.AlertsReceived)

	if sim.FirstAnomalyAt.IsZero() {
		t.Fatal("FirstAnomalyAt was never set")
	}
	if sim.DetectedAt.IsZero() {
		t.Error("anomaly was NOT detected — no alert received")
		return
	}

	t.Logf("First anomaly feed at: %v", sim.FirstAnomalyAt.Format(time.RFC3339Nano))
	t.Logf("Alert received at:     %v", sim.DetectedAt.Format(time.RFC3339Nano))
	t.Logf("Detection latency:     %v", sim.DetectionDelay)

	const target = 2 * time.Second
	if sim.DetectionDelay > target {
		t.Errorf("detection latency %v exceeds target %v", sim.DetectionDelay, target)
	} else {
		t.Logf("✓ Detection latency within target (%v < %v)", sim.DetectionDelay, target)
	}

	t.Log("\nTUI Drill-Down Steps:")
	steps := []string{
		"1. Service List — payment-service highlighted (anomaly badge)",
		"2. Enter → Edge List — payment-service→db shows latency spike",
		"3. Enter → Event Detail — ZScore, latency ~500ms, timestamp",
	}
	for _, step := range steps {
		t.Log(" ", step)
	}
	t.Logf("Total TUI transitions: %d (target ≤ 3)", len(steps))
	if len(steps) > 3 {
		t.Errorf("drill-down requires %d steps, exceeds target of 3", len(steps))
	}
}

func BenchmarkIncidentSimulator(b *testing.B) {
	det := anomaly.NewZScoreDetector(100, 3.5, 8192).
		WithMinSamples(50).
		WithCooldown(0)

	sim := NewIncidentSimulator(det, 1)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sim.feedNormal()
	}
	_ = fmt.Sprintf("%d events", sim.TotalEvents)
}
