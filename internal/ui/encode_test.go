package ui

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/linuxmatters/jive-encoder/internal/encoder"
)

// newTestModel builds a minimal EncodeModel for exercising Update in isolation.
// The encoder is allocated but never initialised: Cancel is a lone atomic store,
// so these cases never touch cgo and drive Update with synthesised messages.
func newTestModel(t *testing.T) *EncodeModel {
	t.Helper()
	enc, err := encoder.New(encoder.Config{InputPath: "in.flac", OutputPath: "out.mp3"})
	if err != nil {
		t.Fatalf("encoder.New: %v", err)
	}
	return &EncodeModel{encoder: enc, nonInteractive: true}
}

// TestEncodeModel_CompleteAfterCancel verifies that a Ctrl+C landing in the gap
// after Encode has already returned nil does not discard the finished encode.
// Classification keys off Encode's return, so a successful completion stays
// successful and Cancelled reports false, keeping the output MP3.
func TestEncodeModel_CompleteAfterCancel(t *testing.T) {
	m := newTestModel(t)

	// Ctrl+C arrives, setting the cancel flag, but Encode has already finished.
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !m.cancelled {
		t.Fatalf("Ctrl+C did not set cancelled flag")
	}

	// The successful completion message arrives after the late Ctrl+C.
	m.Update(EncodingCompleteMsg{Err: nil})

	if m.Cancelled() {
		t.Errorf("successful encode misclassified as cancelled; output would be discarded")
	}
	if m.Error() != nil {
		t.Errorf("successful encode reported error: %v", m.Error())
	}
	if !m.settling {
		t.Errorf("successful encode did not enter settle phase")
	}
}

// TestEncodeModel_GenuineCancel verifies that an Encode returning ErrCancelled
// is reported as cancelled and skips the settle phase.
func TestEncodeModel_GenuineCancel(t *testing.T) {
	m := newTestModel(t)

	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m.Update(EncodingCompleteMsg{Err: encoder.ErrCancelled})

	if !m.Cancelled() {
		t.Errorf("genuine cancel not reported as cancelled")
	}
	if m.settling {
		t.Errorf("genuine cancel should not settle")
	}
}

// TestEncodeModel_ErrorAfterCancel verifies that a real encoding error landing
// after a late Ctrl+C is preserved rather than swallowed. Both cancel and error
// abort the run and discard the output, so the cancel flag may stay set; the
// error must not be lost.
func TestEncodeModel_ErrorAfterCancel(t *testing.T) {
	m := newTestModel(t)

	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	wantErr := errors.New("write frame failed")
	m.Update(EncodingCompleteMsg{Err: wantErr})

	if !errors.Is(m.Error(), wantErr) {
		t.Errorf("real failure not surfaced: got %v, want %v", m.Error(), wantErr)
	}
	if m.settling {
		t.Errorf("error should not settle")
	}
}

// TestFrameTickLoopGating verifies the 60fps animation loop only runs when a
// renderer is present. In non-interactive (WithoutRenderer) mode Init must not
// arm the tick loop, and a successful completion must quit at once rather than
// schedule a settle tick; the interactive path still arms the loop.
func TestFrameTickLoopGating(t *testing.T) {
	t.Run("non-interactive Init schedules no tick", func(t *testing.T) {
		m := newTestModel(t) // nonInteractive: true
		m.Init()
		if m.anim.ticking {
			t.Errorf("non-interactive Init armed the frame-tick loop")
		}
	})

	t.Run("interactive Init arms the tick loop", func(t *testing.T) {
		m := newTestModel(t)
		m.nonInteractive = false
		m.Init()
		if !m.anim.ticking {
			t.Errorf("interactive Init did not arm the frame-tick loop")
		}
	})

	t.Run("non-interactive success quits without ticking", func(t *testing.T) {
		m := newTestModel(t)
		_, cmd := m.Update(EncodingCompleteMsg{Err: nil})
		if cmd == nil {
			t.Fatalf("successful completion returned no command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("non-interactive success did not quit; got a non-quit command")
		}
	})
}

// TestCalculateProgress covers the percentage helper, including the zero-total
// guard that stands in before FFmpeg reports a sample count.
func TestCalculateProgress(t *testing.T) {
	cases := []struct {
		name      string
		processed int64
		total     int64
		want      float64
	}{
		{"zero total", 10, 0, 0},
		{"start", 0, 100, 0},
		{"quarter", 1, 4, 25},
		{"half", 50, 100, 50},
		{"complete", 100, 100, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &EncodeModel{samplesProcessed: c.processed, totalSamples: c.total}
			if got := m.calculateProgress(); got != c.want {
				t.Errorf("calculateProgress() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestCalculateSpeed covers the realtime-multiple helper: the zero-rate guard
// and the audio-over-wall-clock ratio. The ratio case pins startTime into the
// past and allows a small tolerance for the live time.Since read.
func TestCalculateSpeed(t *testing.T) {
	t.Run("zero input rate", func(t *testing.T) {
		m := &EncodeModel{inputRate: 0, samplesProcessed: 44100}
		if got := m.calculateSpeed(); got != 0 {
			t.Errorf("calculateSpeed() = %v, want 0", got)
		}
	})

	t.Run("realtime multiple", func(t *testing.T) {
		// 100 audio seconds decoded in ~1 wall-clock second ≈ 100× realtime.
		m := &EncodeModel{
			inputRate:        44100,
			samplesProcessed: 44100 * 100,
			startTime:        time.Now().Add(-time.Second),
		}
		got := m.calculateSpeed()
		if math.Abs(got-100) > 1 {
			t.Errorf("calculateSpeed() = %v, want ≈100", got)
		}
	})
}

// TestCalculateTimeRemaining covers the ETA helper: the out-of-range guards and
// a mid-encode linear extrapolation, with tolerance for the live clock read.
func TestCalculateTimeRemaining(t *testing.T) {
	t.Run("no progress", func(t *testing.T) {
		m := &EncodeModel{samplesProcessed: 0, totalSamples: 0}
		if got := m.calculateTimeRemaining(); got != 0 {
			t.Errorf("calculateTimeRemaining() = %v, want 0", got)
		}
	})

	t.Run("complete", func(t *testing.T) {
		m := &EncodeModel{samplesProcessed: 100, totalSamples: 100}
		if got := m.calculateTimeRemaining(); got != 0 {
			t.Errorf("calculateTimeRemaining() = %v, want 0", got)
		}
	})

	t.Run("halfway", func(t *testing.T) {
		// 50% done after ~10s extrapolates to ~20s total, ~10s remaining.
		m := &EncodeModel{
			samplesProcessed: 50,
			totalSamples:     100,
			startTime:        time.Now().Add(-10 * time.Second),
		}
		got := m.calculateTimeRemaining()
		if math.Abs(got.Seconds()-10) > 1 {
			t.Errorf("calculateTimeRemaining() = %v, want ≈10s", got)
		}
	})
}

// TestFormatClock covers the media-player clock: MM:SS below an hour, H:MM:SS at
// or above one, and the clamp that keeps a negative duration at zero.
func TestFormatClock(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "00:00"},
		{"negative clamps", -5 * time.Second, "00:00"},
		{"seconds", 5 * time.Second, "00:05"},
		{"minutes", 65 * time.Second, "01:05"},
		{"exact hour", time.Hour, "1:00:00"},
		{"hours", time.Hour + 61*time.Second, "1:01:01"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatClock(c.d); got != c.want {
				t.Errorf("formatClock(%v) = %q, want %q", c.d, got, c.want)
			}
		})
	}
}

// TestFormatDurationHuman covers the "Xm Ys"/"Xs" formatter, including the
// second-only and whole-minute shapes.
func TestFormatDurationHuman(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"seconds", 5 * time.Second, "5s"},
		{"sub-minute", 59 * time.Second, "59s"},
		{"whole minute", time.Minute, "1m"},
		{"minute and seconds", 65 * time.Second, "1m 5s"},
		{"multi minute", 125 * time.Second, "2m 5s"},
		{"whole minutes", 2 * time.Minute, "2m"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatDurationHuman(c.d); got != c.want {
				t.Errorf("formatDurationHuman(%v) = %q, want %q", c.d, got, c.want)
			}
		})
	}
}

// TestMiniBar covers the segmented stats bar: rounding to the nearest cell and
// clamping of out-of-range fractions. Styling may wrap the glyphs in ANSI, so
// the assertions count the ▰/▱ runes rather than compare the whole string.
func TestMiniBar(t *testing.T) {
	const cells = 8
	cases := []struct {
		name     string
		fraction float64
		filled   int
	}{
		{"empty", 0, 0},
		{"full", 1, 8},
		{"half rounds down", 0.5, 4},
		{"rounds up", 0.5625, 5},
		{"overshoot clamps", 1.5, 8},
		{"negative clamps", -0.5, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := miniBar(c.fraction)
			gotFilled := strings.Count(out, "▰")
			gotEmpty := strings.Count(out, "▱")
			if gotFilled != c.filled {
				t.Errorf("miniBar(%v) filled = %d, want %d", c.fraction, gotFilled, c.filled)
			}
			if gotEmpty != cells-c.filled {
				t.Errorf("miniBar(%v) empty = %d, want %d", c.fraction, gotEmpty, cells-c.filled)
			}
		})
	}
}
