package encoder

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"sync/atomic"
	"unsafe"

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

// Encoder handles MP3 encoding from audio input files
type Encoder struct {
	inputPath  string
	outputPath string
	stereo     bool

	ifmtCtx *ffmpeg.AVFormatContext
	ofmtCtx *ffmpeg.AVFormatContext

	decCtx *ffmpeg.AVCodecContext
	encCtx *ffmpeg.AVCodecContext

	decFrame *ffmpeg.AVFrame
	encPkt   *ffmpeg.AVPacket

	filterGraph   *ffmpeg.AVFilterGraph
	bufferSrcCtx  *ffmpeg.AVFilterContext
	bufferSinkCtx *ffmpeg.AVFilterContext
	filteredFrame *ffmpeg.AVFrame

	preset   formatPreset
	metadata Metadata
	coverArt []byte // scaled PNG cover bytes; empty disables the attached-picture stream

	streamIndex      int
	outStreamIndex   int // OUTPUT audio stream index, distinct from input streamIndex
	coverStreamIndex int // attached-picture stream index, -1 when no cover stream
	samplesRead      int64
	totalSamples     int64
	nextPts          int64 // Track PTS for output frames
	closed           bool  // Track if Close() has been called to prevent double-free

	// cancelled is set by Cancel and observed at the top of the decode loop so
	// Encode unwinds the cgo call chain before any Close frees the AV contexts.
	cancelled atomic.Bool
}

// Metadata carries episode tag fields into the encoder so it can write
// muxer-native metadata during Initialize/Encode. It mirrors the text fields of
// the id3 tag set but lives in the encoder package to avoid an encoder->id3
// import cycle (id3 is being removed).
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

	return &Encoder{
		inputPath:        cfg.InputPath,
		outputPath:       cfg.OutputPath,
		stereo:           cfg.Stereo,
		preset:           preset,
		metadata:         cfg.Metadata,
		coverArt:         cfg.CoverArt,
		streamIndex:      -1,
		outStreamIndex:   -1,
		coverStreamIndex: -1,
	}, nil
}

// Initialize opens input and output files, sets up decoder and encoder
func (e *Encoder) Initialize() error {
	// Keep stderr quiet: only surface FFmpeg errors, not its info/warning spam.
	ffmpeg.AVLogSetLevel(ffmpeg.AVLogError)

	if err := e.openInput(); err != nil {
		return fmt.Errorf("failed to open input: %w", err)
	}

	if err := e.openOutput(); err != nil {
		e.Close()
		return fmt.Errorf("failed to open output: %w", err)
	}

	e.decFrame = ffmpeg.AVFrameAlloc()
	e.filteredFrame = ffmpeg.AVFrameAlloc()
	e.encPkt = ffmpeg.AVPacketAlloc()

	if err := e.initFilter(); err != nil {
		return fmt.Errorf("failed to initialize filter: %w", err)
	}

	return nil
}

// openInput opens and analyzes the input audio file
func (e *Encoder) openInput() error {
	urlPtr := ffmpeg.ToCStr(e.inputPath)
	defer urlPtr.Free()

	if _, err := ffmpeg.AVFormatOpenInput(&e.ifmtCtx, urlPtr, nil, nil); err != nil {
		return fmt.Errorf("cannot open input file: %w", err)
	}

	if _, err := ffmpeg.AVFormatFindStreamInfo(e.ifmtCtx, nil); err != nil {
		return fmt.Errorf("cannot find stream information: %w", err)
	}

	streamIdx, err := ffmpeg.AVFindBestStream(e.ifmtCtx, ffmpeg.AVMediaTypeAudio, -1, -1, nil, 0)
	if err != nil {
		return fmt.Errorf("cannot find audio stream: %w", err)
	}
	e.streamIndex = streamIdx

	stream := e.ifmtCtx.Streams().Get(uintptr(e.streamIndex)) //nolint:gosec // streamIndex is validated by AVFindBestStream
	codecPar := stream.Codecpar()

	decoder := ffmpeg.AVCodecFindDecoder(codecPar.CodecId())
	if decoder == nil {
		return fmt.Errorf("decoder not found for codec %d", codecPar.CodecId())
	}

	e.decCtx = ffmpeg.AVCodecAllocContext3(decoder)
	if e.decCtx == nil {
		return fmt.Errorf("failed to allocate decoder context")
	}

	if _, err := ffmpeg.AVCodecParametersToContext(e.decCtx, codecPar); err != nil {
		return fmt.Errorf("failed to copy codec parameters: %w", err)
	}

	if _, err := ffmpeg.AVCodecOpen2(e.decCtx, decoder, nil); err != nil {
		return fmt.Errorf("failed to open decoder: %w", err)
	}

	// Precompute total sample count to drive the progress callback.
	duration := stream.Duration()
	timeBase := stream.TimeBase()
	if duration > 0 {
		durationSec := float64(duration) * float64(timeBase.Num()) / float64(timeBase.Den())
		e.totalSamples = int64(durationSec * float64(e.decCtx.SampleRate()))
	}

	return nil
}

// openOutput creates the output MP3 file and sets up the encoder
func (e *Encoder) openOutput() error {
	namePtr := ffmpeg.ToCStr(e.outputPath)
	defer namePtr.Free()

	if _, err := ffmpeg.AVFormatAllocOutputContext2(&e.ofmtCtx, nil, nil, namePtr); err != nil {
		return fmt.Errorf("failed to create output context: %w", err)
	}

	var encoder *ffmpeg.AVCodec
	if e.preset.encoderName != "" {
		namePtr := ffmpeg.ToCStr(e.preset.encoderName)
		encoder = ffmpeg.AVCodecFindEncoderByName(namePtr)
		namePtr.Free()
	}
	if encoder == nil {
		encoder = ffmpeg.AVCodecFindEncoder(e.preset.codecID)
	}
	if encoder == nil {
		return fmt.Errorf("%s encoder not found", e.preset.name)
	}

	outStream := ffmpeg.AVFormatNewStream(e.ofmtCtx, encoder)
	if outStream == nil {
		return fmt.Errorf("failed to create output stream")
	}
	e.outStreamIndex = outStream.Index()

	e.encCtx = ffmpeg.AVCodecAllocContext3(encoder)
	if e.encCtx == nil {
		return fmt.Errorf("failed to allocate encoder context")
	}

	if e.stereo {
		e.encCtx.SetBitRate(int64(e.preset.stereoBitrate))
		ffmpeg.AVChannelLayoutDefault(e.encCtx.ChLayout(), 2)
	} else {
		e.encCtx.SetBitRate(int64(e.preset.monoBitrate))
		ffmpeg.AVChannelLayoutDefault(e.encCtx.ChLayout(), 1)
	}

	e.encCtx.SetSampleRate(e.preset.sampleRate)
	e.encCtx.SetSampleFmt(e.preset.sampleFmt)

	tb := &ffmpeg.AVRational{}
	tb.SetNum(1)
	tb.SetDen(e.encCtx.SampleRate())
	e.encCtx.SetTimeBase(tb)

	// Encoder tuning passed through AVDictionary, driven by the preset.
	var opts *ffmpeg.AVDictionary

	for key, val := range e.preset.encoderOpts {
		keyPtr := ffmpeg.ToCStr(key)
		valPtr := ffmpeg.ToCStr(val)
		_, err := ffmpeg.AVDictSet(&opts, keyPtr, valPtr, 0)
		keyPtr.Free()
		valPtr.Free()
		if err != nil {
			ffmpeg.AVDictFree(&opts)
			return fmt.Errorf("failed to set encoder option %s: %w", key, err)
		}
	}

	if _, err := ffmpeg.AVCodecOpen2(e.encCtx, encoder, &opts); err != nil {
		ffmpeg.AVDictFree(&opts)
		return fmt.Errorf("failed to open encoder: %w", err)
	}
	ffmpeg.AVDictFree(&opts)

	if _, err := ffmpeg.AVCodecParametersFromContext(outStream.Codecpar(), e.encCtx); err != nil {
		return fmt.Errorf("failed to copy encoder parameters: %w", err)
	}

	outStream.SetTimeBase(e.encCtx.TimeBase())

	// Formats without the NOFILE flag need an explicit AVIO output handle.
	if e.ofmtCtx.Oformat().Flags()&ffmpeg.AVFmtNofile == 0 {
		var pb *ffmpeg.AVIOContext
		if _, err := ffmpeg.AVIOOpen(&pb, e.ofmtCtx.Url(), ffmpeg.AVIOFlagWrite); err != nil {
			return fmt.Errorf("failed to open output file: %w", err)
		}
		e.ofmtCtx.SetPb(pb)
	}

	if err := e.setMuxerMetadata(); err != nil {
		return err
	}

	// Add the attached-picture stream after the audio stream so audio keeps
	// index 0 (outStreamIndex). Opus is not cover-capable and absent cover
	// bytes mean no second stream, leaving the audio-only path unchanged.
	if e.preset.coverCapable && len(e.coverArt) > 0 {
		if err := e.addCoverStream(); err != nil {
			return err
		}
	}

	// id3v2_version is an mp3-muxer-private option, so it goes through the
	// WriteHeader options dict, not the format-context metadata. Other muxers
	// ignore it. The dict is owned here and freed after WriteHeader.
	var muxerOpts *ffmpeg.AVDictionary
	if e.preset.name == "mp3" {
		keyPtr := ffmpeg.ToCStr("id3v2_version")
		valPtr := ffmpeg.ToCStr("4")
		_, err := ffmpeg.AVDictSet(&muxerOpts, keyPtr, valPtr, 0)
		keyPtr.Free()
		valPtr.Free()
		if err != nil {
			ffmpeg.AVDictFree(&muxerOpts)
			return fmt.Errorf("failed to set id3v2_version option: %w", err)
		}
	}

	if _, err := ffmpeg.AVFormatWriteHeader(e.ofmtCtx, &muxerOpts); err != nil {
		ffmpeg.AVDictFree(&muxerOpts)
		return fmt.Errorf("failed to write header: %w", err)
	}
	ffmpeg.AVDictFree(&muxerOpts)

	// Write the cover picture immediately after the header so the muxer carries
	// it as the attached picture before any audio packet.
	if e.coverStreamIndex >= 0 {
		if err := e.writeCoverPacket(); err != nil {
			return err
		}
	}

	return nil
}

// addCoverStream creates the attached-picture stream that carries the scaled
// PNG cover. It is added after the audio stream, so the audio stream keeps
// index 0. The packet itself is written after AVFormatWriteHeader by
// writeCoverPacket.
func (e *Encoder) addCoverStream() error {
	coverStream := ffmpeg.AVFormatNewStream(e.ofmtCtx, nil)
	if coverStream == nil {
		return fmt.Errorf("failed to create cover stream")
	}

	// The mp3 and ipod muxers reject an attached-picture stream without
	// dimensions, so read them from the PNG header.
	cfg, err := png.DecodeConfig(bytes.NewReader(e.coverArt))
	if err != nil {
		return fmt.Errorf("failed to read cover dimensions: %w", err)
	}

	codecPar := coverStream.Codecpar()
	codecPar.SetCodecType(ffmpeg.AVMediaTypeVideo)
	// ScaleCoverArt always emits PNG, so the picture stream uses the PNG codec.
	codecPar.SetCodecId(ffmpeg.AVCodecIdPng)
	codecPar.SetWidth(cfg.Width)
	codecPar.SetHeight(cfg.Height)
	coverStream.SetDisposition(ffmpeg.AVDispositionAttachedPic)

	e.coverStreamIndex = coverStream.Index()
	return nil
}

// writeCoverPacket allocates a packet sized to the cover bytes, copies the PNG
// data into it, marks it a keyframe on the attached-picture stream, and writes
// it to the muxer. The packet is freed before returning, so Close never touches
// it.
func (e *Encoder) writeCoverPacket() error {
	pkt := ffmpeg.AVPacketAlloc()
	if pkt == nil {
		return fmt.Errorf("failed to allocate cover packet")
	}
	defer ffmpeg.AVPacketFree(&pkt)

	if _, err := ffmpeg.AVNewPacket(pkt, len(e.coverArt)); err != nil {
		return fmt.Errorf("failed to allocate cover packet data: %w", err)
	}

	dst := unsafe.Slice((*byte)(pkt.Data()), len(e.coverArt))
	copy(dst, e.coverArt)

	pkt.SetStreamIndex(e.coverStreamIndex)
	pkt.SetFlags(pkt.Flags() | ffmpeg.AVPktFlagKey)

	if _, err := ffmpeg.AVInterleavedWriteFrame(e.ofmtCtx, pkt); err != nil {
		return fmt.Errorf("failed to write cover packet: %w", err)
	}

	return nil
}

// setMuxerMetadata builds the standard-key tag dictionary from the episode
// metadata and hands it to the output format context. SetMetadata transfers
// ownership to the context (freed by avformat_free_context), so this dict is
// never freed here. Preset-agnostic: every format gets the same standard keys.
func (e *Encoder) setMuxerMetadata() error {
	tags := buildMuxerTags(e.metadata)
	if len(tags) == 0 {
		return nil
	}

	var dict *ffmpeg.AVDictionary
	for _, tag := range tags {
		keyPtr := ffmpeg.ToCStr(tag.Key)
		valPtr := ffmpeg.ToCStr(tag.Value)
		_, err := ffmpeg.AVDictSet(&dict, keyPtr, valPtr, 0)
		keyPtr.Free()
		valPtr.Free()
		if err != nil {
			ffmpeg.AVDictFree(&dict)
			return fmt.Errorf("failed to set metadata %s: %w", tag.Key, err)
		}
	}

	e.ofmtCtx.SetMetadata(dict)
	return nil
}

// initFilter sets up audio filter graph for resampling and frame buffering
func (e *Encoder) initFilter() error {
	e.filterGraph = ffmpeg.AVFilterGraphAlloc()
	if e.filterGraph == nil {
		return fmt.Errorf("failed to allocate filter graph")
	}

	bufferSrc := ffmpeg.AVFilterGetByName(ffmpeg.GlobalCStr("abuffer"))
	bufferSink := ffmpeg.AVFilterGetByName(ffmpeg.GlobalCStr("abuffersink"))
	if bufferSrc == nil || bufferSink == nil {
		return fmt.Errorf("abuffer or abuffersink filter not found")
	}

	layoutPtr := ffmpeg.AllocCStr(64)
	defer layoutPtr.Free()
	if _, err := ffmpeg.AVChannelLayoutDescribe(e.decCtx.ChLayout(), layoutPtr, 64); err != nil {
		return fmt.Errorf("failed to describe channel layout: %w", err)
	}

	pktTimebase := e.decCtx.PktTimebase()
	args := fmt.Sprintf(
		"time_base=%d/%d:sample_rate=%d:sample_fmt=%s:channel_layout=%s",
		pktTimebase.Num(), pktTimebase.Den(),
		e.decCtx.SampleRate(),
		ffmpeg.AVGetSampleFmtName(e.decCtx.SampleFmt()).String(),
		layoutPtr.String(),
	)

	argsC := ffmpeg.ToCStr(args)
	defer argsC.Free()

	if _, err := ffmpeg.AVFilterGraphCreateFilter(
		&e.bufferSrcCtx,
		bufferSrc,
		ffmpeg.GlobalCStr("in"),
		argsC,
		nil,
		e.filterGraph,
	); err != nil {
		return fmt.Errorf("failed to create buffer source: %w", err)
	}

	if _, err := ffmpeg.AVFilterGraphCreateFilter(
		&e.bufferSinkCtx,
		bufferSink,
		ffmpeg.GlobalCStr("out"),
		nil,
		nil,
		e.filterGraph,
	); err != nil {
		return fmt.Errorf("failed to create buffer sink: %w", err)
	}

	// Parse filter graph - use aresample for format/rate/channel conversion
	outputs := ffmpeg.AVFilterInoutAlloc()
	inputs := ffmpeg.AVFilterInoutAlloc()
	defer ffmpeg.AVFilterInoutFree(&outputs)
	defer ffmpeg.AVFilterInoutFree(&inputs)

	outputs.SetName(ffmpeg.ToCStr("in"))
	outputs.SetFilterCtx(e.bufferSrcCtx)
	outputs.SetPadIdx(0)
	outputs.SetNext(nil)

	inputs.SetName(ffmpeg.ToCStr("out"))
	inputs.SetFilterCtx(e.bufferSinkCtx)
	inputs.SetPadIdx(0)
	inputs.SetNext(nil)

	// Build filter spec: resample to the preset's sample rate and sample format,
	// set the target channel layout (stereo keeps channels, mono downmixes).
	// Frame sizing is applied to the buffer sink after this graph is built
	// (see below), not via asetnsamples, so each encoder gets its required size.
	channelLayout := "mono"
	if e.stereo {
		channelLayout = "stereo"
	}
	sampleFmtName := ffmpeg.AVGetSampleFmtName(e.preset.sampleFmt).String()
	filterSpec := fmt.Sprintf("aresample=%d:async=1,aformat=sample_fmts=%s:sample_rates=%d:channel_layouts=%s",
		e.preset.sampleRate, sampleFmtName, e.preset.sampleRate, channelLayout)

	filterSpecC := ffmpeg.ToCStr(filterSpec)
	defer filterSpecC.Free()

	if _, err := ffmpeg.AVFilterGraphParsePtr(e.filterGraph, filterSpecC, &inputs, &outputs, nil); err != nil {
		return fmt.Errorf("failed to parse filter graph: %w", err)
	}

	if _, err := ffmpeg.AVFilterGraphConfig(e.filterGraph, nil); err != nil {
		return fmt.Errorf("failed to configure filter graph: %w", err)
	}

	// Fix the buffer-sink frame size to the encoder's required frame size so the
	// filter delivers exactly the frames the encoder expects (LAME 1152, native
	// AAC 1024, libopus its own). Encoders that accept variable-size frames
	// advertise AV_CODEC_CAP_VARIABLE_FRAME_SIZE and need no fixed size.
	// openOutput runs before initFilter, so encCtx.FrameSize() is populated here.
	if frameSize := e.encCtx.FrameSize(); frameSize > 0 &&
		e.encCtx.Codec().Capabilities()&ffmpeg.AVCodecCapVariableFrameSize == 0 {
		ffmpeg.AVBuffersinkSetFrameSize(e.bufferSinkCtx, uint(frameSize))
	}

	return nil
}

// ProgressCallback is called during encoding with progress updates
type ProgressCallback func(samplesProcessed, totalSamples int64)

// Encode performs the actual encoding with progress callbacks
func (e *Encoder) Encode(progressCb ProgressCallback) error {
	packet := ffmpeg.AVPacketAlloc()
	defer ffmpeg.AVPacketFree(&packet)

	outStream := e.ofmtCtx.Streams().Get(uintptr(e.outStreamIndex)) //nolint:gosec // outStreamIndex is set from AVFormatNewStream in openOutput

	for {
		// Observe cancellation before the next cgo call so Encode returns while
		// the AV contexts are still valid, ahead of any Close.
		if e.cancelled.Load() {
			return ErrCancelled
		}

		if _, err := ffmpeg.AVReadFrame(e.ifmtCtx, packet); err != nil {
			if errors.Is(err, ffmpeg.AVErrorEOF) {
				break
			}
			return fmt.Errorf("read frame failed: %w", err)
		}

		if packet.StreamIndex() != e.streamIndex {
			ffmpeg.AVPacketUnref(packet)
			continue
		}

		if _, err := ffmpeg.AVCodecSendPacket(e.decCtx, packet); err != nil {
			ffmpeg.AVPacketUnref(packet)
			return fmt.Errorf("send packet to decoder failed: %w", err)
		}

		ffmpeg.AVPacketUnref(packet)

		for {
			if e.cancelled.Load() {
				return ErrCancelled
			}

			if _, err := ffmpeg.AVCodecReceiveFrame(e.decCtx, e.decFrame); err != nil {
				if errors.Is(err, ffmpeg.EAgain) || errors.Is(err, ffmpeg.AVErrorEOF) {
					break
				}
				return fmt.Errorf("receive frame from decoder failed: %w", err)
			}

			e.samplesRead += int64(e.decFrame.NbSamples())
			if progressCb != nil && e.totalSamples > 0 {
				progressCb(e.samplesRead, e.totalSamples)
			}

			if _, err := ffmpeg.AVBuffersrcAddFrameFlags(e.bufferSrcCtx, e.decFrame, ffmpeg.AVBuffersrcFlagKeepRef); err != nil {
				return fmt.Errorf("failed to feed filter graph: %w", err)
			}

			if err := e.drainFilterGraph(outStream); err != nil {
				return err
			}

			ffmpeg.AVFrameUnref(e.decFrame)
		}
	}

	// Flush decoder
	if _, err := ffmpeg.AVCodecSendPacket(e.decCtx, nil); err != nil {
		return fmt.Errorf("flush decoder failed: %w", err)
	}

	for {
		if _, err := ffmpeg.AVCodecReceiveFrame(e.decCtx, e.decFrame); err != nil {
			if errors.Is(err, ffmpeg.EAgain) || errors.Is(err, ffmpeg.AVErrorEOF) {
				break
			}
			return fmt.Errorf("flush decoder receive failed: %w", err)
		}

		// Keep a ref (AVBuffersrcFlagKeepRef) because we reuse e.decFrame each
		// iteration and unref it ourselves below. The filter-graph flush feeds a
		// nil frame, so KEEP_REF is inapplicable there and it passes 0.
		if _, err := ffmpeg.AVBuffersrcAddFrameFlags(e.bufferSrcCtx, e.decFrame, ffmpeg.AVBuffersrcFlagKeepRef); err != nil {
			return fmt.Errorf("failed to feed filter graph: %w", err)
		}

		if err := e.drainFilterGraph(outStream); err != nil {
			return err
		}

		ffmpeg.AVFrameUnref(e.decFrame)
	}

	// Flush filter graph
	if _, err := ffmpeg.AVBuffersrcAddFrameFlags(e.bufferSrcCtx, nil, 0); err != nil {
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
	if _, err := ffmpeg.AVWriteTrailer(e.ofmtCtx); err != nil {
		return fmt.Errorf("write trailer failed: %w", err)
	}

	return nil
}

// drainFilterGraph reads filtered frames from the buffersink until EAGAIN or
// EOF, encoding each one. Callers feed the buffersrc before invoking this.
func (e *Encoder) drainFilterGraph(outStream *ffmpeg.AVStream) error {
	for {
		if _, err := ffmpeg.AVBuffersinkGetFrame(e.bufferSinkCtx, e.filteredFrame); err != nil {
			if errors.Is(err, ffmpeg.EAgain) || errors.Is(err, ffmpeg.AVErrorEOF) {
				break
			}
			return fmt.Errorf("failed to get filtered frame: %w", err)
		}

		if err := e.encodeFrame(e.filteredFrame, outStream); err != nil {
			return err
		}

		ffmpeg.AVFrameUnref(e.filteredFrame)
	}

	return nil
}

// encodeFrame encodes a single audio frame to MP3
func (e *Encoder) encodeFrame(frame *ffmpeg.AVFrame, outStream *ffmpeg.AVStream) error {
	// Stamp a monotonic PTS from the running sample counter so the filter's
	// reframing does not leave gaps the encoder would reject.
	frame.SetPts(e.nextPts)
	e.nextPts += int64(frame.NbSamples())

	if _, err := ffmpeg.AVCodecSendFrame(e.encCtx, frame); err != nil {
		return fmt.Errorf("send frame to encoder failed: %w", err)
	}

	return e.drainEncoder(outStream, "receive packet from encoder failed")
}

// drainEncoder reads encoded packets from the encoder and writes them to the
// output stream until the encoder needs more input or reaches EOF. recvErrCtx
// labels the receive-packet error so each caller keeps its existing wording.
func (e *Encoder) drainEncoder(outStream *ffmpeg.AVStream, recvErrCtx string) error {
	for {
		ffmpeg.AVPacketUnref(e.encPkt)

		if _, err := ffmpeg.AVCodecReceivePacket(e.encCtx, e.encPkt); err != nil {
			if errors.Is(err, ffmpeg.EAgain) || errors.Is(err, ffmpeg.AVErrorEOF) {
				break
			}
			return fmt.Errorf("%s: %w", recvErrCtx, err)
		}

		// Rescale packet timestamps from encoder to output stream time base.
		e.encPkt.SetStreamIndex(e.outStreamIndex)
		ffmpeg.AVPacketRescaleTs(e.encPkt, e.encCtx.TimeBase(), outStream.TimeBase())

		if _, err := ffmpeg.AVInterleavedWriteFrame(e.ofmtCtx, e.encPkt); err != nil {
			return fmt.Errorf("write frame failed: %w", err)
		}
	}

	return nil
}

// flushEncoder flushes remaining packets from the encoder
func (e *Encoder) flushEncoder(outStream *ffmpeg.AVStream) error {
	if _, err := ffmpeg.AVCodecSendFrame(e.encCtx, nil); err != nil {
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

	if e.filteredFrame != nil {
		ffmpeg.AVFrameFree(&e.filteredFrame)
	}
	if e.encPkt != nil {
		ffmpeg.AVPacketFree(&e.encPkt)
	}
	if e.decFrame != nil {
		ffmpeg.AVFrameFree(&e.decFrame)
	}
	if e.filterGraph != nil {
		ffmpeg.AVFilterGraphFree(&e.filterGraph)
	}
	if e.encCtx != nil {
		ffmpeg.AVCodecFreeContext(&e.encCtx)
	}
	if e.decCtx != nil {
		ffmpeg.AVCodecFreeContext(&e.decCtx)
	}
	if e.ofmtCtx != nil {
		if e.ofmtCtx.Oformat().Flags()&ffmpeg.AVFmtNofile == 0 && e.ofmtCtx.Pb() != nil {
			ffmpeg.AVIOClose(e.ofmtCtx.Pb())
			e.ofmtCtx.SetPb(nil)
		}
		ffmpeg.AVFormatFreeContext(e.ofmtCtx)
	}
	if e.ifmtCtx != nil {
		ffmpeg.AVFormatCloseInput(&e.ifmtCtx)
	}
}

// GetInputInfo returns information about the input audio
func (e *Encoder) GetInputInfo() (sampleRate, channels int, format string) {
	if e.decCtx == nil {
		return 0, 0, "unknown"
	}

	codecName := e.decCtx.Codec().Name()
	return e.decCtx.SampleRate(), e.decCtx.ChLayout().NbChannels(), codecName.String()
}

// GetDurationSecs returns the duration of the encoded audio in seconds.
// This is calculated from the samples processed during encoding, avoiding
// the need to re-open the output file. Should be called after Encode() completes.
func (e *Encoder) GetDurationSecs() int64 {
	if e.encCtx == nil {
		return 0
	}
	sampleRate := e.encCtx.SampleRate()
	if sampleRate <= 0 {
		return 0
	}
	// nextPts tracks total samples written to the encoder; round to nearest second
	return (e.nextPts + int64(sampleRate)/2) / int64(sampleRate)
}

// Bitrate returns the output MP3 bitrate in kbps for the configured channel
// mode: 192 for stereo, 112 for mono.
func (e *Encoder) Bitrate() int {
	if e.stereo {
		return StereoBitrate / 1000
	}
	return MonoBitrate / 1000
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
