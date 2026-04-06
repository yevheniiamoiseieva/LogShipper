package bench

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"collector/internal/anomaly"
)

const (
	evalNormal    = 10_000
	evalAnomalies = 50
	evalMean      = 50.0
	evalStdDev    = 10.0
)

type evalDataset struct {
	values []float64
	labels []bool
}

func buildDataset(seed int64) evalDataset {
	rng := rand.New(rand.NewSource(seed))

	values := make([]float64, evalNormal)
	labels := make([]bool, evalNormal)

	for i := range values {
		values[i] = rng.NormFloat64()*evalStdDev + evalMean
	}

	positions := rng.Perm(evalNormal)[:evalAnomalies]
	kValues := []float64{4, 5, 6}
	for i, pos := range positions {
		k := kValues[i%len(kValues)]
		values[pos] = evalMean + k*evalStdDev
		labels[pos] = true
	}

	return evalDataset{values: values, labels: labels}
}

type evalResult struct {
	Threshold float64
	Window    int
	TP        int
	FP        int
	FN        int
	Precision float64
	Recall    float64
	F1        float64
}

func (r evalResult) String() string {
	return fmt.Sprintf(
		"thresh=%.1f window=%3d | P=%.3f R=%.3f F1=%.3f | TP=%d FP=%d FN=%d",
		r.Threshold, r.Window, r.Precision, r.Recall, r.F1, r.TP, r.FP, r.FN,
	)
}

func evaluate(ds evalDataset, threshold float64, windowSize int) evalResult {
	det := anomaly.NewZScoreDetector(windowSize, threshold, 1024).
		WithMinSamples(windowSize / 2).
		WithCooldown(0)

	detected := make([]bool, len(ds.values))

	const edgeKey = "eval:eval"
	const metric = "value"

	for i, v := range ds.values {
		det.Feed(edgeKey, metric, v)
	drainLoop:
		for {
			select {
			case ev := <-det.Events():
				_ = ev
				detected[i] = true
			default:
				break drainLoop
			}
		}
	}

	var tp, fp, fn int
	for i := range ds.values {
		switch {
		case ds.labels[i] && detected[i]:
			tp++
		case !ds.labels[i] && detected[i]:
			fp++
		case ds.labels[i] && !detected[i]:
			fn++
		}
	}

	var precision, recall, f1 float64
	if tp+fp > 0 {
		precision = float64(tp) / float64(tp+fp)
	}
	if tp+fn > 0 {
		recall = float64(tp) / float64(tp+fn)
	}
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}

	return evalResult{
		Threshold: threshold,
		Window:    windowSize,
		TP:        tp, FP: fp, FN: fn,
		Precision: precision,
		Recall:    recall,
		F1:        f1,
	}
}

func TestAnomalyEval(t *testing.T) {
	ds := buildDataset(42)

	configs := []struct {
		threshold float64
		window    int
	}{
		{2.0, 50},
		{2.5, 50},
		{3.0, 50},
		{3.0, 100},
		{3.0, 200},
		{3.5, 100},
	}

	results := make([]evalResult, 0, len(configs))
	for _, c := range configs {
		r := evaluate(ds, c.threshold, c.window)
		results = append(results, r)
		t.Logf("%s", r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].F1 > results[j].F1
	})
	best := results[0]
	t.Logf("\nBest config: %s", best)

	if best.F1 < 0.85 {
		t.Errorf("best F1=%.3f < 0.85 target; consider tuning detector parameters", best.F1)
	}
}

func BenchmarkAnomalyDetector(b *testing.B) {
	det := anomaly.NewZScoreDetector(100, 3.0, 4096).
		WithMinSamples(50).
		WithCooldown(0)

	rng := rand.New(rand.NewSource(1)) //nolint:gosec
	values := make([]float64, b.N)
	for i := range values {
		values[i] = rng.NormFloat64()*evalStdDev + evalMean
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		det.Feed("bench:svc", "latency", values[i])
		select {
		case <-det.Events():
		default:
		}
	}
}

func TestDatasetSanity(t *testing.T) {
	ds := buildDataset(42)
	anomalyCount := 0
	for _, l := range ds.labels {
		if l {
			anomalyCount++
		}
	}
	if anomalyCount != evalAnomalies {
		t.Fatalf("expected %d anomalies, got %d", evalAnomalies, anomalyCount)
	}
	for i, v := range ds.values {
		if ds.labels[i] {
			zAbs := math.Abs(v-evalMean) / evalStdDev
			if zAbs < 3.9 {
				t.Errorf("anomaly at index %d has |z|=%.2f < 4", i, zAbs)
			}
		}
	}
	t.Logf("Dataset OK: %d normal + %d anomalies", evalNormal-anomalyCount, anomalyCount)
}

func init() {
	_ = time.Second
}
