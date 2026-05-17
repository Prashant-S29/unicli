package ui

import "github.com/charmbracelet/lipgloss"

// Palette
var (
	ColorSuccess = lipgloss.Color("#4ADE80") // green
	ColorError   = lipgloss.Color("#F87171") // red
	ColorWarning = lipgloss.Color("#FBBF24") // amber
	ColorInfo    = lipgloss.Color("#60A5FA") // blue
	ColorMuted   = lipgloss.Color("#6B7280") // gray
	ColorBold    = lipgloss.Color("#F9FAFB") // near white
)

// Symbols
const (
	SymbolSuccess = "✓"
	SymbolError   = "✗"
	SymbolWarning = "!"
	SymbolInfo    = "→"
	SymbolDot     = "•"
)

// Base styles
var (
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	StyleInfo    = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	StyleMuted   = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleBold    = lipgloss.NewStyle().Foreground(ColorBold).Bold(true)

	StyleLabel = lipgloss.NewStyle().Foreground(ColorMuted).Width(10)
)
