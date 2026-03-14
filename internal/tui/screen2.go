package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"collector/internal/graph"
)

// EdgeRow holds display data for one edge in the dependency view.
type EdgeRow struct {
	Key        string
	Peer       string // the other service (not the focused one)
	Direction  string // "upstream" or "downstream"
	Operation  string
	AvgLatency time.Duration
	P99Latency time.Duration
	ErrorRate  float64
	CallCount  int64
	IsAnomaly  bool
	IsCycle    bool
}

// Screen2 shows the dependency detail for one service.
type Screen2 struct {
	service      string
	upstream     []EdgeRow
	downstream   []EdgeRow
	allRows      []EdgeRow // upstream then downstream, for cursor navigation
	cursor       int
	cycleEdges   map[string]bool
	anomalyEdges map[string]bool
	width        int
	height       int
}

func NewScreen2() Screen2 {
	return Screen2{
		cycleEdges:   make(map[string]bool),
		anomalyEdges: make(map[string]bool),
	}
}

func (s *Screen2) SetSize(w, h int) {
	s.width = w
	s.height = h
}

func (s *Screen2) SetService(service string) {
	s.service = service
	s.cursor = 0
}

func (s *Screen2) SetCycleEdges(keys map[string]bool) {
	s.cycleEdges = keys
}

func (s *Screen2) SetAnomalyEdges(keys map[string]bool) {
	s.anomalyEdges = keys
}

func (s *Screen2) Update(snap graph.CallGraphSnapshot) {
	s.upstream = nil
	s.downstream = nil

	for _, e := range snap.Edges {
		key := fmt.Sprintf("%s|%s|%s", e.Src, e.Dst, e.Operation)
		row := EdgeRow{
			Key:        key,
			Operation:  e.Operation,
			AvgLatency: e.AvgLatency(),
			P99Latency: e.LatencyP99,
			ErrorRate:  e.ErrorRate(),
			CallCount:  e.CallCount,
			IsAnomaly:  s.anomalyEdges[key],
			IsCycle:    s.cycleEdges[key],
		}

		if e.Dst == s.service {
			row.Peer = e.Src
			row.Direction = "upstream"
			s.upstream = append(s.upstream, row)
		} else if e.Src == s.service {
			row.Peer = e.Dst
			row.Direction = "downstream"
			s.downstream = append(s.downstream, row)
		}
	}

	s.allRows = append(s.upstream, s.downstream...)
	if s.cursor >= len(s.allRows) {
		s.cursor = max(0, len(s.allRows)-1)
	}
}

func (s *Screen2) HandleKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.allRows)-1 {
			s.cursor++
		}
	}
}

func (s *Screen2) SelectedEdge() *EdgeRow {
	if len(s.allRows) == 0 || s.cursor >= len(s.allRows) {
		return nil
	}
	e := s.allRows[s.cursor]
	return &e
}

func (s *Screen2) View(riskScore float64) string {
	var b strings.Builder

	// Title bar
	riskStr := RiskStyle(riskScore).Render(fmt.Sprintf("risk: %.1f", riskScore))
	title := StyleTitle.Render(fmt.Sprintf("⬡ Service: %s", s.service))
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		title,
		strings.Repeat(" ", max(0, s.width-lipgloss.Width(title)-lipgloss.Width(riskStr)-2)),
		riskStr,
	)
	b.WriteString(header + "\n\n")

	// Upstream section
	b.WriteString(StyleBold.Render("▲ Upstream") +
		StyleDim.Render(fmt.Sprintf("  (%d services calling this)", len(s.upstream))) + "\n")

	if len(s.upstream) == 0 {
		b.WriteString(StyleDim.Render("  — no upstream dependencies —") + "\n")
	} else {
		b.WriteString(s.renderEdgeHeader() + "\n")
		for i, row := range s.upstream {
			globalIdx := i
			b.WriteString(s.renderEdgeRow(row, globalIdx == s.cursor) + "\n")
		}
	}

	b.WriteString("\n")

	// Downstream section
	b.WriteString(StyleBold.Render("▼ Downstream") +
		StyleDim.Render(fmt.Sprintf("  (%d services this calls)", len(s.downstream))) + "\n")

	if len(s.downstream) == 0 {
		b.WriteString(StyleDim.Render("  — no downstream dependencies —") + "\n")
	} else {
		b.WriteString(s.renderEdgeHeader() + "\n")
		for i, row := range s.downstream {
			globalIdx := len(s.upstream) + i
			b.WriteString(s.renderEdgeRow(row, globalIdx == s.cursor) + "\n")
		}
	}

	return b.String()
}

func (s *Screen2) renderEdgeHeader() string {
	return StyleDim.Render(fmt.Sprintf("  %-20s %-30s %10s %10s %10s %10s",
		"PEER", "OPERATION", "AVG LAT", "P99 LAT", "ERR%", "CALLS/MIN"))
}

func (s *Screen2) renderEdgeRow(row EdgeRow, selected bool) string {
	badges := ""
	if row.IsAnomaly {
		badges += StyleAnomalyBadge.Render("⚠ ")
	}
	if row.IsCycle {
		badges += StyleCycleBadge.Render("↺ ")
	}

	op := row.Operation
	if op == "" {
		op = "—"
	}
	if len(op) > 28 {
		op = op[:27] + "…"
	}

	errStr := fmt.Sprintf("%.2f%%", row.ErrorRate*100)
	callsMin := callsPerMin(row.CallCount)

	line := fmt.Sprintf("  %-20s %-30s %10s %10s %10s %10s %s",
		truncateName(row.Peer, 20),
		op,
		formatDuration(row.AvgLatency),
		formatDuration(row.P99Latency),
		errStr,
		callsMin,
		badges,
	)

	if row.IsAnomaly {
		line = StyleError.Render(line)
	}
	if selected {
		line = StyleSelected.Width(s.width).Render(line)
	}
	return line
}

func callsPerMin(total int64) string {
	if total == 0 {
		return "—"
	}
	// Approximate — would need time window for real calls/min
	return fmt.Sprintf("~%d", total)
}
