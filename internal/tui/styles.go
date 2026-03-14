package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Base colors
	colorGreen  = lipgloss.Color("#22c55e")
	colorYellow = lipgloss.Color("#eab308")
	colorRed    = lipgloss.Color("#ef4444")
	colorBlue   = lipgloss.Color("#3b82f6")
	colorCyan   = lipgloss.Color("#06b6d4")
	colorGray   = lipgloss.Color("#6b7280")
	colorDim    = lipgloss.Color("#374151")
	colorWhite  = lipgloss.Color("#f9fafb")
	colorBg     = lipgloss.Color("#0f172a")
	colorPanel  = lipgloss.Color("#1e293b")
	colorBorder = lipgloss.Color("#334155")

	// Risk score colors
	StyleRiskLow  = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	StyleRiskMid  = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	StyleRiskHigh = lipgloss.NewStyle().Foreground(colorRed).Bold(true)

	// Status colors
	StyleOK    = lipgloss.NewStyle().Foreground(colorGreen)
	StyleWarn  = lipgloss.NewStyle().Foreground(colorYellow)
	StyleError = lipgloss.NewStyle().Foreground(colorRed)

	// Layout
	StyleTitle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true).
			Padding(0, 1)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(colorGray).
			Italic(true)

	StyleHeader = lipgloss.NewStyle().
			Background(colorPanel).
			Foreground(colorWhite).
			Bold(true).
			Padding(0, 1).
			Width(0)

	StyleSelected = lipgloss.NewStyle().
			Background(colorBlue).
			Foreground(colorWhite).
			Bold(true)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	StyleStatusBar = lipgloss.NewStyle().
			Background(colorPanel).
			Foreground(colorGray).
			Padding(0, 1)

	StyleStatusKey = lipgloss.NewStyle().
			Background(colorPanel).
			Foreground(colorCyan).
			Bold(true).
			Padding(0, 1)

	StyleHelp = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBlue).
			Background(colorPanel).
			Foreground(colorWhite).
			Padding(1, 2)

	StyleAnomalyBadge = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true)

	StyleCycleBadge = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	StylePopup = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Background(colorPanel).
			Foreground(colorWhite).
			Padding(1, 2).
			Width(80)

	StyleDim  = lipgloss.NewStyle().Foreground(colorGray)
	StyleBold = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
)

// RiskStyle returns the appropriate style for a risk score value.
func RiskStyle(score float64) lipgloss.Style {
	switch {
	case score > 7:
		return StyleRiskHigh
	case score >= 3:
		return StyleRiskMid
	default:
		return StyleRiskLow
	}
}

// StatusStyle returns style based on HTTP status code.
func StatusStyle(code int) lipgloss.Style {
	switch {
	case code >= 500:
		return StyleError
	case code >= 400:
		return StyleWarn
	default:
		return StyleOK
	}
}
