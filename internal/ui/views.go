package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/linuxmatters/jivedrop/internal/encoder"
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
		return boxStyle.Render(b.String())
	}

	fmt.Fprintf(&b, "%s %s", spinnerGlyph, headerStyle.Render("Encoding to MP3..."))
	b.WriteString("\n\n")

	// Clamp the displayed fraction to [0,1] so an under-damped spring
	// overshoot never renders >100%. The spring's internal state keeps
	// its overshoot to settle correctly.
	display := max(0, min(1, m.anim.springPos))
	b.WriteString(m.progressBar.ViewAs(display))
	fmt.Fprintf(&b, "  %s", highlightStyle.Render(fmt.Sprintf("%3.0f%%", display*100)))
	b.WriteString("\n\n")

	elapsed := formatDurationHuman(m.lastUpdateTime.Sub(m.startTime))
	remaining := formatDurationHuman(m.calculateTimeRemaining())
	// During settle the bar is at 100% but the loop keeps ticking; use the frozen
	// speed so the figure does not drift as wall-clock time grows.
	speed := m.calculateSpeed()
	if m.settling {
		speed = m.anim.finalSpeed
	}

	stats := lipgloss.JoinHorizontal(lipgloss.Top,
		keyStyle.Render("Elapsed:"),
		" ",
		highlightStyle.Render(elapsed),
		"   ",
		mutedStyle.Render("~Remaining:"),
		" ",
		mutedStyle.Render(remaining),
		"   ",
		accentStyle.Render(fmt.Sprintf("%.1fx realtime", speed)),
	)

	b.WriteString(stats)
	b.WriteString("\n\n")

	inputSpec := fmt.Sprintf("%s %.1fkHz %s",
		m.inputFormat,
		float64(m.inputRate)/1000.0,
		encoder.FormatChannelMode(m.inputChannels),
	)

	outputSpec := fmt.Sprintf("MP3 %.1fkHz %s CBR %dkbps",
		44.1,
		m.outputMode,
		m.outputBitrate,
	)

	specBlock := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render("Input:"),
			"  ",
			valueStyle.Render(inputSpec),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render("Output:"),
			"  ",
			valueStyle.Render(outputSpec),
		),
	)

	b.WriteString(specBlock)
	b.WriteString("\n")

	return boxStyle.Render(b.String())
}

// completeView renders the completion message
func completeView(m *EncodeModel) string {
	elapsed := formatDurationHuman(m.lastUpdateTime.Sub(m.startTime))
	speed := m.anim.finalSpeed

	msg := fmt.Sprintf("%s Encoded in %s (%.1fx realtime)",
		successStyle.Render("✓"),
		valueStyle.Render(elapsed),
		speed,
	)

	return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, msg))
}

// errorView renders an error message
func errorView(err error) string {
	msg := fmt.Sprintf("%s %s",
		errorStyle.Render("Error:"),
		err.Error(),
	)

	return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, msg))
}
