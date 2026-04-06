package anomaly

import (
	"math"
	"sync"
	"time"

	"collector/internal/metrics"
)

type AnomalyEvent struct {
	EdgeKey   string
	Metric    string
	Value     float64
	ZScore    float64
	Mean      float64
	StdDev    float64
	Threshold float64
	Timestamp time.Time
}

type ZScoreDetector struct {
	windowSize int
	threshold  float64
	minSamples int
	cooldown   time.Duration

	mu          sync.Mutex
	stats       map[string]*RollingStats
	inAnomaly   map[string]bool
	lastAlerted map[string]time.Time

	out chan AnomalyEvent
}

func NewZScoreDetector(windowSize int, threshold float64, bufSize int) *ZScoreDetector {
	return &ZScoreDetector{
		windowSize:  windowSize,
		threshold:   threshold,
		minSamples:  windowSize / 2,
		cooldown:    30 * time.Second,
		stats:       make(map[string]*RollingStats),
		inAnomaly:   make(map[string]bool),
		lastAlerted: make(map[string]time.Time),
		out:         make(chan AnomalyEvent, bufSize),
	}
}

func (d *ZScoreDetector) WithMinSamples(n int) *ZScoreDetector {
	d.minSamples = n
	return d
}

func (d *ZScoreDetector) WithCooldown(cd time.Duration) *ZScoreDetector {
	d.cooldown = cd
	return d
}

func (d *ZScoreDetector) Feed(edgeKey, metric string, value float64) {
	key := edgeKey + ":" + metric

	d.mu.Lock()
	defer d.mu.Unlock()

	s, ok := d.stats[key]
	if !ok {
		s = NewRollingStats(d.windowSize)
		d.stats[key] = s
	}

	s.Add(value)

	if s.Count() < d.minSamples {
		return
	}

	z := s.ZScore(value)
	isAnomaly := math.Abs(z) > d.threshold

	if !isAnomaly {
		d.inAnomaly[key] = false
		return
	}

	if d.inAnomaly[key] {
		return
	}

	if last, ok := d.lastAlerted[key]; ok && time.Since(last) < d.cooldown {
		return
	}

	d.inAnomaly[key] = true
	d.lastAlerted[key] = time.Now()
	metrics.AnomaliesTotal.WithLabelValues(metric).Inc()

	ev := AnomalyEvent{
		EdgeKey:   edgeKey,
		Metric:    metric,
		Value:     value,
		ZScore:    z,
		Mean:      s.Mean(),
		StdDev:    s.StdDev(),
		Threshold: d.threshold,
		Timestamp: time.Now(),
	}

	select {
	case d.out <- ev:
	default:
	}
}

func (d *ZScoreDetector) Events() <-chan AnomalyEvent {
	return d.out
}

func (d *ZScoreDetector) Stats(edgeKey, metric string) (mean, stddev float64) {
	key := edgeKey + ":" + metric

	d.mu.Lock()
	defer d.mu.Unlock()

	s, ok := d.stats[key]
	if !ok {
		return 0, 0
	}
	return s.Mean(), s.StdDev()
}
