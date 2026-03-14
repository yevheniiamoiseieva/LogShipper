package tui

import (
	"time"

	"collector/internal/anomaly"
	"collector/internal/graph"
)

// Screen identifiers
type Screen int

const (
	ScreenServiceList Screen = iota
	ScreenDependency
	ScreenEventLog
)

// SortColumn identifies which column to sort by on Screen 1.
type SortColumn int

const (
	SortByRisk SortColumn = iota
	SortByService
	SortByAnomalies
	SortByLatency
	SortByErrorRate
)

// --- Messages ---

// TickMsg is sent every 500ms to refresh data.
type TickMsg time.Time

// SnapshotMsg carries a fresh graph snapshot.
type SnapshotMsg struct {
	Snapshot graph.CallGraphSnapshot
}

// AnomalyMsg carries a new anomaly event.
type AnomalyMsg struct {
	Event anomaly.AnomalyEvent
}

// GraphEventMsg carries a graph topology event.
type GraphEventMsg struct {
	Event graph.GraphEvent
}

// SelectServiceMsg navigates to Screen 2 for the given service.
type SelectServiceMsg struct {
	Service string
}

// SelectEdgeMsg navigates to Screen 3 for the given edge.
type SelectEdgeMsg struct {
	EdgeKey    string
	SrcService string
	DstService string
	Operation  string
}

// BackMsg navigates back one screen.
type BackMsg struct{}

// FilterMsg updates the filter string on Screen 1.
type FilterMsg struct {
	Text string
}

// ToggleHelpMsg shows/hides the help overlay.
type ToggleHelpMsg struct{}

// ToggleAutoScrollMsg toggles autoscroll on Screen 3.
type ToggleAutoScrollMsg struct{}
