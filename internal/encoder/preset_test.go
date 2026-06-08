package encoder

import (
	"testing"

	"github.com/linuxmatters/ffmpeg-statigo"
)

func TestFormatPreset(t *testing.T) {
	tests := []struct {
		name          string
		codecID       ffmpeg.AVCodecID
		monoBitrate   int
		stereoBitrate int
		sampleFmt     ffmpeg.AVSampleFormat
		extension     string
		lowpassHz     int
		coverCapable  bool
	}{
		{
			name:          "mp3",
			codecID:       ffmpeg.AVCodecIdMp3,
			monoBitrate:   112000,
			stereoBitrate: 192000,
			sampleFmt:     ffmpeg.AVSampleFmtS16P,
			extension:     ".mp3",
			lowpassHz:     20500,
			coverCapable:  true,
		},
		{
			name:          "aac",
			codecID:       ffmpeg.AVCodecIdAac,
			monoBitrate:   64000,
			stereoBitrate: 128000,
			sampleFmt:     ffmpeg.AVSampleFmtFltp,
			extension:     ".m4a",
			lowpassHz:     0,
			coverCapable:  true,
		},
		{
			name:          "opus",
			codecID:       ffmpeg.AVCodecIdOpus,
			monoBitrate:   32000,
			stereoBitrate: 48000,
			sampleFmt:     ffmpeg.AVSampleFmtFlt,
			extension:     ".opus",
			lowpassHz:     0,
			coverCapable:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preset, ok := presetFor(tt.name)
			if !ok {
				t.Fatalf("presetFor(%q) returned not-found", tt.name)
			}
			if preset.codecID != tt.codecID {
				t.Errorf("codecID = %v, want %v", preset.codecID, tt.codecID)
			}
			if preset.monoBitrate != tt.monoBitrate {
				t.Errorf("monoBitrate = %d, want %d", preset.monoBitrate, tt.monoBitrate)
			}
			if preset.stereoBitrate != tt.stereoBitrate {
				t.Errorf("stereoBitrate = %d, want %d", preset.stereoBitrate, tt.stereoBitrate)
			}
			if preset.sampleFmt != tt.sampleFmt {
				t.Errorf("sampleFmt = %v, want %v", preset.sampleFmt, tt.sampleFmt)
			}
			if preset.extension != tt.extension {
				t.Errorf("extension = %q, want %q", preset.extension, tt.extension)
			}
			if preset.lowpassHz != tt.lowpassHz {
				t.Errorf("lowpassHz = %d, want %d", preset.lowpassHz, tt.lowpassHz)
			}
			if preset.coverCapable != tt.coverCapable {
				t.Errorf("coverCapable = %v, want %v", preset.coverCapable, tt.coverCapable)
			}
		})
	}

	if _, ok := presetFor("flac"); ok {
		t.Error("presetFor(\"flac\") returned found, want not-found")
	}
}
