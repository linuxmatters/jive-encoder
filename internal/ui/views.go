package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/linuxmatters/jive-encoder/internal/encoder"
)

// progressView renders the encoding progress UI
func progressView(m *EncodeModel) string {
	var b strings.Builder

	// Persistent spinner accent rendered off the shared tick (see encode.go).
	spinnerGlyph := spinnerStyle.Render(spinnerFrames[m.anim.spinnerFrame%len(spinnerFrames)])

	// Before the first non-zero progress, FFmpeg is still initialising, so the
	// spinner stands in as an indeterminate indicator with a "preparing" cue.
	if m.totalSamples == 0 {
		fmt.Fprintf(&b, "%s %s",
			spinnerGlyph,
			headerStyle.Render("Preparing to encode..."),
		)
		return frameStyle.Render(b.String())
	}

	fmt.Fprintf(&b, "%s %s", spinnerGlyph, headerStyle.Render(fmt.Sprintf("Encoding to %s...", m.outputFormat)))
	b.WriteString("\n")

	// Clamp the displayed fraction to [0,1] so an under-damped spring
	// overshoot never renders >100%. The spring's internal state keeps
	// its overshoot to settle correctly.
	display := max(0, min(1, m.anim.springPos))
	b.WriteString(m.progressBar.ViewAs(display))
	fmt.Fprintf(&b, "  %s", highlightStyle.Render(fmt.Sprintf("%3.0f%%", display*100)))
	b.WriteString("\n\n")

	elapsed := formatClock(m.lastUpdateTime.Sub(m.startTime))
	remaining := formatClock(m.calculateTimeRemaining())
	// During settle the bar is at 100% but the loop keeps ticking; use the frozen
	// speed so the figure does not drift as wall-clock time grows.
	speed := m.calculateSpeed()
	if m.settling {
		speed = m.anim.finalSpeed
	}

	stats := lipgloss.JoinHorizontal(lipgloss.Center,
		clockStyle.Render(elapsed),
		"  ",
		miniBar(display),
		"  ",
		clockStyle.Render(remaining),
		"   ",
		mutedStyle.Render("·"),
		"   ",
		boltStyle.Render("⚡"),
		" ",
		highlightStyle.Render(fmt.Sprintf("%.1f×", speed)),
	)

	b.WriteString(stats)
	b.WriteString("\n\n")

	inputSpec := fmt.Sprintf("%s %.1f㎑ %s",
		m.inputFormat,
		float64(m.inputRate)/1000.0,
		encoder.FormatChannelMode(m.inputChannels),
	)

	rateMode := "CBR"
	if m.outputVBR {
		rateMode = "VBR"
	}
	outputSpec := fmt.Sprintf("%s %.1f㎑ %s %s %dkbps",
		m.outputFormat,
		float64(m.outputRate)/1000.0,
		m.outputMode,
		rateMode,
		m.outputBitrate,
	)

	// Fix the label cell to the longer label ("Output:") so both value columns
	// start at the same offset.
	labelStyle := keyStyle.Width(lipgloss.Width("Output:"))

	specBlock := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			labelStyle.Render("Input:"),
			"  ",
			valueStyle.Render(inputSpec),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			labelStyle.Render("Output:"),
			"  ",
			valueStyle.Render(outputSpec),
		),
	)

	b.WriteString(specBlock)

	return frameStyle.Render(b.String())
}

// miniBar renders a short segmented progress bar for the media-player stats row.
// It mirrors the main bar's clamped fraction but uses alignment-safe
// single-width glyphs (▰ filled, ▱ empty) so the row never shifts.
func miniBar(fraction float64) string {
	const cells = 8
	filled := int(fraction*cells + 0.5)
	filled = max(0, min(cells, filled))

	bar := strings.Repeat("▰", filled) + strings.Repeat("▱", cells-filled)
	return mutedStyle.Render(bar)
}

// completeView renders the completion message
func completeView(m *EncodeModel) string {
	elapsed := formatDurationHuman(m.lastUpdateTime.Sub(m.startTime))
	speed := m.anim.finalSpeed

	msg := fmt.Sprintf("%s Encoded in %s (%s %s)",
		successStyle.Render("✓"),
		valueStyle.Render(elapsed),
		boltStyle.Render("⚡"),
		highlightStyle.Render(fmt.Sprintf("%.1f×", speed)),
	)

	return frameStyle.Render(lipgloss.JoinVertical(lipgloss.Left, msg))
}

// errorView renders an error message
func errorView(err error) string {
	msg := fmt.Sprintf("%s %s",
		errorStyle.Render("Error:"),
		err.Error(),
	)

	return frameStyle.Render(lipgloss.JoinVertical(lipgloss.Left, msg))
}
