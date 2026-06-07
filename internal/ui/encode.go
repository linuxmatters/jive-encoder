package ui

import (
	"errors"
	"fmt"
	"math"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/harmonica"
	"github.com/linuxmatters/jivedrop/internal/encoder"
)

// ProgressUpdate represents a progress update from the encoder
type ProgressUpdate struct {
	SamplesProcessed int64
	TotalSamples     int64
}

// EncodingCompleteMsg signals that encoding has finished
type EncodingCompleteMsg struct {
	Err error
}

// frameTickMsg drives the animation clock at a fixed frame rate.
type frameTickMsg struct{}

// spinnerFrames are the shimmer glyphs advanced off the shared tick. The
// renderer (progressView) reads this slice via m.anim.spinnerFrame.
var spinnerFrames = []string{"·", "✦", "✧", "✶"}

// spinnerTicksPerFrame throttles the 60fps shared tick down to ~8fps spinner
// cadence (60 / 8 ≈ 7.5fps).
const spinnerTicksPerFrame = 8

// settleEpsilon is the convergence threshold for the completion settle: once the
// spring is within this distance of 1.0 with near-zero velocity, the program quits.
const settleEpsilon = 0.001

// settleCap bounds the completion settle so a non-converging spring cannot hang.
const settleCap = 500 * time.Millisecond

// animState groups the animation fields, keeping EncodeModel legible.
type animState struct {
	spring       harmonica.Spring
	springPos    float64
	springVel    float64
	spinnerFrame int
	tickCount    int
	settleStart  time.Time
	// finalSpeed freezes the "Nx realtime" figure captured the instant encoding
	// completes, so progressView renders a stable value through the settle phase.
	finalSpeed float64
}

// EncodeModel is the Bubbletea model for encoding progress
type EncodeModel struct {
	// Progress bar component
	progressBar progress.Model

	// Encoder state
	encoder      *encoder.Encoder
	progressChan chan ProgressUpdate

	// Progress tracking
	samplesProcessed int64
	totalSamples     int64
	startTime        time.Time
	lastUpdateTime   time.Time

	// Audio specs for display
	inputFormat   string
	inputRate     int
	inputChannels int
	outputMode    string // "mono" or "stereo"
	outputBitrate int

	// Completion state
	complete  bool
	settling  bool
	cancelled bool
	err       error

	// nonInteractive suppresses the rendered view under WithoutRenderer mode.
	nonInteractive bool

	// Animation state
	anim animState
}

// NewEncodeModel creates a new encoding model
func NewEncodeModel(enc *encoder.Encoder, outputMode string, outputBitrate int, nonInteractive bool) *EncodeModel {
	sampleRate, channels, format := enc.GetInputInfo()

	// Disco ball gradient: indigo → white (cool shimmer effect)
	p := progress.New(
		progress.WithColors(gradientIndigo, gradientWhite),
		progress.WithWidth(progressBarWidth),
		progress.WithoutPercentage(),
	)

	return &EncodeModel{
		progressBar:    p,
		encoder:        enc,
		progressChan:   make(chan ProgressUpdate, 10),
		startTime:      time.Now(),
		lastUpdateTime: time.Now(),
		inputFormat:    format,
		inputRate:      sampleRate,
		inputChannels:  channels,
		outputMode:     outputMode,
		outputBitrate:  outputBitrate,
		nonInteractive: nonInteractive,
		anim: animState{
			spring: harmonica.NewSpring(harmonica.FPS(60), 6.0, 0.7),
		},
	}
}

// Init initializes the model and starts encoding
func (m *EncodeModel) Init() tea.Cmd {
	return tea.Batch(
		m.startEncoding(),
		m.waitForProgress(),
		m.tickFrame(),
	)
}

// Update handles messages
func (m *EncodeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Ctrl+C must cancel the encode and await its return, never quit
		// underneath the live cgo call. Cancel signals the encoder; the program
		// quits only once Encode returns via EncodingCompleteMsg. A second
		// Ctrl+C is a no-op: the encode is already unwinding.
		if msg.String() == "ctrl+c" && !m.cancelled {
			m.cancelled = true
			m.encoder.Cancel()
		}

	case ProgressUpdate:
		m.samplesProcessed = msg.SamplesProcessed
		m.totalSamples = msg.TotalSamples
		m.lastUpdateTime = time.Now()

		return m, m.waitForProgress()

	case frameTickMsg:
		target := m.calculateProgress() / 100
		if m.settling {
			target = 1.0
		}
		m.anim.springPos, m.anim.springVel = m.anim.spring.Update(m.anim.springPos, m.anim.springVel, target)

		// Advance the spinner frame ourselves off this shared tick, throttled to
		// ~8fps. The spinner's own Tick loop is never started.
		m.anim.tickCount++
		if m.anim.tickCount >= spinnerTicksPerFrame {
			m.anim.tickCount = 0
			m.anim.spinnerFrame = (m.anim.spinnerFrame + 1) % len(spinnerFrames)
		}

		if m.settling {
			converged := math.Abs(m.anim.springPos-1) < settleEpsilon && math.Abs(m.anim.springVel) < settleEpsilon
			if converged || time.Since(m.anim.settleStart) > settleCap {
				m.settling = false
				return m, tea.Quit
			}
			return m, m.tickFrame()
		}
		if !m.complete {
			return m, m.tickFrame()
		}
		return m, nil

	case EncodingCompleteMsg:
		if errors.Is(msg.Err, encoder.ErrCancelled) {
			// Cancelled: Encode observed the cancel and unwound, so the deferred
			// Close is now safe. Skip the settle and quit promptly. err stays nil;
			// cancellation is reported via Cancelled(). Classify off Encode's
			// return, not m.cancelled: a Ctrl+C landing after Encode already
			// returned must not discard a finished encode.
			m.cancelled = true
			m.complete = true
			return m, tea.Quit
		}
		if msg.Err != nil {
			m.complete = true
			m.err = msg.Err
			return m, tea.Quit
		}
		// Success: Encode finished before any cancel took effect. Clear a late
		// Ctrl+C flag so the completed MP3 is kept, then settle the spring to 100%
		// before quitting, keeping the bar visible via the settling → progressView
		// route.
		m.cancelled = false
		m.complete = true
		m.settling = true
		m.anim.settleStart = time.Now()
		m.anim.finalSpeed = m.calculateSpeed()
		return m, m.tickFrame()
	}

	return m, nil
}

// View renders the UI
func (m *EncodeModel) View() tea.View {
	if m.nonInteractive {
		// WithoutRenderer mode emits no frames; return an empty view so no
		// partial output reaches the pipe.
		return tea.NewView("")
	}

	if m.err != nil {
		return tea.NewView(errorView(m.err))
	}

	if m.cancelled {
		// Keep the last progress frame on screen rather than the success view.
		return tea.NewView(progressView(m))
	}

	if m.settling {
		return tea.NewView(progressView(m))
	}

	if m.complete {
		return tea.NewView(completeView(m))
	}

	return tea.NewView(progressView(m))
}

// startEncoding starts the encoding process in a goroutine
func (m *EncodeModel) startEncoding() tea.Cmd {
	return func() tea.Msg {
		err := m.encoder.Encode(func(samplesProcessed, totalSamples int64) {
			select {
			case m.progressChan <- ProgressUpdate{
				SamplesProcessed: samplesProcessed,
				TotalSamples:     totalSamples,
			}:
			default:
				// Drop updates when the buffer is full; the UI only needs the latest.
			}
		})

		// Closing the channel signals completion to waitForProgress.
		close(m.progressChan)

		return EncodingCompleteMsg{Err: err}
	}
}

// waitForProgress waits for the next progress update
func (m *EncodeModel) waitForProgress() tea.Cmd {
	return func() tea.Msg {
		update, ok := <-m.progressChan
		if !ok {
			// Channel closed, encoding complete
			return nil
		}
		return update
	}
}

// tickFrame schedules the next animation frame at 60fps.
func (m *EncodeModel) tickFrame() tea.Cmd {
	return tea.Tick(time.Second/60, func(time.Time) tea.Msg {
		return frameTickMsg{}
	})
}

// calculateProgress returns progress percentage (0-100)
func (m *EncodeModel) calculateProgress() float64 {
	if m.totalSamples == 0 {
		return 0
	}
	return float64(m.samplesProcessed) / float64(m.totalSamples) * 100
}

// calculateSpeed returns encoding speed (e.g., "101.2x realtime")
func (m *EncodeModel) calculateSpeed() float64 {
	if m.inputRate == 0 {
		return 0
	}

	elapsed := time.Since(m.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}

	// Calculate audio duration processed (in seconds)
	audioProcessed := float64(m.samplesProcessed) / float64(m.inputRate)

	// Speed = audio duration / wall clock time
	return audioProcessed / elapsed
}

// calculateTimeRemaining returns estimated time remaining
func (m *EncodeModel) calculateTimeRemaining() time.Duration {
	progress := m.calculateProgress()
	if progress <= 0 || progress >= 100 {
		return 0
	}

	elapsed := time.Since(m.startTime)

	// Use progress percentage for accurate estimation
	// If we've completed X%, the remaining (100-X)% will take proportionally longer
	totalEstimated := float64(elapsed) * 100.0 / progress
	remaining := time.Duration(totalEstimated) - elapsed

	return remaining
}

// formatDurationHuman formats a duration as "Xm Ys" or "Xs"
func formatDurationHuman(d time.Duration) string {
	d = d.Round(time.Second)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	m := int(d.Minutes())
	s := int(d.Seconds()) % 60

	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}

	return fmt.Sprintf("%dm %ds", m, s)
}

// formatClock formats a duration as a media-player clock: "MM:SS", or
// "H:MM:SS" once the duration reaches an hour.
func formatClock(d time.Duration) string {
	d = max(d.Round(time.Second), 0)

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}

	return fmt.Sprintf("%02d:%02d", m, s)
}

// Error returns any error that occurred during encoding
func (m *EncodeModel) Error() error {
	return m.err
}

// Cancelled reports whether the user interrupted the encode with Ctrl+C.
func (m *EncodeModel) Cancelled() bool {
	return m.cancelled
}
