package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/linuxmatters/jivedrop/internal/encoder"
)

// ProgressUpdate represents a progress update from the encoder
type ProgressUpdate struct {
	SamplesProcessed int64
	TotalSamples     int64
	Err              error
}

// EncodingCompleteMsg signals that encoding has finished
type EncodingCompleteMsg struct {
	Err error
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
	complete bool
	err      error
}

// NewEncodeModel creates a new encoding model
func NewEncodeModel(enc *encoder.Encoder, outputMode string, outputBitrate int) *EncodeModel {
	sampleRate, channels, format := enc.GetInputInfo()

	// Disco ball gradient: indigo → white (cool shimmer effect)
	p := progress.New(
		progress.WithGradient(string(gradientIndigo), string(gradientWhite)),
		progress.WithWidth(40),
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
	}
}

// Init initializes the model and starts encoding
func (m *EncodeModel) Init() tea.Cmd {
	return tea.Batch(
		m.startEncoding(),
		m.waitForProgress(),
	)
}

// Update handles messages
func (m *EncodeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Allow Ctrl+C to quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case ProgressUpdate:
		m.samplesProcessed = msg.SamplesProcessed
		m.totalSamples = msg.TotalSamples
		m.lastUpdateTime = time.Now()

		if msg.Err != nil {
			m.err = msg.Err
			m.complete = true
			return m, tea.Quit
		}

		return m, m.waitForProgress()

	case EncodingCompleteMsg:
		m.complete = true
		m.err = msg.Err
		return m, tea.Quit

	case error:
		m.err = msg
		m.complete = true
		return m, tea.Quit
	}

	return m, nil
}

// View renders the UI
func (m *EncodeModel) View() string {
	if m.err != nil {
		return errorView(m.err)
	}

	if m.complete {
		return completeView(m)
	}

	return progressView(m)
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

// Error returns any error that occurred during encoding
func (m *EncodeModel) Error() error {
	return m.err
}
