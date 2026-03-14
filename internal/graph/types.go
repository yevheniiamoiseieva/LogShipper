package graph

import (
	"time"
)

type NodeID = string

type GraphEventType int

const (
	GraphEventNewEdge GraphEventType = iota
	GraphEventNewCycle
	GraphEventEdgeGone
)

func (t GraphEventType) String() string {
	switch t {
	case GraphEventNewEdge:
		return "NewEdge"
	case GraphEventNewCycle:
		return "NewCycle"
	case GraphEventEdgeGone:
		return "EdgeGone"
	default:
		return "Unknown"
	}
}

type Edge struct {
	Src       NodeID
	Dst       NodeID
	Operation string

	CallCount  int64
	ErrorCount int64
	LatencySum time.Duration
	LatencyP99 time.Duration

	LastSeen  time.Time
	FirstSeen time.Time

	latencyWindow []time.Duration
}

func (e *Edge) ErrorRate() float64 {
	if e.CallCount == 0 {
		return 0
	}
	return float64(e.ErrorCount) / float64(e.CallCount)
}

func (e *Edge) AvgLatency() time.Duration {
	if e.CallCount == 0 {
		return 0
	}
	return e.LatencySum / time.Duration(e.CallCount)
}

func (e *Edge) addLatency(d time.Duration) {
	const windowSize = 100

	e.latencyWindow = append(e.latencyWindow, d)
	if len(e.latencyWindow) > windowSize {
		e.latencyWindow = e.latencyWindow[len(e.latencyWindow)-windowSize:]
	}
	e.LatencyP99 = calcP99(e.latencyWindow)
}

func calcP99(vals []time.Duration) time.Duration {
	if len(vals) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(vals))
	copy(sorted, vals)

	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	idx := int(float64(len(sorted)-1) * 0.99)
	return sorted[idx]
}

type GraphEvent struct {
	Type      GraphEventType
	Edge      Edge
	Cycle     []NodeID
	Timestamp time.Time
}

type CallGraphSnapshot struct {
	Nodes []NodeID
	Edges []Edge
	At    time.Time
}
