package graph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"collector/internal/metrics"
)

type NormalizedEvent struct {
	SrcService string
	DstService string
	Operation  string
	IsError    bool
	Latency    time.Duration
	OccurredAt time.Time
}

func edgeKey(src, dst, op string) string {
	return fmt.Sprintf("%s|%s|%s", src, dst, op)
}

type CallGraph struct {
	mu  sync.RWMutex
	cfg Config

	nodes map[NodeID]struct{}
	edges map[string]*Edge

	knownEdges map[string]bool

	cycles   *cycleDetector
	events   chan GraphEvent
	detector anomalyFeeder
}

func New(bufSize int) *CallGraph {
	cfg := defaultConfig()
	cfg.EventBufSize = bufSize
	cfg.applyDefaults()
	return NewWithConfig(cfg)
}

func NewWithConfig(cfg Config) *CallGraph {
	cfg.applyDefaults()
	g := &CallGraph{
		cfg:        cfg,
		nodes:      make(map[NodeID]struct{}),
		edges:      make(map[string]*Edge),
		knownEdges: make(map[string]bool),
		cycles:     newCycleDetector(),
		events:     make(chan GraphEvent, cfg.EventBufSize),
	}
	return g
}

func (g *CallGraph) Start(ctx context.Context) {
	go g.staleSweeper(ctx)
}

func (g *CallGraph) Feed(ev *NormalizedEvent) {
	if ev == nil {
		return
	}

	src, dst, op := ev.SrcService, ev.DstService, ev.Operation
	if src == "" || dst == "" {
		return
	}

	key := edgeKey(src, dst, op)
	isNew := false

	g.mu.Lock()

	g.nodes[src] = struct{}{}
	g.nodes[dst] = struct{}{}

	edge, exists := g.edges[key]
	if !exists {
		now := ev.OccurredAt
		if now.IsZero() {
			now = time.Now()
		}
		edge = &Edge{
			Src:       src,
			Dst:       dst,
			Operation: op,
			FirstSeen: now,
		}
		g.edges[key] = edge
	}

	ts := ev.OccurredAt
	if ts.IsZero() {
		ts = time.Now()
	}
	edge.CallCount++
	edge.LatencySum += ev.Latency
	edge.LastSeen = ts
	if ev.IsError {
		edge.ErrorCount++
	}
	edge.addLatency(ev.Latency)

	if !g.knownEdges[key] {
		g.knownEdges[key] = true
		isNew = true
	}

	adj := g.buildAdjacency()
	edgeCopy := *edge
	nodeCount := len(adj)
	edgeCount := len(g.edges)

	g.mu.Unlock()

	if isNew {
		metrics.GraphNewEdges.Inc()
		g.emit(GraphEvent{
			Type:      GraphEventNewEdge,
			Edge:      edgeCopy,
			Timestamp: ts,
		})

		newCycles := g.cycles.findNewCycles(adj)
		for _, cycle := range newCycles {
			metrics.GraphCycles.Inc()
			g.emit(GraphEvent{
				Type:      GraphEventNewCycle,
				Edge:      edgeCopy,
				Cycle:     cycle,
				Timestamp: ts,
			})
		}
	}

	metrics.GraphNodes.Set(float64(nodeCount))
	metrics.GraphEdges.Set(float64(edgeCount))
	if g.detector != nil {
		g.detector.Feed(key, "latency", float64(ev.Latency.Milliseconds()))
		g.detector.Feed(key, "error_rate", edge.ErrorRate())
	}

}

func (g *CallGraph) buildAdjacency() map[NodeID][]NodeID {
	adj := make(map[NodeID][]NodeID, len(g.nodes))
	for id := range g.nodes {
		adj[id] = nil
	}
	for _, e := range g.edges {
		adj[e.Src] = append(adj[e.Src], e.Dst)
	}
	return adj
}

func (g *CallGraph) emit(ev GraphEvent) {
	select {
	case g.events <- ev:
	default:
	}
}

func (g *CallGraph) Edges() []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]Edge, 0, len(g.edges))
	for _, e := range g.edges {
		result = append(result, *e)
	}
	return result
}

func (g *CallGraph) EdgesFrom(src NodeID) []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []Edge
	for _, e := range g.edges {
		if e.Src == src {
			result = append(result, *e)
		}
	}
	return result
}

func (g *CallGraph) EdgesTo(dst NodeID) []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []Edge
	for _, e := range g.edges {
		if e.Dst == dst {
			result = append(result, *e)
		}
	}
	return result
}

func (g *CallGraph) Nodes() []NodeID {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]NodeID, 0, len(g.nodes))
	for id := range g.nodes {
		result = append(result, id)
	}
	return result
}

func (g *CallGraph) Events() <-chan GraphEvent {
	return g.events
}

func (g *CallGraph) Snapshot() CallGraphSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]NodeID, 0, len(g.nodes))
	for id := range g.nodes {
		nodes = append(nodes, id)
	}

	edges := make([]Edge, 0, len(g.edges))
	for _, e := range g.edges {
		edges = append(edges, *e)
	}

	return CallGraphSnapshot{
		Nodes: nodes,
		Edges: edges,
		At:    time.Now(),
	}
}

func (g *CallGraph) staleSweeper(ctx context.Context) {
	ticker := time.NewTicker(g.cfg.StaleScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.sweepStale()
		}
	}
}

func (g *CallGraph) sweepStale() {
	deadline := time.Now().Add(-g.cfg.EdgeTTL)

	g.mu.Lock()
	var gone []Edge
	for key, e := range g.edges {
		if e.LastSeen.Before(deadline) {
			gone = append(gone, *e)
			delete(g.edges, key)
			delete(g.knownEdges, key)
		}
	}
	g.rebuildNodes()
	g.mu.Unlock()

	now := time.Now()
	for _, e := range gone {
		g.emit(GraphEvent{
			Type:      GraphEventEdgeGone,
			Edge:      e,
			Timestamp: now,
		})
	}
}

func (g *CallGraph) rebuildNodes() {
	newNodes := make(map[NodeID]struct{}, len(g.nodes))
	for _, e := range g.edges {
		newNodes[e.Src] = struct{}{}
		newNodes[e.Dst] = struct{}{}
	}
	g.nodes = newNodes
}

type anomalyFeeder interface {
	Feed(edgeKey, metric string, value float64)
}

func (g *CallGraph) WithAnomalyDetector(d anomalyFeeder) *CallGraph {
	g.detector = d
	return g
}
