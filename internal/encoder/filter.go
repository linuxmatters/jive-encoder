package encoder

import (
	"fmt"

	"github.com/linuxmatters/ffmpeg-statigo"
)

// initFilter sets up the audio filter graph for resampling and frame buffering.
func (e *Encoder) initFilter() error {
	e.filter.graph = ffmpeg.AVFilterGraphAlloc()
	if e.filter.graph == nil {
		return fmt.Errorf("failed to allocate filter graph")
	}

	bufferSrc := ffmpeg.AVFilterGetByName(ffmpeg.GlobalCStr("abuffer"))
	bufferSink := ffmpeg.AVFilterGetByName(ffmpeg.GlobalCStr("abuffersink"))
	if bufferSrc == nil || bufferSink == nil {
		return fmt.Errorf("abuffer or abuffersink filter not found")
	}

	layoutPtr := ffmpeg.AllocCStr(64)
	defer layoutPtr.Free()
	if _, err := ffmpeg.AVChannelLayoutDescribe(e.input.codec.ChLayout(), layoutPtr, 64); err != nil {
		return fmt.Errorf("failed to describe channel layout: %w", err)
	}

	pktTimebase := e.input.codec.PktTimebase()
	args := fmt.Sprintf(
		"time_base=%d/%d:sample_rate=%d:sample_fmt=%s:channel_layout=%s",
		pktTimebase.Num(), pktTimebase.Den(),
		e.input.codec.SampleRate(),
		ffmpeg.AVGetSampleFmtName(e.input.codec.SampleFmt()).String(),
		layoutPtr.String(),
	)

	argsC := ffmpeg.ToCStr(args)
	defer argsC.Free()

	if _, err := ffmpeg.AVFilterGraphCreateFilter(
		&e.filter.src,
		bufferSrc,
		ffmpeg.GlobalCStr("in"),
		argsC,
		nil,
		e.filter.graph,
	); err != nil {
		return fmt.Errorf("failed to create buffer source: %w", err)
	}

	if _, err := ffmpeg.AVFilterGraphCreateFilter(
		&e.filter.sink,
		bufferSink,
		ffmpeg.GlobalCStr("out"),
		nil,
		nil,
		e.filter.graph,
	); err != nil {
		return fmt.Errorf("failed to create buffer sink: %w", err)
	}

	outputs := ffmpeg.AVFilterInoutAlloc()
	inputs := ffmpeg.AVFilterInoutAlloc()
	if outputs == nil || inputs == nil {
		ffmpeg.AVFilterInoutFree(&outputs)
		ffmpeg.AVFilterInoutFree(&inputs)
		return fmt.Errorf("failed to allocate filter endpoints")
	}
	defer ffmpeg.AVFilterInoutFree(&outputs)
	defer ffmpeg.AVFilterInoutFree(&inputs)

	outputs.SetName(ffmpeg.ToCStr("in"))
	outputs.SetFilterCtx(e.filter.src)
	outputs.SetPadIdx(0)
	outputs.SetNext(nil)

	inputs.SetName(ffmpeg.ToCStr("out"))
	inputs.SetFilterCtx(e.filter.sink)
	inputs.SetPadIdx(0)
	inputs.SetNext(nil)

	if err := e.parseFilterGraph(&inputs, &outputs); err != nil {
		return err
	}

	if _, err := ffmpeg.AVFilterGraphConfig(e.filter.graph, nil); err != nil {
		return fmt.Errorf("failed to configure filter graph: %w", err)
	}

	e.setFilterFrameSize()
	return nil
}

func (e *Encoder) parseFilterGraph(inputs, outputs **ffmpeg.AVFilterInOut) error {
	channelLayout := "mono"
	if e.stereo {
		channelLayout = "stereo"
	}

	sampleFmtName := ffmpeg.AVGetSampleFmtName(e.preset.sampleFmt).String()
	filterSpec := fmt.Sprintf("aresample=%d:async=1,aformat=sample_fmts=%s:sample_rates=%d:channel_layouts=%s",
		e.preset.sampleRate, sampleFmtName, e.preset.sampleRate, channelLayout)

	filterSpecC := ffmpeg.ToCStr(filterSpec)
	defer filterSpecC.Free()

	if _, err := ffmpeg.AVFilterGraphParsePtr(e.filter.graph, filterSpecC, inputs, outputs, nil); err != nil {
		return fmt.Errorf("failed to parse filter graph: %w", err)
	}

	return nil
}

func (e *Encoder) setFilterFrameSize() {
	// Fix the buffer-sink frame size to the encoder's required frame size so the
	// filter delivers exactly the frames the encoder expects (LAME 1152, native
	// AAC 1024, libopus its own). Encoders that accept variable-size frames
	// advertise AV_CODEC_CAP_VARIABLE_FRAME_SIZE and need no fixed size.
	// openOutput runs before initFilter, so FrameSize is populated here.
	if frameSize := e.output.codec.FrameSize(); frameSize > 0 &&
		e.output.codec.Codec().Capabilities()&ffmpeg.AVCodecCapVariableFrameSize == 0 {
		ffmpeg.AVBuffersinkSetFrameSize(e.filter.sink, uint(frameSize))
	}
}
