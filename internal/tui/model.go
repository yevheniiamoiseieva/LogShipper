package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"collector/internal/anomaly"
	"collector/internal/event"
	"collector/internal/graph"
)

const tickInterval = 500 * time.Millisecond

type Model struct {
	graph    *graph.CallGraph
	detector *anomaly.ZScoreDetector
	cancel   context.CancelFunc

	screen  Screen
	screen1 Screen1
	screen2 Screen2
	screen3 Screen3

	startTime      time.Time
	totalEvents    int64
	totalAnomalies int64
	anomalyCounts  map[string]int
	newAlerts      map[string]bool
	cycleEdges     map[string]bool
	anomalyEdges   map[string]bool

	width    int
	height   int
	showHelp bool
	spinner  spinner.Model

	lastSnapshot graph.CallGraphSnapshot
}

func New(g *graph.CallGraph, det *anomaly.ZScoreDetector, cancel context.CancelFunc) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		graph:         g,
		detector:      det,
		cancel:        cancel,
		screen:        ScreenServiceList,
		screen1:       NewScreen1(),
		screen2:       NewScreen2(),
		screen3:       NewScreen3(),
		startTime:     time.Now(),
		anomalyCounts: make(map[string]int),
		newAlerts:     make(map[string]bool),
		cycleEdges:    make(map[string]bool),
		anomalyEdges:  make(map[string]bool),
		spinner:       sp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tick(),
		m.spinner.Tick,
		listenGraphEvents(m.graph),
		listenAnomalyEvents(m.detector),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.screen1.SetSize(msg.Width, msg.Height)
		m.screen2.SetSize(msg.Width, msg.Height)
		m.screen3.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "r":
			snap := m.graph.Snapshot()
			m.applySnapshot(snap)
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		}

		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		switch m.screen {
		case ScreenServiceList:
			switch msg.String() {
			case "enter":
				svc := m.screen1.SelectedService()
				if svc != "" {
					m.screen2.SetService(svc)
					m.screen2.SetCycleEdges(m.cycleEdges)
					m.screen2.SetAnomalyEdges(m.anomalyEdges)
					m.screen2.Update(m.lastSnapshot)
					m.screen = ScreenDependency
				}
			default:
				cmds = append(cmds, m.screen1.HandleKey(msg))
			}

		case ScreenDependency:
			switch msg.String() {
			case "esc":
				m.screen = ScreenServiceList
			case "enter":
				edge := m.screen2.SelectedEdge()
				if edge != nil {
					m.screen3.SetEdge(
						edgeSrc(edge, m.screen2.service),
						edgeDst(edge, m.screen2.service),
						edge.Operation,
						edge.Key,
					)
					m.screen = ScreenEventLog
				}
			default:
				m.screen2.HandleKey(msg)
			}

		case ScreenEventLog:
			switch msg.String() {
			case "esc":
				m.screen = ScreenDependency
			default:
				m.screen3.HandleKey(msg)
			}
		}

	case TickMsg:
		snap := m.graph.Snapshot()
		m.applySnapshot(snap)
		m.screen1.ToggleBlink()
		cmds = append(cmds, tick())

	case AnomalyMsg:
		ev := msg.Event
		m.anomalyCounts[ev.EdgeKey]++
		m.totalAnomalies++
		parts := strings.SplitN(ev.EdgeKey, "|", 3)
		if len(parts) >= 1 {
			m.newAlerts[parts[0]] = true
		}
		m.anomalyEdges[ev.EdgeKey] = true
		cmds = append(cmds, listenAnomalyEvents(m.detector))

	case GraphEventMsg:
		gev := msg.Event
		m.totalEvents++
		if gev.Type == graph.GraphEventNewCycle {
			key := fmt.Sprintf("%s|%s|%s", gev.Edge.Src, gev.Edge.Dst, gev.Edge.Operation)
			m.cycleEdges[key] = true
		}
		cmds = append(cmds, listenGraphEvents(m.graph))

	case event.NormalizedEvent:
		m.screen3.AddEvent(&msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) applySnapshot(snap graph.CallGraphSnapshot) {
	m.lastSnapshot = snap
	m.screen1.Update(snap, m.anomalyCounts, m.newAlerts)
	if m.screen == ScreenDependency {
		m.screen2.Update(snap)
	}
	m.newAlerts = make(map[string]bool)
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "initializing..."
	}

	if m.showHelp {
		return m.helpView()
	}

	var content string
	switch m.screen {
	case ScreenServiceList:
		content = m.screen1.View(m.width)
	case ScreenDependency:
		content = m.screen2.View(m.serviceRisk(m.screen2.service))
	case ScreenEventLog:
		content = m.screen3.View()
	}

	statusBar := m.statusBar()
	keyBar := m.keyBar()

	reserved := lipgloss.Height(statusBar) + lipgloss.Height(keyBar) + 1
	contentHeight := m.height - reserved
	if contentHeight < 0 {
		contentHeight = 0
	}

	lines := strings.Split(content, "\n")
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	content = strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar, keyBar)
}

func (m Model) statusBar() string {
	if m.width == 0 {
		return ""
	}
	uptime := time.Since(m.startTime).Round(time.Second)
	h := int(uptime.Hours())
	mn := int(uptime.Minutes()) % 60
	sec := int(uptime.Seconds()) % 60

	parts := []string{
		StyleStatusKey.Render("Events:") + StyleStatusBar.Render(fmt.Sprintf(" %d", m.totalEvents)),
		StyleStatusKey.Render("Anomalies:") + StyleStatusBar.Render(fmt.Sprintf(" %d", m.totalAnomalies)),
		StyleStatusKey.Render("Services:") + StyleStatusBar.Render(fmt.Sprintf(" %d", len(m.lastSnapshot.Nodes))),
		StyleStatusKey.Render("Uptime:") + StyleStatusBar.Render(fmt.Sprintf(" %02d:%02d:%02d", h, mn, sec)),
		m.spinner.View(),
	}

	bar := strings.Join(parts, StyleDim.Render(" | "))
	return StyleStatusBar.Width(m.width).Render(bar)
}

func (m Model) keyBar() string {
	if m.width == 0 {
		return ""
	}
	var keys string
	switch m.screen {
	case ScreenServiceList:
		keys = "↑↓ navigate  enter select  / filter  s sort  r refresh  ? help  q quit"
	case ScreenDependency:
		keys = "↑↓ navigate  enter events  esc back  ? help  q quit"
	case ScreenEventLog:
		keys = "↑↓ navigate  enter details  a autoscroll  esc back  q quit"
	}
	return StyleDim.Width(m.width).Render(keys)
}

func (m Model) helpView() string {
	if m.width == 0 {
		return ""
	}
	content := StyleBold.Render("Keyboard shortcuts") + "\n\n" +
		StyleBold.Render("Global\n") +
		"  q / Ctrl+C    quit\n" +
		"  r             force refresh\n" +
		"  ?             toggle help\n\n" +
		StyleBold.Render("Screen 1 — Service List\n") +
		"  ↑ / k         up\n" +
		"  ↓ / j         down\n" +
		"  enter         open service detail\n" +
		"  /             filter by name\n" +
		"  s             cycle sort column\n\n" +
		StyleBold.Render("Screen 2 — Dependency View\n") +
		"  ↑ / k         up\n" +
		"  ↓ / j         down\n" +
		"  enter         open event log\n" +
		"  esc           back to service list\n\n" +
		StyleBold.Render("Screen 3 — Event Log\n") +
		"  ↑ / k         up\n" +
		"  ↓ / j         down\n" +
		"  enter         show raw JSON\n" +
		"  a             toggle autoscroll\n" +
		"  esc           back to dependency view\n\n" +
		StyleBold.Render("Legend\n") +
		"  ⚠             anomaly detected\n" +
		"  ↺             part of a dependency cycle\n" +
		"  ●             new unseen alert (blinks)\n\n" +
		StyleDim.Render("press any key to close")

	popupWidth := m.width - 4
	if popupWidth > 60 {
		popupWidth = 60
	}
	popup := StyleHelp.Width(popupWidth).Render(content)

	leftPad := (m.width - lipgloss.Width(popup)) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	topPad := (m.height - lipgloss.Height(popup)) / 2
	if topPad < 0 {
		topPad = 0
	}

	var b strings.Builder
	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}
	b.WriteString(strings.Repeat(" ", leftPad) + popup)
	return b.String()
}

func (m Model) serviceRisk(svc string) float64 {
	for _, row := range m.screen1.rows {
		if row.Name == svc {
			return row.RiskScore
		}
	}
	return 0
}

func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func listenGraphEvents(g *graph.CallGraph) tea.Cmd {
	return func() tea.Msg {
		ev := <-g.Events()
		return GraphEventMsg{Event: ev}
	}
}

func listenAnomalyEvents(det *anomaly.ZScoreDetector) tea.Cmd {
	if det == nil {
		return nil
	}
	return func() tea.Msg {
		ev := <-det.Events()
		return AnomalyMsg{Event: ev}
	}
}

func edgeSrc(edge *EdgeRow, focusedService string) string {
	if edge.Direction == "upstream" {
		return edge.Peer
	}
	return focusedService
}

func edgeDst(edge *EdgeRow, focusedService string) string {
	if edge.Direction == "downstream" {
		return edge.Peer
	}
	return focusedService
}
