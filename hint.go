package metadata

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The hints of what to read of an input next (see HintMetadata), which a
// handler returns when it reads an input which has them
const (
	// HintMarkdown is a hint that the input's content can be read as
	// Markdown, such as a document's text, to index it
	HintMarkdown = "markdown"

	// HintArtwork is a hint that the input has artwork, such as a cover, or
	// a thumbnail, which can be extracted
	HintArtwork = "artwork"

	// HintOCR is a hint that the input has pages which are images, such as
	// a scanned PDF, whose text can be recognised
	HintOCR = "ocr"

	// HintTranscription is a hint that the input is audio which isn't known
	// to be music, which may be speech, which can be transcribed
	HintTranscription = "transcription"

	// The hints that the input is a track of a release, such as a song, by
	// the primary type of the release, as MusicBrainz defines it, which can
	// be grouped with the release's other tracks. A track with an album's
	// name, and no release type, is a track of an album
	HintAlbum     = "album"
	HintSingle    = "single"
	HintEP        = "ep"
	HintBroadcast = "broadcast"
	HintOther     = "other"

	// HintTV is a hint that the input is an episode of a TV show, which can
	// be grouped with the show's other episodes
	HintTV = "tv"

	// HintEvent is a hint that the input has events, such as a calendar's,
	// with when they are, which can be indexed by time
	HintEvent = "event"

	// HintGeo is a hint that the input has places, such as where a photo
	// was taken, or a contact's address, which can be indexed by location
	HintGeo = "geo"
)
