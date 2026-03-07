package graph

import (
	"context"
	"sort"
	"testing"
	"time"
)

func makeEvent(src, dst, op string, latency time.Duration, isErr bool) *NormalizedEvent {
	return &NormalizedEvent{
		SrcService: src,
		DstService: dst,
		Operation:  op,
		IsError:    isErr,
		Latency:    latency,
		OccurredAt: time.Now(),
	}
}

func drainEvents(ch <-chan GraphEvent, timeout time.Duration) []GraphEvent {
	var out []GraphEvent
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

func sortNodes(ns []NodeID) []NodeID {
	sort.Strings(ns)
	return ns
}

func filterByType(evs []GraphEvent, t GraphEventType) []GraphEvent {
	var out []GraphEvent
	for _, e := range evs {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func TestEdge_ErrorRate(t *testing.T) {
	e := &Edge{CallCount: 10, ErrorCount: 3}
	if got := e.ErrorRate(); got != 0.3 {
		t.Errorf("ErrorRate = %v, want 0.3", got)
	}
}

func TestEdge_ErrorRate_ZeroCalls(t *testing.T) {
	e := &Edge{}
	if got := e.ErrorRate(); got != 0 {
		t.Errorf("ErrorRate with zero calls = %v, want 0", got)
	}
}

func TestEdge_AvgLatency(t *testing.T) {
	e := &Edge{CallCount: 4, LatencySum: 400 * time.Millisecond}
	if got := e.AvgLatency(); got != 100*time.Millisecond {
		t.Errorf("AvgLatency = %v, want 100ms", got)
	}
}

func TestEdge_AvgLatency_ZeroCalls(t *testing.T) {
	e := &Edge{}
	if got := e.AvgLatency(); got != 0 {
		t.Errorf("AvgLatency with zero calls = %v, want 0", got)
	}
}

func TestEdge_P99Latency(t *testing.T) {
	e := &Edge{}
	for i := 1; i <= 100; i++ {
		e.addLatency(time.Duration(i) * time.Millisecond)
	}
	if e.LatencyP99 != 99*time.Millisecond {
		t.Errorf("LatencyP99 = %v, want 99ms", e.LatencyP99)
	}
}

func TestEdge_P99SlidingWindow(t *testing.T) {
	e := &Edge{}
	for i := 1; i <= 100; i++ {
		e.addLatency(time.Duration(i) * time.Millisecond)
	}
	for i := 0; i < 100; i++ {
		e.addLatency(500 * time.Millisecond)
	}
	if e.LatencyP99 != 500*time.Millisecond {
		t.Errorf("after slide, LatencyP99 = %v, want 500ms", e.LatencyP99)
	}
}

func TestCalcP99_Empty(t *testing.T) {
	if got := calcP99(nil); got != 0 {
		t.Errorf("calcP99(nil) = %v, want 0", got)
	}
}

func TestCalcP99_Single(t *testing.T) {
	if got := calcP99([]time.Duration{42 * time.Millisecond}); got != 42*time.Millisecond {
		t.Errorf("calcP99 single = %v, want 42ms", got)
	}
}

func TestGraph_New(t *testing.T) {
	g := New(64)
	if g == nil {
		t.Fatal("New returned nil")
	}
	if len(g.Nodes()) != 0 {
		t.Error("new graph should have no nodes")
	}
	if len(g.Edges()) != 0 {
		t.Error("new graph should have no edges")
	}
}

func TestGraph_Feed_AddsNodesAndEdge(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "call", 10*time.Millisecond, false))

	nodes := sortNodes(g.Nodes())
	if len(nodes) != 2 || nodes[0] != "A" || nodes[1] != "B" {
		t.Errorf("unexpected nodes: %v", nodes)
	}

	edges := g.Edges()
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Src != "A" || edges[0].Dst != "B" {
		t.Errorf("unexpected edge: %+v", edges[0])
	}
}

func TestGraph_Feed_Aggregation(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "op", 10*time.Millisecond, false))
	g.Feed(makeEvent("A", "B", "op", 20*time.Millisecond, true))
	g.Feed(makeEvent("A", "B", "op", 30*time.Millisecond, false))

	edges := g.Edges()
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	e := edges[0]
	if e.CallCount != 3 {
		t.Errorf("CallCount = %d, want 3", e.CallCount)
	}
	if e.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", e.ErrorCount)
	}
	if e.LatencySum != 60*time.Millisecond {
		t.Errorf("LatencySum = %v, want 60ms", e.LatencySum)
	}
	if e.AvgLatency() != 20*time.Millisecond {
		t.Errorf("AvgLatency = %v, want 20ms", e.AvgLatency())
	}
}

func TestGraph_Feed_DifferentOperations(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "read", 5*time.Millisecond, false))
	g.Feed(makeEvent("A", "B", "write", 5*time.Millisecond, false))

	if len(g.Edges()) != 2 {
		t.Errorf("expected 2 edges (different ops), got %d", len(g.Edges()))
	}
}

func TestGraph_Feed_NilIgnored(t *testing.T) {
	g := New(64)
	g.Feed(nil)
	if len(g.Edges()) != 0 {
		t.Error("nil event should not add edges")
	}
}

func TestGraph_Feed_EmptySrcDst(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("", "B", "op", 0, false))
	g.Feed(makeEvent("A", "", "op", 0, false))
	if len(g.Edges()) != 0 {
		t.Error("empty src/dst should be ignored")
	}
}

func TestGraph_EdgesFrom(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "op", 0, false))
	g.Feed(makeEvent("A", "C", "op", 0, false))
	g.Feed(makeEvent("X", "Y", "op", 0, false))

	if len(g.EdgesFrom("A")) != 2 {
		t.Errorf("EdgesFrom(A) = %d, want 2", len(g.EdgesFrom("A")))
	}
}

func TestGraph_EdgesTo(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "Z", "op", 0, false))
	g.Feed(makeEvent("B", "Z", "op", 0, false))
	g.Feed(makeEvent("C", "X", "op", 0, false))

	if len(g.EdgesTo("Z")) != 2 {
		t.Errorf("EdgesTo(Z) = %d, want 2", len(g.EdgesTo("Z")))
	}
}

func TestGraph_Snapshot(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "op", 5*time.Millisecond, false))

	snap := g.Snapshot()
	if len(snap.Nodes) != 2 {
		t.Errorf("snapshot nodes = %d, want 2", len(snap.Nodes))
	}
	if len(snap.Edges) != 1 {
		t.Errorf("snapshot edges = %d, want 1", len(snap.Edges))
	}
	if snap.At.IsZero() {
		t.Error("snapshot timestamp should not be zero")
	}
}

func TestGraph_Events_NewEdge(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "op", 0, false))

	evs := drainEvents(g.Events(), 50*time.Millisecond)
	newEdgeEvs := filterByType(evs, GraphEventNewEdge)
	if len(newEdgeEvs) != 1 {
		t.Fatalf("expected 1 NewEdge event, got %d", len(newEdgeEvs))
	}
	if newEdgeEvs[0].Edge.Src != "A" || newEdgeEvs[0].Edge.Dst != "B" {
		t.Errorf("wrong edge in event: %+v", newEdgeEvs[0].Edge)
	}
}

func TestGraph_Events_NewEdge_OnlyOnce(t *testing.T) {
	g := New(64)
	for i := 0; i < 5; i++ {
		g.Feed(makeEvent("A", "B", "op", 0, false))
	}

	evs := drainEvents(g.Events(), 50*time.Millisecond)
	if len(filterByType(evs, GraphEventNewEdge)) != 1 {
		t.Errorf("NewEdge event should fire exactly once, got %d", len(filterByType(evs, GraphEventNewEdge)))
	}
}

func TestGraph_Events_Cycle_Simple(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "op", 0, false))
	g.Feed(makeEvent("B", "A", "op", 0, false))

	evs := drainEvents(g.Events(), 100*time.Millisecond)
	cycleEvs := filterByType(evs, GraphEventNewCycle)
	if len(cycleEvs) == 0 {
		t.Fatal("expected NewCycle event for A→B→A")
	}
	if len(cycleEvs[0].Cycle) < 2 {
		t.Errorf("cycle path too short: %v", cycleEvs[0].Cycle)
	}
}

func TestGraph_Events_Cycle_Triangle(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "op", 0, false))
	g.Feed(makeEvent("B", "C", "op", 0, false))
	g.Feed(makeEvent("C", "A", "op", 0, false))

	evs := drainEvents(g.Events(), 100*time.Millisecond)
	if len(filterByType(evs, GraphEventNewCycle)) == 0 {
		t.Fatal("expected NewCycle event for triangle A→B→C→A")
	}
}

func TestGraph_Events_Cycle_NoDuplicates(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "op", 0, false))
	g.Feed(makeEvent("B", "A", "op", 0, false))
	g.Feed(makeEvent("A", "B", "op", 0, false))
	g.Feed(makeEvent("B", "A", "op", 0, false))

	evs := drainEvents(g.Events(), 100*time.Millisecond)
	if len(filterByType(evs, GraphEventNewCycle)) != 1 {
		t.Errorf("expected exactly 1 NewCycle event, got %d", len(filterByType(evs, GraphEventNewCycle)))
	}
}

func TestGraph_NoCycle_DAG(t *testing.T) {
	g := New(64)
	g.Feed(makeEvent("A", "B", "op", 0, false))
	g.Feed(makeEvent("B", "C", "op", 0, false))

	evs := drainEvents(g.Events(), 100*time.Millisecond)
	if len(filterByType(evs, GraphEventNewCycle)) != 0 {
		t.Errorf("expected no NewCycle for DAG, got %d", len(filterByType(evs, GraphEventNewCycle)))
	}
}

func TestGraph_Events_EdgeGone(t *testing.T) {
	cfg := defaultConfig()
	cfg.EdgeTTL = 50 * time.Millisecond
	cfg.StaleScanInterval = 20 * time.Millisecond
	g := NewWithConfig(cfg)
	g.Start(context.Background())

	g.Feed(makeEvent("A", "B", "op", 0, false))
	time.Sleep(200 * time.Millisecond)

	evs := drainEvents(g.Events(), 100*time.Millisecond)
	goneEvs := filterByType(evs, GraphEventEdgeGone)
	if len(goneEvs) == 0 {
		t.Fatal("expected EdgeGone event after TTL expiry")
	}
	if goneEvs[0].Edge.Src != "A" || goneEvs[0].Edge.Dst != "B" {
		t.Errorf("wrong edge in EdgeGone event: %+v", goneEvs[0].Edge)
	}
}

func TestGraph_EdgeGone_RemovesFromEdges(t *testing.T) {
	cfg := defaultConfig()
	cfg.EdgeTTL = 30 * time.Millisecond
	cfg.StaleScanInterval = 10 * time.Millisecond
	g := NewWithConfig(cfg)
	g.Start(context.Background())

	g.Feed(makeEvent("X", "Y", "op", 0, false))
	time.Sleep(150 * time.Millisecond)

	if len(g.Edges()) != 0 {
		t.Errorf("stale edge should be removed, got %d edges", len(g.Edges()))
	}
}

func TestGraph_EdgeGone_ReportedAsNewAfterRemoval(t *testing.T) {
	cfg := defaultConfig()
	cfg.EdgeTTL = 30 * time.Millisecond
	cfg.StaleScanInterval = 10 * time.Millisecond
	g := NewWithConfig(cfg)
	g.Start(context.Background())

	g.Feed(makeEvent("A", "B", "op", 0, false))
	time.Sleep(150 * time.Millisecond)

	g.Feed(makeEvent("A", "B", "op", 0, false))

	evs := drainEvents(g.Events(), 100*time.Millisecond)
	if len(filterByType(evs, GraphEventNewEdge)) < 2 {
		t.Errorf("expected 2 NewEdge events (initial + re-appear), got %d", len(filterByType(evs, GraphEventNewEdge)))
	}
}

func TestCycleKey_Normalisation(t *testing.T) {
	k1 := cycleKey([]NodeID{"A", "B", "C", "A"})
	k2 := cycleKey([]NodeID{"B", "C", "A", "B"})
	if k1 != k2 {
		t.Errorf("cycleKey not normalised: %q vs %q", k1, k2)
	}
}

func TestCycleKey_Empty(t *testing.T) {
	if got := cycleKey(nil); got != "" {
		t.Errorf("cycleKey(nil) = %q, want empty", got)
	}
}
