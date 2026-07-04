package encoder

import (
	"bytes"
	"fmt"
	"image/png"
	"unsafe"

	"github.com/linuxmatters/ffmpeg-statigo"
)

// addCoverStream creates the attached-picture stream that carries the scaled
// PNG cover. It is added after the audio stream, so the audio stream keeps
// index 0. The packet itself is written after AVFormatWriteHeader by
// writeCoverPacket.
func (e *Encoder) addCoverStream() error {
	coverStream := ffmpeg.AVFormatNewStream(e.output.format, nil)
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

	e.output.coverStreamIndex = coverStream.Index()
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

	pkt.SetStreamIndex(e.output.coverStreamIndex)
	pkt.SetFlags(pkt.Flags() | ffmpeg.AVPktFlagKey)

	if _, err := ffmpeg.AVInterleavedWriteFrame(e.output.format, pkt); err != nil {
		return fmt.Errorf("failed to write cover packet: %w", err)
	}

	return nil
}
