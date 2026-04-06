package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings used across screens.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Esc        key.Binding
	Quit       key.Binding
	Refresh    key.Binding
	Filter     key.Binding
	Sort       key.Binding
	AutoScroll key.Binding
	Help       key.Binding
}

var Keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Sort: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "sort"),
	),
	AutoScroll: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "autoscroll"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Esc, k.Quit, k.Help}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Esc},
		{k.Filter, k.Sort, k.Refresh, k.AutoScroll},
		{k.Quit, k.Help},
	}
}
