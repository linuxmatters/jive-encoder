package encoder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/linuxmatters/ffmpeg-statigo"
)

// ErrCancelled is returned by Encode when Cancel has been called before the
// encode finished. Callers treat it as a clean stop, not an encoding failure.
var ErrCancelled = errors.New("encoding cancelled")

// Podcast bitrate presets in bits per second: 192kbps stereo, 112kbps mono.
const (
	MonoBitrate   = 112000
	StereoBitrate = 192000
)

// Encoder handles audio encoding (MP3, AAC, Opus) from audio input files
type Encoder struct {
	inputPath  string
	outputPath string
	stereo     bool

	input    inputState
	output   outputState
	filter   filterState
	pipeline pipelineState

	preset   formatPreset
	metadata Metadata
	coverArt []byte // scaled PNG cover bytes; empty disables the attached-picture stream

	closed bool // Track if Close() has been called to prevent double-free

	// cancelled is set by Cancel and observed at the top of the decode loop so
	// Encode unwinds the cgo call chain before any Close frees the AV contexts.
	cancelled atomic.Bool
}

type inputState struct {
	format       *ffmpeg.AVFormatContext
	codec        *ffmpeg.AVCodecContext
	streamIndex  int
	totalSamples int64
}

type outputState struct {
	format           *ffmpeg.AVFormatContext
	codec            *ffmpeg.AVCodecContext
	audioStreamIndex int
	coverStreamIndex int
}

type filterState struct {
	graph *ffmpeg.AVFilterGraph
	src   *ffmpeg.AVFilterContext
	sink  *ffmpeg.AVFilterContext
	frame *ffmpeg.AVFrame
}

type pipelineState struct {
	decoded     *ffmpeg.AVFrame
	packet      *ffmpeg.AVPacket
	samplesRead int64
	nextPts     int64
}

// Metadata carries episode tag fields into the encoder so it can write
// muxer-native metadata during Initialize/Encode. It is the single tag-field
// carrier: the CLI workflows build it and the encoder maps its fields to
// standard muxer tag keys.
type Metadata struct {
	EpisodeNumber string
	Title         string
	Artist        string
	Album         string
	Date          string
	Comment       string
}

// Config holds encoder configuration
type Config struct {
	InputPath  string
	OutputPath string
	Stereo     bool     // true = 192kbps stereo, false = 112kbps mono
	Format     string   // output format (mp3, aac, opus); defaults to mp3 when empty
	Metadata   Metadata // episode tag fields written as muxer-native metadata
	CoverArt   []byte   // scaled PNG cover bytes; embedded as an attached picture for cover-capable formats
}

// New creates a new encoder instance
func New(cfg Config) (*Encoder, error) {
	if cfg.InputPath == "" {
		return nil, fmt.Errorf("input path is required")
	}
	if cfg.OutputPath == "" {
		return nil, fmt.Errorf("output path is required")
	}

	format := cfg.Format
	if format == "" {
		format = "mp3"
	}
	preset, ok := presetFor(format)
	if !ok {
		return nil, fmt.Errorf("unknown output format: %q", format)
	}
	if outputExt := filepath.Ext(cfg.OutputPath); !strings.EqualFold(outputExt, preset.extension) {
		return nil, fmt.Errorf("output path extension %q does not match %s format extension %q", outputExt, preset.name, preset.extension)
	}

	return &Encoder{
		inputPath:  cfg.InputPath,
		outputPath: cfg.OutputPath,
		stereo:     cfg.Stereo,
		preset:     preset,
		metadata:   cfg.Metadata,
		coverArt:   cfg.CoverArt,
		input: inputState{
			streamIndex: -1,
		},
		output: outputState{
			audioStreamIndex: -1,
			coverStreamIndex: -1,
		},
	}, nil
}

// Initialize opens input and output files, sets up decoder and encoder
func (e *Encoder) Initialize() error {
	// Keep stderr quiet: only surface FFmpeg errors, not its info/warning spam.
	ffmpeg.AVLogSetLevel(ffmpeg.AVLogError)

	// Every error path releases what was already opened. Close is idempotent
	// (guarded by e.closed), so the caller's own deferred Close is a safe no-op
	// after these calls.
	if err := e.openInput(); err != nil {
		e.Close()
		return fmt.Errorf("failed to open input: %w", err)
	}

	if err := e.openOutput(); err != nil {
		e.Close()
		return fmt.Errorf("failed to open output: %w", err)
	}

	e.pipeline.decoded = ffmpeg.AVFrameAlloc()
	e.filter.frame = ffmpeg.AVFrameAlloc()
	e.pipeline.packet = ffmpeg.AVPacketAlloc()

	if err := e.initFilter(); err != nil {
		e.Close()
		return fmt.Errorf("failed to initialize filter: %w", err)
	}

	return nil
}

// openInput opens and analyses the input audio file
func (e *Encoder) openInput() error {
	urlPtr := ffmpeg.ToCStr(e.inputPath)
	defer urlPtr.Free()

	if _, err := ffmpeg.AVFormatOpenInput(&e.input.format, urlPtr, nil, nil); err != nil {
		return fmt.Errorf("cannot open input file: %w", err)
	}

	if _, err := ffmpeg.AVFormatFindStreamInfo(e.input.format, nil); err != nil {
		return fmt.Errorf("cannot find stream information: %w", err)
	}

	streamIdx, err := ffmpeg.AVFindBestStream(e.input.format, ffmpeg.AVMediaTypeAudio, -1, -1, nil, 0)
	if err != nil {
		return fmt.Errorf("cannot find audio stream: %w", err)
	}
	e.input.streamIndex = streamIdx

	stream := e.input.format.Streams().Get(uintptr(e.input.streamIndex)) //nolint:gosec // streamIndex is validated by AVFindBestStream
	codecPar := stream.Codecpar()

	decoder := ffmpeg.AVCodecFindDecoder(codecPar.CodecId())
	if decoder == nil {
		return fmt.Errorf("decoder not found for codec %d", codecPar.CodecId())
	}

	e.input.codec = ffmpeg.AVCodecAllocContext3(decoder)
	if e.input.codec == nil {
		return fmt.Errorf("failed to allocate decoder context")
	}

	if _, err := ffmpeg.AVCodecParametersToContext(e.input.codec, codecPar); err != nil {
		return fmt.Errorf("failed to copy codec parameters: %w", err)
	}

	if _, err := ffmpeg.AVCodecOpen2(e.input.codec, decoder, nil); err != nil {
		return fmt.Errorf("failed to open decoder: %w", err)
	}

	// Precompute total sample count to drive the progress callback.
	duration := stream.Duration()
	timeBase := stream.TimeBase()
	if duration > 0 {
		durationSec := float64(duration) * float64(timeBase.Num()) / float64(timeBase.Den())
		e.input.totalSamples = int64(durationSec * float64(e.input.codec.SampleRate()))
	}

	return nil
}

// ProgressCallback is called during encoding with progress updates
type ProgressCallback func(samplesProcessed, totalSamples int64)

// Encode performs the actual encoding with progress callbacks
func (e *Encoder) Encode(progressCb ProgressCallback) (err error) {
	packet := ffmpeg.AVPacketAlloc()
	defer ffmpeg.AVPacketFree(&packet)

	// Any error return (including ErrCancelled) leaves a truncated output file.
	// Remove it so a failed or cancelled run leaves nothing behind; a successful
	// encode keeps the file. os.Remove is idempotent, so the CLI caller's own
	// cleanup after Encode stays a safe no-op.
	defer func() {
		if err != nil {
			os.Remove(e.outputPath)
		}
	}()

	outStream := e.output.format.Streams().Get(uintptr(e.output.audioStreamIndex)) //nolint:gosec // audioStreamIndex is set from AVFormatNewStream in openOutput

	for {
		// Observe cancellation before the next cgo call so Encode returns while
		// the AV contexts are still valid, ahead of any Close.
		if e.cancelled.Load() {
			return ErrCancelled
		}

		if _, err := ffmpeg.AVReadFrame(e.input.format, packet); err != nil {
			if errors.Is(err, ffmpeg.AVErrorEOF) {
				break
			}
			return fmt.Errorf("read frame failed: %w", err)
		}

		if packet.StreamIndex() != e.input.streamIndex {
			ffmpeg.AVPacketUnref(packet)
			continue
		}

		if _, err := ffmpeg.AVCodecSendPacket(e.input.codec, packet); err != nil {
			ffmpeg.AVPacketUnref(packet)
			return fmt.Errorf("send packet to decoder failed: %w", err)
		}

		ffmpeg.AVPacketUnref(packet)

		for {
			if e.cancelled.Load() {
				return ErrCancelled
			}

			if _, err := ffmpeg.AVCodecReceiveFrame(e.input.codec, e.pipeline.decoded); err != nil {
				if errors.Is(err, ffmpeg.EAgain) || errors.Is(err, ffmpeg.AVErrorEOF) {
					break
				}
				return fmt.Errorf("receive frame from decoder failed: %w", err)
			}

			e.pipeline.samplesRead += int64(e.pipeline.decoded.NbSamples())
			if progressCb != nil && e.input.totalSamples > 0 {
				progressCb(e.pipeline.samplesRead, e.input.totalSamples)
			}

			if _, err := ffmpeg.AVBuffersrcAddFrameFlags(e.filter.src, e.pipeline.decoded, ffmpeg.AVBuffersrcFlagKeepRef); err != nil {
				return fmt.Errorf("failed to feed filter graph: %w", err)
			}

			if err := e.drainFilterGraph(outStream); err != nil {
				return err
			}

			ffmpeg.AVFrameUnref(e.pipeline.decoded)
		}
	}

	// Flush decoder
	if _, err := ffmpeg.AVCodecSendPacket(e.input.codec, nil); err != nil {
		return fmt.Errorf("flush decoder failed: %w", err)
	}

	for {
		if _, err := ffmpeg.AVCodecReceiveFrame(e.input.codec, e.pipeline.decoded); err != nil {
			if errors.Is(err, ffmpeg.EAgain) || errors.Is(err, ffmpeg.AVErrorEOF) {
				break
			}
			return fmt.Errorf("flush decoder receive failed: %w", err)
		}

		// Keep a ref (AVBuffersrcFlagKeepRef) because we reuse the decoded frame
		// each iteration and unref it ourselves below. The filter-graph flush feeds a
		// nil frame, so KEEP_REF is inapplicable there and it passes 0.
		if _, err := ffmpeg.AVBuffersrcAddFrameFlags(e.filter.src, e.pipeline.decoded, ffmpeg.AVBuffersrcFlagKeepRef); err != nil {
			return fmt.Errorf("failed to feed filter graph: %w", err)
		}

		if err := e.drainFilterGraph(outStream); err != nil {
			return err
		}

		ffmpeg.AVFrameUnref(e.pipeline.decoded)
	}

	// Flush filter graph
	if _, err := ffmpeg.AVBuffersrcAddFrameFlags(e.filter.src, nil, 0); err != nil {
		return fmt.Errorf("failed to flush filter graph: %w", err)
	}

	if err := e.drainFilterGraph(outStream); err != nil {
		return err
	}

	// Flush encoder
	if err := e.flushEncoder(outStream); err != nil {
		return err
	}

	// Write trailer
	if _, err := ffmpeg.AVWriteTrailer(e.output.format); err != nil {
		return fmt.Errorf("write trailer failed: %w", err)
	}

	return nil
}

// drainFilterGraph reads filtered frames from the buffersink until EAGAIN or
// EOF, encoding each one. Callers feed the buffersrc before invoking this.
func (e *Encoder) drainFilterGraph(outStream *ffmpeg.AVStream) error {
	for {
		if _, err := ffmpeg.AVBuffersinkGetFrame(e.filter.sink, e.filter.frame); err != nil {
			if errors.Is(err, ffmpeg.EAgain) || errors.Is(err, ffmpeg.AVErrorEOF) {
				break
			}
			return fmt.Errorf("failed to get filtered frame: %w", err)
		}

		if err := e.encodeFrame(e.filter.frame, outStream); err != nil {
			return err
		}

		ffmpeg.AVFrameUnref(e.filter.frame)
	}

	return nil
}

// encodeFrame encodes a single audio frame with the preset's codec
func (e *Encoder) encodeFrame(frame *ffmpeg.AVFrame, outStream *ffmpeg.AVStream) error {
	// Stamp a monotonic PTS from the running sample counter so the filter's
	// reframing does not leave gaps the encoder would reject.
	frame.SetPts(e.pipeline.nextPts)
	e.pipeline.nextPts += int64(frame.NbSamples())

	if _, err := ffmpeg.AVCodecSendFrame(e.output.codec, frame); err != nil {
		return fmt.Errorf("send frame to encoder failed: %w", err)
	}

	return e.drainEncoder(outStream, "receive packet from encoder failed")
}

// drainEncoder reads encoded packets from the encoder and writes them to the
// output stream until the encoder needs more input or reaches EOF. recvErrCtx
// labels the receive-packet error so each caller keeps its existing wording.
func (e *Encoder) drainEncoder(outStream *ffmpeg.AVStream, recvErrCtx string) error {
	for {
		ffmpeg.AVPacketUnref(e.pipeline.packet)

		if _, err := ffmpeg.AVCodecReceivePacket(e.output.codec, e.pipeline.packet); err != nil {
			if errors.Is(err, ffmpeg.EAgain) || errors.Is(err, ffmpeg.AVErrorEOF) {
				break
			}
			return fmt.Errorf("%s: %w", recvErrCtx, err)
		}

		// Rescale packet timestamps from encoder to output stream time base.
		e.pipeline.packet.SetStreamIndex(e.output.audioStreamIndex)
		ffmpeg.AVPacketRescaleTs(e.pipeline.packet, e.output.codec.TimeBase(), outStream.TimeBase())

		if _, err := ffmpeg.AVInterleavedWriteFrame(e.output.format, e.pipeline.packet); err != nil {
			return fmt.Errorf("write frame failed: %w", err)
		}
	}

	return nil
}

// flushEncoder flushes remaining packets from the encoder
func (e *Encoder) flushEncoder(outStream *ffmpeg.AVStream) error {
	if _, err := ffmpeg.AVCodecSendFrame(e.output.codec, nil); err != nil {
		return fmt.Errorf("flush encoder failed: %w", err)
	}

	return e.drainEncoder(outStream, "flush encoder receive failed")
}

// Cancel requests that a running Encode stop at the next loop iteration. It is
// safe to call from another goroutine and returns immediately; Encode then
// returns ErrCancelled once its current cgo call unwinds. Cancel does not free
// any resources; the caller must still await Encode before calling Close.
func (e *Encoder) Cancel() {
	e.cancelled.Store(true)
}

// Close releases all resources
func (e *Encoder) Close() {
	// Prevent double-close which could cause issues with already-freed FFmpeg resources
	if e.closed {
		return
	}
	e.closed = true

	if e.filter.frame != nil {
		ffmpeg.AVFrameFree(&e.filter.frame)
	}
	if e.pipeline.packet != nil {
		ffmpeg.AVPacketFree(&e.pipeline.packet)
	}
	if e.pipeline.decoded != nil {
		ffmpeg.AVFrameFree(&e.pipeline.decoded)
	}
	if e.filter.graph != nil {
		ffmpeg.AVFilterGraphFree(&e.filter.graph)
	}
	if e.output.codec != nil {
		ffmpeg.AVCodecFreeContext(&e.output.codec)
	}
	if e.input.codec != nil {
		ffmpeg.AVCodecFreeContext(&e.input.codec)
	}
	if e.output.format != nil {
		if e.output.format.Oformat().Flags()&ffmpeg.AVFmtNofile == 0 && e.output.format.Pb() != nil {
			ffmpeg.AVIOClose(e.output.format.Pb())
			e.output.format.SetPb(nil)
		}
		ffmpeg.AVFormatFreeContext(e.output.format)
	}
	if e.input.format != nil {
		ffmpeg.AVFormatCloseInput(&e.input.format)
	}
}

// GetInputInfo returns information about the input audio
func (e *Encoder) GetInputInfo() (sampleRate, channels int, format string) {
	if e.input.codec == nil {
		return 0, 0, "unknown"
	}

	codecName := e.input.codec.Codec().Name()
	return e.input.codec.SampleRate(), e.input.codec.ChLayout().NbChannels(), codecName.String()
}

// GetDurationSecs returns the duration of the encoded audio in seconds.
// This is calculated from the samples processed during encoding, avoiding
// the need to re-open the output file. Should be called after Encode() completes.
func (e *Encoder) GetDurationSecs() int64 {
	if e.output.codec == nil {
		return 0
	}
	sampleRate := e.output.codec.SampleRate()
	if sampleRate <= 0 {
		return 0
	}
	// nextPts tracks total samples written to the encoder; round to nearest second
	return (e.pipeline.nextPts + int64(sampleRate)/2) / int64(sampleRate)
}

// Bitrate returns the output bitrate in kbps for the configured channel mode,
// read from the active format preset (CBR for MP3/AAC, the VBR target for Opus).
func (e *Encoder) Bitrate() int {
	if e.stereo {
		return e.preset.stereoBitrate / 1000
	}
	return e.preset.monoBitrate / 1000
}

// FormatLabel returns the uppercase format name for display (e.g. "MP3",
// "AAC", "OPUS").
func (e *Encoder) FormatLabel() string {
	return strings.ToUpper(e.preset.name)
}

// OutputSampleRate returns the output sample rate in Hz, read from the active
// format preset (44.1 kHz for MP3/AAC, 48 kHz for Opus).
func (e *Encoder) OutputSampleRate() int {
	return e.preset.sampleRate
}

// VBR reports whether the active format encodes at a variable bitrate. The UI
// labels the bitrate "VBR" when true and "CBR" otherwise.
func (e *Encoder) VBR() bool {
	return e.preset.vbr
}

// ChannelMode returns the output channel mode label: "stereo" or "mono".
func (e *Encoder) ChannelMode() string {
	if e.stereo {
		return "stereo"
	}
	return "mono"
}

// FormatChannelMode formats channel count as "mono", "stereo", etc.
func FormatChannelMode(channels int) string {
	switch channels {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	default:
		return fmt.Sprintf("%dch", channels)
	}
}
