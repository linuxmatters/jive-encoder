package encoder

import (
	"github.com/linuxmatters/ffmpeg-statigo"
)

// formatPreset describes how a single output format is encoded and muxed. The
// table below is the single source of truth for codec, bitrate, sample format,
// muxer, extension, lowpass policy, and cover capability, so the encoder reads
// the preset rather than branching on the format name.
type formatPreset struct {
	// name is the lowercase format identifier (mp3, aac, opus).
	name string
	// codecID is the FFmpeg codec used for the audio stream.
	codecID ffmpeg.AVCodecID
	// encoderName, when set, names a specific encoder to try before falling
	// back to the codec ID (e.g. libopus).
	encoderName string
	// monoBitrate and stereoBitrate are the constant bitrates in bits per
	// second for each channel mode.
	monoBitrate   int
	stereoBitrate int
	// vbr selects variable bitrate encoding when true.
	vbr bool
	// sampleFmt is the sample format the encoder expects and the filter graph
	// must produce.
	sampleFmt ffmpeg.AVSampleFormat
	// sampleRate is the output sample rate in Hz. MP3 and AAC use 44.1 kHz;
	// libopus rejects 44.1 kHz at open, so Opus uses 48 kHz.
	sampleRate int
	// muxer is the output format name for AVFormatAllocOutputContext2.
	muxer string
	// extension is the output file extension including the leading dot.
	extension string
	// lowpassHz is the lowpass cutoff in Hz, or 0 for no lowpass.
	lowpassHz int
	// coverCapable reports whether the format embeds an attached-picture cover.
	coverCapable bool
	// encoderOpts are extra encoder options passed via AVDictionary.
	encoderOpts map[string]string
}

// formatPresets maps each supported format name to its preset. MP3 and AAC use
// 44.1 kHz; Opus uses 48 kHz (libopus rejects 44.1 kHz at open). MP3 is CBR with
// a 20.5 kHz lowpass and LAME compression level 3; AAC-LC is CBR with no lowpass;
// Opus is VBR with no lowpass.
var formatPresets = map[string]formatPreset{
	"mp3": {
		name:          "mp3",
		codecID:       ffmpeg.AVCodecIdMp3,
		monoBitrate:   MonoBitrate,
		stereoBitrate: StereoBitrate,
		vbr:           false,
		sampleFmt:     ffmpeg.AVSampleFmtS16P,
		sampleRate:    44100,
		muxer:         "mp3",
		extension:     ".mp3",
		lowpassHz:     20500,
		coverCapable:  true,
		encoderOpts: map[string]string{
			"compression_level": "3",
			"cutoff":            "20500",
		},
	},
	"aac": {
		name:          "aac",
		codecID:       ffmpeg.AVCodecIdAac,
		monoBitrate:   64000,
		stereoBitrate: 128000,
		vbr:           false,
		sampleFmt:     ffmpeg.AVSampleFmtFltp,
		sampleRate:    44100,
		muxer:         "ipod",
		extension:     ".m4a",
		lowpassHz:     0,
		coverCapable:  true,
		encoderOpts:   nil,
	},
	"opus": {
		name:          "opus",
		codecID:       ffmpeg.AVCodecIdOpus,
		encoderName:   "libopus",
		monoBitrate:   32000,
		stereoBitrate: 48000,
		vbr:           true,
		sampleFmt:     ffmpeg.AVSampleFmtFlt,
		sampleRate:    48000,
		muxer:         "opus",
		extension:     ".opus",
		lowpassHz:     0,
		coverCapable:  false,
		encoderOpts: map[string]string{
			"vbr":               "on",
			"compression_level": "10",
		},
	},
}

// presetFor resolves a format name to its preset. The second return value is
// false when the name is unknown.
func presetFor(name string) (formatPreset, bool) {
	preset, ok := formatPresets[name]
	return preset, ok
}

// ExtensionFor returns the output file extension (including the leading dot)
// for the given format name. Unknown formats return an empty string.
func ExtensionFor(format string) string {
	preset, ok := formatPresets[format]
	if !ok {
		return ""
	}
	return preset.extension
}
