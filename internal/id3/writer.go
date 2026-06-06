package id3

import (
	"fmt"

	"github.com/bogem/id3v2/v2"
)

// TagInfo holds all the information needed to write ID3v2 tags
type TagInfo struct {
	EpisodeNumber string
	Title         string
	Artist        string // Optional: defaults to empty if not provided
	Album         string // Optional: defaults to empty if not provided
	Date          string // Optional: Format: "YYYY-MM"
	Comment       string // Optional: defaults to empty if not provided
	CoverArtData  []byte // Optional: pre-processed cover art bytes
}

// WriteTags writes ID3v2.4 tags to an MP3 file
func WriteTags(mp3Path string, info TagInfo) error {
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: false})
	if err != nil {
		return fmt.Errorf("failed to open MP3 for tagging: %w", err)
	}
	defer tag.Close()

	tag.SetVersion(4)

	// TIT2: Title = "Episode: Title"
	titleFrame := fmt.Sprintf("%s: %s", info.EpisodeNumber, info.Title)
	tag.SetTitle(titleFrame)

	// TALB: Album (only if provided)
	if info.Album != "" {
		tag.SetAlbum(info.Album)
	}

	// TRCK: Track number
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), tag.DefaultEncoding(), info.EpisodeNumber)

	// TPE1: Artist (only if provided)
	if info.Artist != "" {
		tag.SetArtist(info.Artist)
	}

	// TDRC: Recording date (year and month)
	if info.Date != "" {
		tag.AddTextFrame(tag.CommonID("Recording time"), tag.DefaultEncoding(), info.Date)
	}

	// COMM: Comment (only if provided)
	if info.Comment != "" {
		commentFrame := id3v2.CommentFrame{
			Encoding:    id3v2.EncodingUTF8,
			Language:    "eng",
			Description: "",
			Text:        info.Comment,
		}
		tag.AddCommentFrame(commentFrame)
	}

	// APIC: Cover art
	if len(info.CoverArtData) > 0 {
		addCoverArtData(tag, info.CoverArtData, info.Artist)
	}

	if err := tag.Save(); err != nil {
		return fmt.Errorf("failed to save ID3 tags: %w", err)
	}

	return nil
}

// addCoverArtData adds pre-processed cover artwork as an APIC frame
// The description defaults to "{artist} Logo"
func addCoverArtData(tag *id3v2.Tag, artwork []byte, artist string) {
	var desc string
	if artist != "" {
		desc = fmt.Sprintf("%s Logo", artist)
	}

	pic := id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    "image/png",
		PictureType: id3v2.PTFrontCover,
		Description: desc,
		Picture:     artwork,
	}

	tag.AddAttachedPicture(pic)
}
