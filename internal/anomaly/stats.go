package anomaly

import "math"

type RollingStats struct {
	windowSize int
	count      int
	mean       float64
	m2         float64
	window     []float64
	pos        int
}

func NewRollingStats(windowSize int) *RollingStats {
	return &RollingStats{
		windowSize: windowSize,
		window:     make([]float64, 0, windowSize),
	}
}

func (s *RollingStats) Add(value float64) {
	if len(s.window) < s.windowSize {
		s.window = append(s.window, value)
		s.pos = len(s.window) - 1
	} else {
		s.pos = (s.pos + 1) % s.windowSize
		s.window[s.pos] = value
	}

	s.recalc()
}

func (s *RollingStats) recalc() {
	s.mean = 0
	s.m2 = 0
	s.count = len(s.window)

	for i, x := range s.window {
		delta := x - s.mean
		s.mean += delta / float64(i+1)
		delta2 := x - s.mean
		s.m2 += delta * delta2
	}
}

func (s *RollingStats) Mean() float64 {
	return s.mean
}

func (s *RollingStats) StdDev() float64 {
	if s.count < 2 {
		return 0
	}
	return math.Sqrt(s.m2 / float64(s.count))
}

func (s *RollingStats) Count() int {
	return s.count
}

func (s *RollingStats) ZScore(value float64) float64 {
	sd := s.StdDev()
	if sd == 0 {
		return 0
	}
	return (value - s.mean) / sd
}
