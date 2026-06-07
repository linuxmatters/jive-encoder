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

// clockStyle renders the elapsed/remaining MM:SS values in white, matching the
// unbolded weight of the media-player stats row.
var clockStyle = lipgloss.NewStyle().
	Foreground(cli.TextColor)

// frameWidth fixes the lipgloss style width (content area plus the box's 2-cell
// horizontal padding each side) shared by every framed view, so the progress,
// complete and error boxes match in outer width.
const frameWidth = 50

// frameChrome is the non-content width that lipgloss's Width(frameWidth) absorbs:
// the 2-cell rounded border plus boxStyle's Padding(1, 2) horizontal padding
// (2 cells each side). Subtract it from frameWidth to get the content area.
const frameChrome = 6

// frameContentWidth is the usable width inside the frame's border and padding.
const frameContentWidth = frameWidth - frameChrome

// percentField is the constant-width tail after the bar: a 2-cell gap plus a
// 4-cell "%3.0f%%" field ("100%"), kept fixed so the box never resizes across
// single/double/triple-digit percentages.
const percentField = 6

// progressBarWidth sizes the gradient bar so the bar + gap + percentage field
// fits inside the frame's content area with one column of slack, keeping the
// percentage inline on the bar's line instead of wrapping below it.
const progressBarWidth = frameContentWidth - percentField - 1

// frameStyle is the shared box used by every framed view at a fixed width.
var frameStyle = boxStyle.Width(frameWidth)

// boltStyle renders the speed lightning bolt in a warm gold against the cool
// palette, giving it an understated "energy" cue.
var boltStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFD166"))
