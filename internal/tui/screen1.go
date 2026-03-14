package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"collector/internal/graph"
)

// ServiceRow holds computed display data for one service.
type ServiceRow struct {
	Name        string
	Incoming    int
	Outgoing    int
	Anomalies   int
	AvgLatency  time.Duration
	ErrorRate   float64
	RiskScore   float64
	HasNewAlert bool
}

// Screen1 is the main service list view.
type Screen1 struct {
	rows        []ServiceRow
	filtered    []ServiceRow
	cursor      int
	sortCol     SortColumn
	sortAsc     bool
	filterMode  bool
	filterInput textinput.Model
	blinkOn     bool
	width       int
	height      int
}

func NewScreen1() Screen1 {
	ti := textinput.New()
	ti.Placeholder = "filter services..."
	ti.CharLimit = 64
	return Screen1{
		sortCol: SortByRisk,
	}
}

func (s *Screen1) SetSize(w, h int) {
	s.width = w
	s.height = h
}

// Update rebuilds rows from a graph snapshot and anomaly counts.
func (s *Screen1) Update(snap graph.CallGraphSnapshot, anomalyCounts map[string]int, newAlerts map[string]bool) {
	inMap := make(map[string][]graph.Edge)
	outMap := make(map[string][]graph.Edge)
	for _, e := range snap.Edges {
		outMap[e.Src] = append(outMap[e.Src], e)
		inMap[e.Dst] = append(inMap[e.Dst], e)
	}

	rows := make([]ServiceRow, 0, len(snap.Nodes))
	for _, node := range snap.Nodes {
		out := outMap[node]
		in := inMap[node]

		var totalLatency time.Duration
		var totalCalls, totalErrors int64
		for _, e := range out {
			totalLatency += e.AvgLatency() * time.Duration(e.CallCount)
			totalCalls += e.CallCount
			totalErrors += e.ErrorCount
		}

		var avgLatency time.Duration
		var errRate float64
		if totalCalls > 0 {
			avgLatency = totalLatency / time.Duration(totalCalls)
			errRate = float64(totalErrors) / float64(totalCalls)
		}

		ac := anomalyCounts[node]
		risk := riskScore(ac, errRate, avgLatency)

		rows = append(rows, ServiceRow{
			Name:        node,
			Incoming:    len(in),
			Outgoing:    len(out),
			Anomalies:   ac,
			AvgLatency:  avgLatency,
			ErrorRate:   errRate,
			RiskScore:   risk,
			HasNewAlert: newAlerts[node],
		})
	}

	s.rows = rows
	s.applyFilterAndSort()
}

func riskScore(anomalies int, errRate float64, avgLatency time.Duration) float64 {
	ms := float64(avgLatency.Milliseconds())
	return float64(anomalies)*2.0 + errRate*10.0 + ms/100.0
}

func (s *Screen1) applyFilterAndSort() {
	filter := strings.ToLower(s.filterInput.Value())
	filtered := make([]ServiceRow, 0, len(s.rows))
	for _, r := range s.rows {
		if filter == "" || strings.Contains(strings.ToLower(r.Name), filter) {
			filtered = append(filtered, r)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		var less bool
		switch s.sortCol {
		case SortByService:
			less = a.Name < b.Name
		case SortByAnomalies:
			less = a.Anomalies > b.Anomalies
		case SortByLatency:
			less = a.AvgLatency > b.AvgLatency
		case SortByErrorRate:
			less = a.ErrorRate > b.ErrorRate
		default: // SortByRisk
			less = a.RiskScore > b.RiskScore
		}
		if s.sortAsc {
			return !less
		}
		return less
	})

	s.filtered = filtered
	if s.cursor >= len(s.filtered) {
		s.cursor = max(0, len(s.filtered)-1)
	}
}

func (s *Screen1) HandleKey(msg tea.KeyMsg) tea.Cmd {
	if s.filterMode {
		var cmd tea.Cmd
		s.filterInput, cmd = s.filterInput.Update(msg)
		switch msg.String() {
		case "enter", "esc":
			s.filterMode = false
			s.filterInput.Blur()
		}
		s.applyFilterAndSort()
		return cmd
	}

	switch {
	case msg.String() == "up", msg.String() == "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case msg.String() == "down", msg.String() == "j":
		if s.cursor < len(s.filtered)-1 {
			s.cursor++
		}
	case msg.String() == "s":
		s.sortCol = (s.sortCol + 1) % 5
		s.applyFilterAndSort()
	case msg.String() == "/":
		s.filterMode = true
		s.filterInput.Focus()
	}
	return nil
}

func (s *Screen1) SelectedService() string {
	if len(s.filtered) == 0 {
		return ""
	}
	return s.filtered[s.cursor].Name
}

func (s *Screen1) ToggleBlink() {
	s.blinkOn = !s.blinkOn
}

func (s *Screen1) View(width int) string {
	var b strings.Builder

	// Header
	title := StyleTitle.Render("⬡ Service Dependency Monitor")
	sortLabel := StyleDim.Render(fmt.Sprintf("sort: %s", sortColName(s.sortCol)))
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		title,
		strings.Repeat(" ", max(0, width-lipgloss.Width(title)-lipgloss.Width(sortLabel)-2)),
		sortLabel,
	)
	b.WriteString(header + "\n\n")

	// Filter input
	if s.filterMode {
		b.WriteString(StyleBorder.Render("/ "+s.filterInput.View()) + "\n")
	}

	// Column headers
	colHeaders := fmt.Sprintf("  %-22s %8s %8s %10s %12s %10s %10s",
		"SERVICE", "IN", "OUT", "ANOMALIES", "AVG LAT", "ERR RATE", "RISK")
	b.WriteString(StyleHeader.Width(width).Render(colHeaders) + "\n")

	// Rows
	visibleStart := max(0, s.cursor-s.visibleRows()/2)
	visibleEnd := min(len(s.filtered), visibleStart+s.visibleRows())

	for i := visibleStart; i < visibleEnd; i++ {
		row := s.filtered[i]
		line := s.renderRow(row, i == s.cursor, width)
		b.WriteString(line + "\n")
	}

	// Padding
	shown := visibleEnd - visibleStart
	for i := shown; i < s.visibleRows(); i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (s *Screen1) renderRow(r ServiceRow, selected bool, width int) string {
	anomalyStr := fmt.Sprintf("%d", r.Anomalies)
	if r.Anomalies > 0 {
		blink := ""
		if r.HasNewAlert && s.blinkOn {
			blink = " ●"
		}
		anomalyStr = StyleAnomalyBadge.Render(fmt.Sprintf("⚠ %d%s", r.Anomalies, blink))
	}

	riskStr := RiskStyle(r.RiskScore).Render(fmt.Sprintf("%.1f", r.RiskScore))
	errStr := fmt.Sprintf("%.2f%%", r.ErrorRate*100)

	line := fmt.Sprintf("  %-22s %8d %8d %10s %12s %10s %10s",
		truncateName(r.Name, 22),
		r.Incoming,
		r.Outgoing,
		anomalyStr,
		formatDuration(r.AvgLatency),
		errStr,
		riskStr,
	)

	if selected {
		return StyleSelected.Width(width).Render(line)
	}
	return line
}

func (s *Screen1) visibleRows() int {
	// Reserve lines for header, title, status bar
	rows := s.height - 8
	if rows < 5 {
		rows = 5
	}
	return rows
}

func sortColName(c SortColumn) string {
	switch c {
	case SortByService:
		return "name"
	case SortByAnomalies:
		return "anomalies"
	case SortByLatency:
		return "latency"
	case SortByErrorRate:
		return "error rate"
	default:
		return "risk score"
	}
}

func truncateName(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func formatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	if ms == 0 {
		return "—"
	}
	return fmt.Sprintf("%dms", ms)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// suppress unused import
var _ = math.Pi
