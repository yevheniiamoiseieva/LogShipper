package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"collector/internal/event"
)

const maxEvents = 100

// EventEntry wraps a NormalizedEvent for display.
type EventEntry struct {
	Event      *event.NormalizedEvent
	ReceivedAt time.Time
}

// Screen3 shows the event log for a selected edge.
type Screen3 struct {
	edgeKey    string
	srcService string
	dstService string
	operation  string

	events     []EventEntry
	cursor     int
	autoScroll bool
	showPopup  bool
	popupJSON  string

	width  int
	height int
}

func NewScreen3() Screen3 {
	return Screen3{autoScroll: true}
}

func (s *Screen3) SetSize(w, h int) {
	s.width = w
	s.height = h
}

func (s *Screen3) SetEdge(src, dst, op, key string) {
	s.srcService = src
	s.dstService = dst
	s.operation = op
	s.edgeKey = key
	s.events = nil
	s.cursor = 0
	s.showPopup = false
}

// AddEvent appends a new event if it matches the current edge.
func (s *Screen3) AddEvent(ev *event.NormalizedEvent) {
	if ev == nil {
		return
	}
	key := fmt.Sprintf("%s|%s|%s", ev.SrcService, ev.DstService, ev.Operation)
	if key != s.edgeKey {
		return
	}

	s.events = append(s.events, EventEntry{Event: ev, ReceivedAt: time.Now()})
	if len(s.events) > maxEvents {
		s.events = s.events[len(s.events)-maxEvents:]
	}
	if s.autoScroll {
		s.cursor = len(s.events) - 1
	}
}

func (s *Screen3) HandleKey(msg tea.KeyMsg) {
	if s.showPopup {
		s.showPopup = false
		return
	}
	switch msg.String() {
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
			s.autoScroll = false
		}
	case "down", "j":
		if s.cursor < len(s.events)-1 {
			s.cursor++
		}
	case "a":
		s.autoScroll = !s.autoScroll
		if s.autoScroll && len(s.events) > 0 {
			s.cursor = len(s.events) - 1
		}
	case "enter":
		if len(s.events) > 0 && s.cursor < len(s.events) {
			s.showPopup = true
			s.popupJSON = formatRawJSON(s.events[s.cursor].Event)
		}
	}
}

func (s *Screen3) View() string {
	var b strings.Builder

	// Title
	op := s.operation
	if op == "" {
		op = "all operations"
	}
	title := StyleTitle.Render(fmt.Sprintf("⬡ Edge: %s → %s  |  op: %s",
		s.srcService, s.dstService, op))
	b.WriteString(title + "\n")

	autoStr := StyleDim.Render("autoscroll: ")
	if s.autoScroll {
		autoStr += StyleOK.Render("ON [a]")
	} else {
		autoStr += StyleWarn.Render("OFF [a]")
	}
	b.WriteString(autoStr + "\n\n")

	// Column headers
	b.WriteString(StyleDim.Render(fmt.Sprintf("  %-24s %10s %8s %-18s %s",
		"TIMESTAMP", "LATENCY", "STATUS", "TRACE ID", "SOURCE")) + "\n")

	// Events list
	visible := s.visibleRows()
	start := max(0, s.cursor-visible+1)
	if s.autoScroll {
		start = max(0, len(s.events)-visible)
	}
	end := min(len(s.events), start+visible)

	if len(s.events) == 0 {
		b.WriteString(StyleDim.Render("  — waiting for events —") + "\n")
	}

	for i := start; i < end; i++ {
		entry := s.events[i]
		line := s.renderEventRow(entry, i == s.cursor)
		b.WriteString(line + "\n")
	}

	// Popup overlay
	if s.showPopup {
		popup := StylePopup.Render(
			StyleBold.Render("Raw Event JSON") + "\n\n" +
				s.popupJSON + "\n\n" +
				StyleDim.Render("press any key to close"),
		)
		b.WriteString("\n" + popup)
	}

	return b.String()
}

func (s *Screen3) renderEventRow(entry EventEntry, selected bool) string {
	ev := entry.Event
	ts := ev.Timestamp.Format("2006-01-02 15:04:05.000")

	latency := formatDuration(ev.Latency)
	statusStr := fmt.Sprintf("%d", ev.StatusCode)
	if ev.StatusCode == 0 {
		statusStr = "—"
	}

	traceID := ev.TraceID
	if traceID == "" {
		traceID = "—"
	}
	if len(traceID) > 16 {
		traceID = traceID[:16]
	}

	line := fmt.Sprintf("  %-24s %10s %8s %-18s %s",
		ts, latency, statusStr, traceID, ev.SourceName)

	style := StatusStyle(ev.StatusCode)
	line = style.Render(line)

	if selected && !s.showPopup {
		line = StyleSelected.Width(s.width).Render(line)
	}
	return line
}

func (s *Screen3) visibleRows() int {
	rows := s.height - 10
	if rows < 5 {
		rows = 5
	}
	return rows
}

func formatRawJSON(ev *event.NormalizedEvent) string {
	if ev == nil {
		return "{}"
	}
	// Build a display map
	m := map[string]any{
		"timestamp":   ev.Timestamp.Format(time.RFC3339Nano),
		"src_service": ev.SrcService,
		"dst_service": ev.DstService,
		"operation":   ev.Operation,
		"status_code": ev.StatusCode,
		"latency_ms":  ev.Latency.Milliseconds(),
		"trace_id":    ev.TraceID,
		"span_id":     ev.SpanID,
		"level":       ev.Level,
		"source_name": ev.SourceName,
		"raw":         ev.Raw,
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "{}"
	}
	result := string(b)
	// Truncate for popup display
	lines := strings.Split(result, "\n")
	if len(lines) > 30 {
		lines = append(lines[:30], "  ... (truncated)")
	}
	return strings.Join(lines, "\n")
}
