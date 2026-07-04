package ui

import (
	"errors"
	"testing"

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
