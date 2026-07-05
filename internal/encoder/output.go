package encoder

import (
	"fmt"
	"maps"
	"strconv"

	"github.com/linuxmatters/ffmpeg-statigo"
)

// openOutput creates the output file for the preset's format and sets up the
// output-side encoder, muxer, metadata, and optional attached-picture stream.
func (e *Encoder) openOutput() error {
	namePtr := ffmpeg.ToCStr(e.outputPath)
	defer namePtr.Free()

	if _, err := ffmpeg.AVFormatAllocOutputContext2(&e.output.format, nil, nil, namePtr); err != nil {
		return fmt.Errorf("failed to create output context: %w", err)
	}

	encoder, err := e.findOutputEncoder()
	if err != nil {
		return err
	}

	outStream := ffmpeg.AVFormatNewStream(e.output.format, encoder)
	if outStream == nil {
		return fmt.Errorf("failed to create output stream")
	}
	e.output.audioStreamIndex = outStream.Index()

	if err := e.configureOutputCodec(encoder, outStream); err != nil {
		return err
	}

	// Formats without the NOFILE flag need an explicit AVIO output handle.
	if e.output.format.Oformat().Flags()&ffmpeg.AVFmtNofile == 0 {
		var pb *ffmpeg.AVIOContext
		if _, err := ffmpeg.AVIOOpen(&pb, e.output.format.Url(), ffmpeg.AVIOFlagWrite); err != nil {
			return fmt.Errorf("failed to open output file: %w", err)
		}
		e.output.format.SetPb(pb)
	}

	if err := e.setMuxerMetadata(); err != nil {
		return err
	}

	// Add the attached-picture stream after the audio stream so audio keeps
	// index 0 (audioStreamIndex). Opus is not cover-capable and absent cover
	// bytes mean no second stream, leaving the audio-only path unchanged.
	if e.preset.coverCapable && len(e.coverArt) > 0 {
		if err := e.addCoverStream(); err != nil {
			return err
		}
	}

	if err := e.writeHeader(); err != nil {
		return err
	}

	// Write the cover picture immediately after the header so the muxer carries
	// it as the attached picture before any audio packet.
	if e.output.coverStreamIndex >= 0 {
		if err := e.writeCoverPacket(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) findOutputEncoder() (*ffmpeg.AVCodec, error) {
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
		return nil, fmt.Errorf("%s encoder not found", e.preset.name)
	}

	return encoder, nil
}

func (e *Encoder) configureOutputCodec(encoder *ffmpeg.AVCodec, outStream *ffmpeg.AVStream) error {
	e.output.codec = ffmpeg.AVCodecAllocContext3(encoder)
	if e.output.codec == nil {
		return fmt.Errorf("failed to allocate encoder context")
	}

	if e.stereo {
		e.output.codec.SetBitRate(int64(e.preset.stereoBitrate))
		ffmpeg.AVChannelLayoutDefault(e.output.codec.ChLayout(), 2)
	} else {
		e.output.codec.SetBitRate(int64(e.preset.monoBitrate))
		ffmpeg.AVChannelLayoutDefault(e.output.codec.ChLayout(), 1)
	}

	e.output.codec.SetSampleRate(e.preset.sampleRate)
	e.output.codec.SetSampleFmt(e.preset.sampleFmt)

	tb := &ffmpeg.AVRational{}
	tb.SetNum(1)
	tb.SetDen(e.output.codec.SampleRate())
	e.output.codec.SetTimeBase(tb)

	opts, err := e.encoderOptions()
	if err != nil {
		return err
	}
	defer ffmpeg.AVDictFree(&opts)

	if _, err := ffmpeg.AVCodecOpen2(e.output.codec, encoder, &opts); err != nil {
		return fmt.Errorf("failed to open encoder: %w", err)
	}

	if _, err := ffmpeg.AVCodecParametersFromContext(outStream.Codecpar(), e.output.codec); err != nil {
		return fmt.Errorf("failed to copy encoder parameters: %w", err)
	}

	outStream.SetTimeBase(e.output.codec.TimeBase())
	return nil
}

func (e *Encoder) encoderOptions() (*ffmpeg.AVDictionary, error) {
	encoderOpts := e.preset.encoderOpts
	if e.preset.lowpassHz > 0 {
		encoderOpts = make(map[string]string, len(e.preset.encoderOpts)+1)
		maps.Copy(encoderOpts, e.preset.encoderOpts)
		encoderOpts["cutoff"] = strconv.Itoa(e.preset.lowpassHz)
	}

	var opts *ffmpeg.AVDictionary
	if err := setDictOptions(&opts, encoderOpts); err != nil {
		return nil, err
	}

	return opts, nil
}

// setDictOptions sets each key/value pair from opts into dict, freeing the C
// strings for both key and value after every entry. On the first failure it
// frees dict and returns the error labelled with the offending key. The caller
// owns dict and remains responsible for freeing it (or transferring ownership).
func setDictOptions(dict **ffmpeg.AVDictionary, opts map[string]string) error {
	for key, val := range opts {
		keyPtr := ffmpeg.ToCStr(key)
		valPtr := ffmpeg.ToCStr(val)
		_, err := ffmpeg.AVDictSet(dict, keyPtr, valPtr, 0)
		keyPtr.Free()
		valPtr.Free()
		if err != nil {
			ffmpeg.AVDictFree(dict)
			return fmt.Errorf("failed to set option %s: %w", key, err)
		}
	}

	return nil
}

func (e *Encoder) writeHeader() error {
	var muxerOpts *ffmpeg.AVDictionary
	defer ffmpeg.AVDictFree(&muxerOpts)

	// id3v2_version is an mp3-muxer-private option, so it goes through the
	// WriteHeader options dict, not the format-context metadata. Other muxers
	// ignore it.
	if e.preset.name == "mp3" {
		if err := setDictOptions(&muxerOpts, map[string]string{"id3v2_version": "4"}); err != nil {
			return err
		}
	}

	if _, err := ffmpeg.AVFormatWriteHeader(e.output.format, &muxerOpts); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
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

	opts := make(map[string]string, len(tags))
	for _, tag := range tags {
		opts[tag.Key] = tag.Value
	}

	var dict *ffmpeg.AVDictionary
	if err := setDictOptions(&dict, opts); err != nil {
		return err
	}

	e.output.format.SetMetadata(dict)
	return nil
}
