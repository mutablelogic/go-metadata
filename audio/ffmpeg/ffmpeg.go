//go:build cgo

// Package ffmpeg registers handlers of audio, such as MP3, AAC, FLAC and WAV
// files, which return their tags, such as their title, artists and album,
// their MusicBrainz identifiers, their duration and codec, and their cover
// art as artwork, which use ffmpeg, so need cgo
package ffmpeg

import (
	"context"
	"io"
	"maps"
	"mime"
	"regexp"
	"slices"
	"strconv"
	"strings"

	// Packages
	gomedia "github.com/mutablelogic/go-media"
	profile "github.com/mutablelogic/go-media/profile/schema"
	reader "github.com/mutablelogic/go-media/reader"
	ff "github.com/mutablelogic/go-media/sys/ffmpeg80"
	metadata "github.com/mutablelogic/go-metadata"
	playlist "github.com/mutablelogic/go-metadata/audio/playlist"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// entry is a tag of audio, by its namespace and key, whose value is a
// string, or a number or flag (see tagValues)
type entry struct {
	ns, key string
	value   any
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// The namespaces of the metadata of audio: its tags, duration and codec,
	// and its MusicBrainz and AcoustID identifiers
	AudioNamespace       = "audio"
	MusicBrainzNamespace = "musicbrainz"
	AcoustIDNamespace    = "acoustid"

	// recordingIDKey is the key of a recording's MusicBrainz identifier
	recordingIDKey = "recording-id"

	// isrcKey is the key of the ISRC of a recording, which is its
	// identifier, dc:identifier, when it's valid
	isrcKey = "audio:isrc"
)

var (
	reContentType = regexp.MustCompile(`^audio/.+$`)

	// reNotWord matches what isn't a letter or a digit in the name of a tag
	reNotWord = regexp.MustCompile(`\W+`)

	// audioTypes are the content types of audio, by their extension
	audioTypes = map[string]string{
		".m4a":  "audio/mp4",
		".flac": "audio/flac",
	}

	// flagKeys are the keys of the tags of audio which are flags, which are
	// true or false
	flagKeys = map[string]bool{"audio:compilation": true}

	// numberKeys are the keys of the tags of audio which are numbers
	numberKeys = map[string]bool{"audio:tracks": true, "audio:discs": true, "audio:bpm": true}

	// countKeys are the keys of the tags of audio which are a number, and
	// often the total, such as 4/12, by the key of the total
	countKeys = map[string]string{"audio:track": "tracks", "audio:disc": "discs"}

	// listKeys are the keys of the tags which are lists of identifiers, which
	// are joined with a slash or semicolon, such as the artists of a
	// recording with more than one
	listKeys = map[string]bool{
		"musicbrainz:artist-id":          true,
		"musicbrainz:album-artist-id":    true,
		"musicbrainz:original-artist-id": true,
		"musicbrainz:work-id":            true,
	}

	// reListSeparator matches what's between the identifiers of a list, and
	// between ISRCs
	reListSeparator = regexp.MustCompile(`\s*[/;,]\s*`)

	// reDate matches a date, which is a year, and maybe a month and a day,
	// and maybe a time, such as 1992-09-23
	reDate = regexp.MustCompile(`^(\d{4})(-\d{2}(-\d{2})?)?([T ].*)?$`)

	// reISRC matches an International Standard Recording Code, which is a
	// country, a registrant, a year and a number, such as GBAYE0601498
	reISRC = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}[0-9]{7}$`)

	// sortTags are the keys which a sort tag fills in, when they're missing
	sortTags = []struct{ key, sort string }{
		{"dc:title", "audio:sort-name"},
		{"dc:creator", "audio:sort-artist"},
		{"dc:creator", "audio:sort-album-artist"},
		{"audio:album", "audio:sort-album"},
	}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// ffmpeg logs to stderr, which isn't wanted
	ff.AVUtil_log_set_level(ff.AV_LOG_QUIET)

	for ext, contentType := range audioTypes {
		if err := mime.AddExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	metadata.AddHandler(reContentType, "ffmpeg", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		// A playlist, which is text, isn't read by ffmpeg
		if playlist.ContentTypes.MatchString(o.ContentType()) {
			return nil, nil
		}
		kind := metadata.KindMetadata(dc.KindAudio)
		rd, err := reader.NewReader(r)
		if err != nil {
			// Audio which can't be read has no more metadata, which isn't
			// an error, as another handler may read it, but has a note
			return append(kind, meta.New(AudioNamespace, "note", "The audio can't be read, so it may be damaged or incomplete.")), nil
		}
		defer rd.Close()

		// The tags, and the recording's MusicBrainz identifier, which an
		// MP3 has in its ID3 tag's UFID frame, rather than a tag ffmpeg
		// returns
		entries := tagEntries(rd.Metadata())
		if !slices.ContainsFunc(entries, func(e entry) bool { return e.ns == MusicBrainzNamespace && e.key == recordingIDKey }) {
			if ra, ok := r.(io.ReaderAt); ok {
				if id := ufidRecordingID(ra); id != "" {
					entries = append(entries, entry{MusicBrainzNamespace, recordingIDKey, id})
				}
			}
		}
		var result []metadata.Metadata
		for _, e := range entries {
			if m := meta.New(e.ns, e.key, e.value); m != nil {
				result = append(result, m)
			}
		}
		if d := rd.Duration(); d > 0 {
			result = append(result, meta.New(AudioNamespace, "duration", d))
		}
		// Audio which MusicBrainz identifies is a recording
		if slices.ContainsFunc(entries, func(e entry) bool { return e.ns == MusicBrainzNamespace && e.key == recordingIDKey }) {
			kind = metadata.KindMetadata(dc.KindRecording)
		}
		return append(append(result, streamMetadata(rd.Streams())...), kind...), nil
	}, AudioNamespace, MusicBrainzNamespace, AcoustIDNamespace, dc.Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// tagEntries returns the tags of audio, mapped onto their keys (see
// sanitizeKey), and their values (see tagValues), without those which are
// empty or aren't metadata. A key is from the first tag which has it, and
// the title, artist and album are from the tags which are for sorting, such
// as sort_name, when the file doesn't have them, as music downloaded from
// Apple Music only has the sort tags.
func tagEntries(tags []gomedia.Metadata) []entry {
	var result []entry
	values := make(map[string]any)
	for _, tag := range tags {
		key, value := sanitizeKey(tag.Key()), strings.TrimSpace(tag.Value())
		if key == "" || value == "" {
			continue
		}

		// A tag can have more than one value of a key, such as a list of
		// identifiers, but not of a key from an earlier tag
		added := make(map[string]any)
		for _, e := range tagValues(key, value) {
			key := e.ns + ":" + e.key
			if _, exists := values[key]; exists {
				continue
			}
			result = append(result, e)
			if _, exists := added[key]; !exists {
				added[key] = e.value
			}
		}
		maps.Copy(values, added)
	}
	for _, tag := range sortTags {
		if _, exists := values[tag.key]; exists {
			continue
		}
		if sorted, exists := values[tag.sort]; exists {
			ns, name, _ := strings.Cut(tag.key, ":")
			result = append(result, entry{ns, name, sorted})
			values[tag.key] = sorted
		}
	}
	return result
}

// tagValues returns the values of a tag with a key, which are:
//   - a flag, such as audio:compilation, which is 1 or 0, as a bool;
//   - a number, such as audio:bpm, as an int;
//   - a track or disc number and its total, such as 4/12, as audio:track
//     and audio:tracks;
//   - a date, such as 1992-09-23, as its year, audio:year, and as
//     dc:date, when it's more than a year;
//   - a list of identifiers, such as the MusicBrainz identifiers of more
//     than one artist, as more than one value;
//   - ISRCs, of which those which are valid identify the recording, as
//     dc:identifier.
//
// Otherwise, or when it isn't what it should be, it's the value as it is.
func tagValues(key, value string) []entry {
	ns, name, _ := strings.Cut(key, ":")
	switch {
	case flagKeys[key]:
		if flag, err := strconv.ParseBool(value); err == nil {
			return []entry{{ns, name, flag}}
		}
	case numberKeys[key]:
		if n, err := strconv.Atoi(value); err == nil {
			return []entry{{ns, name, n}}
		}
	case countKeys[key] != "":
		number, total, _ := strings.Cut(value, "/")
		if n, err := strconv.Atoi(strings.TrimSpace(number)); err == nil {
			result := []entry{{ns, name, n}}
			if t, err := strconv.Atoi(strings.TrimSpace(total)); err == nil {
				result = append(result, entry{ns, countKeys[key], t})
			}
			return result
		}
	case key == "audio:year":
		if match := reDate.FindStringSubmatch(value); match != nil {
			year, _ := strconv.Atoi(match[1])
			result := []entry{{ns, name, year}}
			if match[2] != "" {
				result = append(result, entry{dc.Namespace, dc.Date, match[1] + match[2]})
			}
			return result
		}
	case listKeys[key]:
		var result []entry
		for _, id := range reListSeparator.Split(value, -1) {
			if id != "" {
				result = append(result, entry{ns, name, id})
			}
		}
		return result
	case key == isrcKey:
		var result []entry
		for _, value := range reListSeparator.Split(value, -1) {
			if isrc, ok := normalizeISRC(value); ok {
				result = append(result, entry{dc.Namespace, dc.Identifier, isrc})
			} else if value != "" {
				result = append(result, entry{ns, name, value})
			}
		}
		return result
	}
	return []entry{{ns, name, value}}
}

// normalizeISRC returns an ISRC without the hyphens or spaces it's often
// written with, in upper case, such as GBAYE0601498 for GB-AYE-06-01498, or
// false when it isn't one
func normalizeISRC(value string) (string, bool) {
	isrc := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(value)))
	return isrc, reISRC.MatchString(isrc)
}

// streamMetadata returns the codec, sample rate, number of channels and bit
// rate of the first audio stream, by the index of the streams, which don't
// include cover art
func streamMetadata(streams map[int]profile.Profile) []metadata.Metadata {
	for _, index := range slices.Sorted(maps.Keys(streams)) {
		par := streams[index].Par()
		if par == nil || par.CodecType() != ff.AVMEDIA_TYPE_AUDIO {
			continue
		}
		result := []metadata.Metadata{meta.New(AudioNamespace, "codec", par.CodecID().Name())}
		if rate := par.SampleRate(); rate > 0 {
			result = append(result, meta.New(AudioNamespace, "sample-rate", rate))
		}
		if layout := par.ChannelLayout(); layout.NumChannels() > 0 {
			result = append(result, meta.New(AudioNamespace, "channels", layout.NumChannels()))
		}
		if rate := par.BitRate(); rate > 0 {
			result = append(result, meta.New(AudioNamespace, "bit-rate", rate))
		}
		return result
	}
	return nil
}

// sanitizeKey normalizes a raw ffmpeg/format tag key into a
// "namespace:name" metadata key, mapping common variant spellings onto a
// canonical dc:/audio: key, and dropping noisy or uninteresting tags
// (returning "").
func sanitizeKey(key string) string {
	key = strings.ToLower(key)

	// Replace any non-alphanumeric characters with dashes
	key = reNotWord.ReplaceAllString(key, "-")
	key = strings.ReplaceAll(key, "_", "-")
	key = strings.Trim(key, "-")

	// The private frames of ID3 tags, which are binary, and their unique
	// file identifiers, other than MusicBrainz's (see ufidRecordingID)
	if strings.HasPrefix(key, "id3v2-priv-") || strings.HasPrefix(key, "ufid") {
		return ""
	}

	switch key {
	// Noisy or uninteresting tags
	case "eitunnorm", "tagging-time", "accurateripdiscid", "accurateripresult", "comment", "id3v1-comment",
		"itunes-cddb-1", "itunmovi", "itunnorm", "itunsmpb", "gapless-playback", "itunextc",
		"compatible-brands", "itunes-cddb-ids", "account-id", "major-brand", "minor-version", "encoder",
		"itunpgap", "account-type", "media-type", "rating", "episode-sort", "season-number",
		"encoding-params", "tflt", "xxx":
		return ""

	// The length of the audio, in milliseconds or minutes and seconds, which
	// its duration is
	case "tlen", "length":
		return ""

	// Canonical mappings
	case "title", "tracktitle":
		return "dc:title"
	case "artists", "artist", "album-artist":
		return "dc:creator"
	case "publisher", "label", "organization":
		return "dc:publisher"
	case "copyright":
		return "dc:rights"
	case "album", "albumtitle":
		return "audio:album"
	case "genre", "music-genre":
		return "audio:genre"
	case "originalyear", "year", "date", "originaldate", "tdor":
		return "audio:year"
	case "itunes-cddb-tracknumber", "track", "tracknumber":
		return "audio:track"
	case "totaltracks", "tracktotal":
		return "audio:tracks"
	case "disc", "discnumber":
		return "audio:disc"
	case "totaldiscs", "disctotal":
		return "audio:discs"
	case "bpm", "tmpo":
		return "audio:bpm"

	// AcoustID's identifier, which MusicBrainz Picard writes, as Acoustid Id
	// in an MP3 or MP4 file, and ACOUSTID_ID in a FLAC file, and not its
	// fingerprint, which is only of use to look up the identifier
	case "acoustid-id":
		return "acoustid:id"
	case "acoustid-fingerprint":
		return ""
	}

	// ID3 frames which ffmpeg doesn't name, and its names of the sort tags,
	// which are named as an MP4 file's are
	if key, found := id3Keys[key]; found {
		return key
	}

	// MusicBrainz's identifiers, and the details of the release
	if key, found := musicbrainzKey(key); found {
		return key
	}
	return "audio:" + key
}

// id3Keys are the keys of ID3 frames which ffmpeg doesn't name, such as
// TSO2, and of the sort tags as ffmpeg names them, such as artist-sort,
// which are named as an MP4 file's are, such as sort_artist
var id3Keys = map[string]string{
	"tso2":              "audio:sort-album-artist",
	"tsoc":              "audio:sort-composer",
	"tcm":               "audio:composer",
	"tcmp":              "audio:compilation",
	"tmed":              "audio:media",
	"tory":              "audio:year",
	"tsrc":              "audio:isrc",
	"tpa":               "audio:disc",
	"tpos":              "audio:disc",
	"tsst":              "audio:discsubtitle",
	"tpe4":              "audio:remixer",
	"tbpm":              "audio:bpm",
	"isrc":              "audio:isrc",
	"artist-sort":       "audio:sort-artist",
	"album-sort":        "audio:sort-album",
	"title-sort":        "audio:sort-name",
	"album-artist-sort": "audio:sort-album-artist",
	"composer-sort":     "audio:sort-composer",
}

// musicbrainzTags are the keys of MusicBrainz's tags, by their names without
// separators, which are named for MusicBrainz's entities: an album is a
// release, and a track is a recording, and a release's track is a track.
// Tags are spelled differently in each container, such as
// MUSICBRAINZ_ALBUMID in a FLAC file, and MusicBrainz Album Id in an MP3 or
// MP4 file, and the details of a release don't have the prefix in a FLAC
// file, such as RELEASETYPE for MusicBrainz Album Type.
var musicbrainzTags = map[string]string{
	"albumid":             "musicbrainz:release-id",
	"releasegroupid":      "musicbrainz:release-group-id",
	"trackid":             "musicbrainz:recording-id",
	"releasetrackid":      "musicbrainz:track-id",
	"artistid":            "musicbrainz:artist-id",
	"albumartistid":       "musicbrainz:album-artist-id",
	"workid":              "musicbrainz:work-id",
	"discid":              "musicbrainz:disc-id",
	"originalalbumid":     "musicbrainz:original-release-id",
	"originalartistid":    "musicbrainz:original-artist-id",
	"albumtype":           "musicbrainz:release-type",
	"albumstatus":         "musicbrainz:release-status",
	"albumreleasecountry": "musicbrainz:release-country",
}

// musicbrainzDetails are the details of a release in a FLAC file, which
// don't have MusicBrainz's prefix, by their names
var musicbrainzDetails = map[string]string{
	"releasetype":    "musicbrainz:release-type",
	"releasestatus":  "musicbrainz:release-status",
	"releasecountry": "musicbrainz:release-country",
}

// musicbrainzKey returns the key of a MusicBrainz tag, whose key is
// normalized, or false when it isn't one. A tag with MusicBrainz's prefix
// which isn't known keeps its name.
func musicbrainzKey(key string) (string, bool) {
	if key, found := musicbrainzDetails[key]; found {
		return key, true
	}
	name, found := strings.CutPrefix(key, "musicbrainz-")
	if !found || name == "" {
		return "", false
	} else if key, found := musicbrainzTags[strings.ReplaceAll(name, "-", "")]; found {
		return key, true
	}
	return "musicbrainz:" + name, true
}
