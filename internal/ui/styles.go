package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/linuxmatters/jivedrop/internal/cli"
)

// Import shared colour palette from cli package
var (
	primaryColor = cli.PrimaryColor
	accentColor  = cli.AccentColor
	mutedColor   = cli.MutedColor

	// Disco ball gradient colours
	gradientIndigo = cli.GradientIndigo
	gradientWhite  = cli.GradientWhite
)

// Header style for section titles
var headerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(primaryColor).
	MarginBottom(1)

// Accent style for highlighted values
var accentStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(accentColor)

// Spinner style for the encoding progress indicator
var spinnerStyle = lipgloss.NewStyle().
	Foreground(accentColor)

// Shared styles from cli package
var (
	successStyle   = cli.SuccessStyle
	errorStyle     = cli.ErrorStyle
	highlightStyle = cli.HighlightStyle
	keyStyle       = cli.KeyStyle
	valueStyle     = cli.ValueStyle
	boxStyle       = cli.BoxStyle
)

// Muted text style (no cli equivalent)
var mutedStyle = lipgloss.NewStyle().
	Foreground(mutedColor).
	Italic(true)
