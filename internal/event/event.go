// Package event defines UI event types shared between app, console, and tui packages.
package event

// UIEventKind classifies a UI event.
type UIEventKind int

const (
	UIEventOutput UIEventKind = iota
	UIEventStatus
	UIEventModal
	UIEventPanel
)

// UIPanelKind identifies a modal panel type.
type UIPanelKind int

const (
	UIPanelNone UIPanelKind = iota
	UIPanelForward
	UIPanelPlugin
	UIPanelMode
)

// UIEvent is emitted by the app core and consumed by TUI or console frontends.
type UIEvent struct {
	Kind  UIEventKind
	Title string
	Text  string
	Panel UIPanelKind
}
