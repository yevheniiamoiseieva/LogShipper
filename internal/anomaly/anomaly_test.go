package anomaly

import (
	"math"
	"testing"
	"time"
)

func TestRollingStats_Empty(t *testing.T) {
	s := NewRollingStats(10)
	if s.Count() != 0 {
		t.Errorf("Count = %d, want 0", s.Count())
	}
	if s.Mean() != 0 {
		t.Errorf("Mean = %v, want 0", s.Mean())
	}
	if s.StdDev() != 0 {
		t.Errorf("StdDev = %v, want 0", s.StdDev())
	}
	if s.ZScore(5) != 0 {
		t.Errorf("ZScore with no data = %v, want 0", s.ZScore(5))
	}
}

func TestRollingStats_Mean(t *testing.T) {
	s := NewRollingStats(10)
	for _, v := range []float64{1, 2, 3, 4, 5} {
		s.Add(v)
	}
	if s.Mean() != 3.0 {
		t.Errorf("Mean = %v, want 3.0", s.Mean())
	}
}

func TestRollingStats_StdDev(t *testing.T) {
	s := NewRollingStats(10)
	for _, v := range []float64{2, 4, 4, 4, 5, 5, 7, 9} {
		s.Add(v)
	}
	got := s.StdDev()
	want := 2.0
	if math.Abs(got-want) > 0.01 {
		t.Errorf("StdDev = %v, want ~%v", got, want)
	}
}

func TestRollingStats_SingleValue(t *testing.T) {
	s := NewRollingStats(10)
	s.Add(42)
	if s.Count() != 1 {
		t.Errorf("Count = %d, want 1", s.Count())
	}
	if s.Mean() != 42 {
		t.Errorf("Mean = %v, want 42", s.Mean())
	}
	if s.StdDev() != 0 {
		t.Errorf("StdDev with 1 value = %v, want 0", s.StdDev())
	}
}

func TestRollingStats_Window(t *testing.T) {
	s := NewRollingStats(3)
	s.Add(1)
	s.Add(2)
	s.Add(3)
	s.Add(4) // вытесняет 1

	if s.Count() != 3 {
		t.Errorf("Count after overflow = %d, want 3", s.Count())
	}
	if s.Mean() != 3.0 {
		t.Errorf("Mean after overflow = %v, want 3.0", s.Mean())
	}
}

func TestRollingStats_ZScore(t *testing.T) {
	s := NewRollingStats(100)
	for i := 0; i < 100; i++ {
		s.Add(10.0)
	}
	s.Add(10.0)
	if z := s.ZScore(10.0); z != 0 {
		t.Errorf("ZScore of mean = %v, want 0", z)
	}
}

func TestRollingStats_ZScore_Outlier(t *testing.T) {
	s := NewRollingStats(100)
	for i := 0; i < 100; i++ {
		s.Add(10.0 + float64(i%3))
	}

	z := s.ZScore(100.0)
	if z <= 3.0 {
		t.Errorf("ZScore of outlier = %v, want > 3.0", z)
	}
}

func TestDetector_NoEventBelowThreshold(t *testing.T) {
	d := NewZScoreDetector(20, 3.0, 64)
	d.WithMinSamples(5)

	for i := 0; i < 20; i++ {
		d.Feed("A|B|op", "latency", 10.0)
	}

	evs := drainAnomalyEvents(d.Events(), 50*time.Millisecond)
	if len(evs) != 0 {
		t.Errorf("expected no anomaly events for stable input, got %d", len(evs))
	}
}

func TestDetector_EventAboveThreshold(t *testing.T) {
	d := NewZScoreDetector(50, 3.0, 64)
	d.WithMinSamples(10)

	for i := 0; i < 50; i++ {
		d.Feed("A|B|op", "latency", 10.0)
	}
	d.Feed("A|B|op", "latency", 10000.0)

	evs := drainAnomalyEvents(d.Events(), 100*time.Millisecond)
	if len(evs) == 0 {
		t.Fatal("expected anomaly event for outlier value")
	}
	ev := evs[0]
	if ev.EdgeKey != "A|B|op" {
		t.Errorf("EdgeKey = %q, want A|B|op", ev.EdgeKey)
	}
	if ev.Metric != "latency" {
		t.Errorf("Metric = %q, want latency", ev.Metric)
	}
	if math.Abs(ev.ZScore) <= 3.0 {
		t.Errorf("ZScore = %v, want > 3.0", ev.ZScore)
	}
}

func TestDetector_NoDuplicateWhileInAnomaly(t *testing.T) {
	d := NewZScoreDetector(50, 3.0, 64)
	d.WithMinSamples(10).WithCooldown(0)

	for i := 0; i < 50; i++ {
		d.Feed("A|B|op", "latency", 10.0)
	}
	for i := 0; i < 5; i++ {
		d.Feed("A|B|op", "latency", 10000.0)
	}

	evs := drainAnomalyEvents(d.Events(), 100*time.Millisecond)
	if len(evs) != 1 {
		t.Errorf("expected 1 anomaly event while in anomaly state, got %d", len(evs))
	}
}

func TestDetector_RecoveryAllowsNewEvent(t *testing.T) {
	d := NewZScoreDetector(50, 3.0, 64)
	d.WithMinSamples(10).WithCooldown(0)

	for i := 0; i < 50; i++ {
		d.Feed("A|B|op", "latency", 10.0)
	}

	d.Feed("A|B|op", "latency", 10000.0)

	for i := 0; i < 10; i++ {
		d.Feed("A|B|op", "latency", 10.0)
	}

	d.Feed("A|B|op", "latency", 10000.0)

	evs := drainAnomalyEvents(d.Events(), 100*time.Millisecond)
	if len(evs) < 2 {
		t.Errorf("expected 2 anomaly events after recovery, got %d", len(evs))
	}
}

func TestDetector_MinSamplesNotReached(t *testing.T) {
	d := NewZScoreDetector(100, 3.0, 64)
	d.WithMinSamples(50)

	for i := 0; i < 10; i++ {
		d.Feed("A|B|op", "latency", float64(i*100))
	}

	evs := drainAnomalyEvents(d.Events(), 50*time.Millisecond)
	if len(evs) != 0 {
		t.Errorf("expected no events before min_samples reached, got %d", len(evs))
	}
}

func TestDetector_Cooldown(t *testing.T) {
	d := NewZScoreDetector(50, 3.0, 64)
	d.WithMinSamples(10).WithCooldown(1 * time.Hour)

	for i := 0; i < 50; i++ {
		d.Feed("A|B|op", "latency", 10.0)
	}

	d.Feed("A|B|op", "latency", 10000.0)

	for i := 0; i < 5; i++ {
		d.Feed("A|B|op", "latency", 10.0)
	}
	d.Feed("A|B|op", "latency", 10000.0)

	evs := drainAnomalyEvents(d.Events(), 100*time.Millisecond)
	if len(evs) != 1 {
		t.Errorf("cooldown: expected 1 event, got %d", len(evs))
	}
}

func TestDetector_MultipleEdges(t *testing.T) {
	d := NewZScoreDetector(50, 3.0, 64)
	d.WithMinSamples(10)

	for i := 0; i < 50; i++ {
		d.Feed("A|B|op", "latency", 10.0)
		d.Feed("C|D|op", "latency", 10.0)
	}
	d.Feed("A|B|op", "latency", 10000.0)
	d.Feed("C|D|op", "latency", 10000.0)

	evs := drainAnomalyEvents(d.Events(), 100*time.Millisecond)
	if len(evs) != 2 {
		t.Errorf("expected 2 events (one per edge), got %d", len(evs))
	}
}

func TestDetector_Stats(t *testing.T) {
	d := NewZScoreDetector(50, 3.0, 64)
	for i := 0; i < 10; i++ {
		d.Feed("A|B|op", "latency", 10.0)
	}
	mean, stddev := d.Stats("A|B|op", "latency")
	if mean != 10.0 {
		t.Errorf("Stats mean = %v, want 10.0", mean)
	}
	if stddev != 0 {
		t.Errorf("Stats stddev = %v, want 0 (constant input)", stddev)
	}
}

func TestDetector_Stats_UnknownKey(t *testing.T) {
	d := NewZScoreDetector(50, 3.0, 64)
	mean, stddev := d.Stats("unknown", "latency")
	if mean != 0 || stddev != 0 {
		t.Errorf("unknown key: want 0,0 got %v,%v", mean, stddev)
	}
}

func drainAnomalyEvents(ch <-chan AnomalyEvent, timeout time.Duration) []AnomalyEvent {
	var out []AnomalyEvent
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
}
