package id3

// TagInfo carries episode metadata from the CLI workflows to the encoder, which
// writes it as muxer-native tags during encoding.
type TagInfo struct {
	EpisodeNumber string
	Title         string
	Artist        string // Optional: defaults to empty if not provided
	Album         string // Optional: defaults to empty if not provided
	Date          string // Optional: Format: "YYYY-MM"
	Comment       string // Optional: defaults to empty if not provided
}
