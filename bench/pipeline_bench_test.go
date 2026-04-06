package bench

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"collector/internal/event"
	"collector/internal/parse"
)

func drainNormalized(ch <-chan *event.NormalizedEvent, n int) {
	for i := 0; i < n; i++ {
		<-ch
	}
}

func parseLines(lines [][]byte) ([]*event.NormalizedEvent, time.Duration) {
	results := make([]*event.NormalizedEvent, len(lines))
	start := time.Now()
	for i, line := range lines {
		results[i] = parse.ParseNormalized(string(line), "bench")
	}
	return results, time.Since(start)
}

func BenchmarkPipeline_JSON(b *testing.B) {
	gen := NewLogGenerator(42, DefaultConfig())

	lines := make([][]byte, b.N)
	for i := range lines {
		lines[i] = gen.NextJSON()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		parse.ParseNormalized(string(lines[i]), "bench")
	}
}

func BenchmarkPipeline_ECS(b *testing.B) {
	gen := NewLogGenerator(42, DefaultConfig())

	lines := make([][]byte, b.N)
	for i := range lines {
		lines[i] = gen.NextECS()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		parse.ParseNormalized(string(lines[i]), "bench")
	}
}

func BenchmarkPipeline_Plain(b *testing.B) {
	gen := NewLogGenerator(42, DefaultConfig())

	lines := make([][]byte, b.N)
	for i := range lines {
		lines[i] = gen.NextPlain()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		parse.ParseNormalized(string(lines[i]), "bench")
	}
}

func BenchmarkLatency_E2E(b *testing.B) {
	gen := NewLogGenerator(99, DefaultConfig())

	lines := make([][]byte, b.N)
	for i := range lines {
		lines[i] = gen.NextJSON()
	}

	durations := make([]time.Duration, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t0 := time.Now()
		parse.ParseNormalized(string(lines[i]), "bench")
		durations[i] = time.Since(t0)
	}
	b.StopTimer()

	sortDurations(durations)
	p50 := durations[b.N*50/100]
	p95 := durations[b.N*95/100]
	p99 := durations[b.N*99/100]

	b.ReportMetric(float64(p50.Microseconds()), "p50_µs")
	b.ReportMetric(float64(p95.Microseconds()), "p95_µs")
	b.ReportMetric(float64(p99.Microseconds()), "p99_µs")
}

func BenchmarkMemory_1M(b *testing.B) {
	const total = 1_000_000
	gen := NewLogGenerator(7, DefaultConfig())

	lines := make([]string, total)
	for i := range lines {
		lines[i] = string(gen.NextJSON())
	}

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		sink := make([]*event.NormalizedEvent, 0, total)
		for _, l := range lines {
			sink = append(sink, parse.ParseNormalized(l, "bench"))
		}
		_ = sink
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	b.ReportMetric(float64(ms.HeapInuse)/1024/1024, "heap_MB")
}

func BenchmarkThroughput_1K(b *testing.B) { benchThroughput(b, 1_000) }

func BenchmarkThroughput_10K(b *testing.B) { benchThroughput(b, 10_000) }

func BenchmarkThroughput_100K(b *testing.B) { benchThroughput(b, 100_000) }

func benchThroughput(b *testing.B, targetRPS int) {
	b.Helper()
	gen := NewLogGenerator(1, DefaultConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interval := time.Second / time.Duration(targetRPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	const ringSize = 10_000
	ring := make([]string, ringSize)
	for i := range ring {
		ring[i] = string(gen.NextJSON())
	}

	processed := 0
	start := time.Now()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
		parse.ParseNormalized(ring[i%ringSize], "bench")
		processed++
	}

	elapsed := time.Since(start)
	actualRPS := float64(processed) / elapsed.Seconds()
	b.ReportMetric(actualRPS, "actual_ev/s")

	_ = strings.TrimSpace
}

func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j] < d[j-1]; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}
