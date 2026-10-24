//go:build cgo

package ffmpeg

import (
	"fmt"
	"image"
	"reflect"
	"testing"

	// Packages
	gomedia "github.com/mutablelogic/go-media"
)

// Test_sanitizeKey_000 checks that common tag key variants are mapped onto
// their canonical dc:/audio: key.
func Test_sanitizeKey_000(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"title", "dc:title"},
		{"TrackTitle", "dc:title"},
		{"artist", "dc:creator"},
		{"Album_Artist", "dc:creator"},
		{"publisher", "dc:publisher"},
		{"LABEL", "dc:publisher"},
		{"copyright", "dc:rights"},
		{"album", "audio:album"},
		{"AlbumTitle", "audio:album"},
		{"genre", "audio:genre"},
		{"music_genre", "audio:genre"},
		{"year", "audio:year"},
		{"originaldate", "audio:year"},
		{"track", "audio:track"},
		{"iTunes_CDDB_TrackNumber", "audio:track"},
	}
	for _, test := range tests {
		if got := sanitizeKey(test.in); got != test.want {
			t.Errorf("sanitizeKey(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// Test_sanitizeKey_musicbrainz checks that MusicBrainz's tags, as they're
// spelled in FLAC, MP3 and MP4 files, are mapped onto the musicbrainz
// namespace, named for MusicBrainz's entities
func Test_sanitizeKey_musicbrainz(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"MUSICBRAINZ_ALBUMID", "musicbrainz:release-id"},
		{"MusicBrainz Album Id", "musicbrainz:release-id"},
		{"MUSICBRAINZ_RELEASEGROUPID", "musicbrainz:release-group-id"},
		{"MusicBrainz Release Group Id", "musicbrainz:release-group-id"},
		{"MUSICBRAINZ_TRACKID", "musicbrainz:recording-id"},
		{"MusicBrainz Track Id", "musicbrainz:recording-id"},
		{"MUSICBRAINZ_RELEASETRACKID", "musicbrainz:track-id"},
		{"MusicBrainz Release Track Id", "musicbrainz:track-id"},
		{"MUSICBRAINZ_ARTISTID", "musicbrainz:artist-id"},
		{"MusicBrainz Album Artist Id", "musicbrainz:album-artist-id"},
		{"MUSICBRAINZ_ALBUMARTISTID", "musicbrainz:album-artist-id"},
		{"MusicBrainz Album Type", "musicbrainz:release-type"},
		{"RELEASETYPE", "musicbrainz:release-type"},
		{"MusicBrainz Album Status", "musicbrainz:release-status"},
		{"RELEASESTATUS", "musicbrainz:release-status"},
		{"MusicBrainz Album Release Country", "musicbrainz:release-country"},
		{"RELEASECOUNTRY", "musicbrainz:release-country"},
		{"MusicBrainz Work Id", "musicbrainz:work-id"},
		{"MUSICBRAINZ_NEWTHING", "musicbrainz:newthing"},
		{"musicbrainz", "audio:musicbrainz"},
	}
	for _, test := range tests {
		if got := sanitizeKey(test.in); got != test.want {
			t.Errorf("sanitizeKey(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// Test_sanitizeKey_acoustid checks that AcoustID's tags, as they're spelled
// in FLAC, MP3 and MP4 files, are mapped onto the acoustid namespace, and
// that the fingerprint is dropped
func Test_sanitizeKey_acoustid(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Acoustid Id", "acoustid:id"},
		{"ACOUSTID_ID", "acoustid:id"},
		{"acoustid_id", "acoustid:id"},
		{"Acoustid Fingerprint", ""},
		{"ACOUSTID_FINGERPRINT", ""},
	}
	for _, test := range tests {
		if got := sanitizeKey(test.in); got != test.want {
			t.Errorf("sanitizeKey(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// Test_sanitizeKey_001 checks that noisy/uninteresting tags are dropped.
func Test_sanitizeKey_001(t *testing.T) {
	noise := []string{
		"comment", "iTunNORM", "iTunSMPB", "TAGGING_TIME",
		"major_brand", "minor_version", "compatible_brands", "encoder",
	}
	for _, key := range noise {
		if got := sanitizeKey(key); got != "" {
			t.Errorf("sanitizeKey(%q) = %q, want \"\" (dropped)", key, got)
		}
	}
}

// tag is a tag of audio
type tag struct{ key, value string }

func (t tag) Key() string        { return t.key }
func (t tag) Value() string      { return t.value }
func (t tag) Bytes() []byte      { return nil }
func (t tag) Image() image.Image { return nil }
func (t tag) Any() any           { return t.value }

// entryMap returns the entries of the tags, by key
func entryMap(tags ...tag) map[string]string {
	in := make([]gomedia.Metadata, len(tags))
	for i, t := range tags {
		in[i] = t
	}
	result := map[string]string{}
	for _, e := range tagEntries(in) {
		result[e.ns+":"+e.key] = fmt.Sprint(e.value)
	}
	return result
}

func Test_tagEntries_000(t *testing.T) {
	// Only sort tags, as Apple Music downloads have, fill in the title,
	// artist and album
	got := entryMap(tag{"sort_name", "Jungle Love"}, tag{"sort_artist", "Time"}, tag{"sort_album", "Ice Cream Castle"})
	for key, want := range map[string]string{"dc:title": "Jungle Love", "dc:creator": "Time", "audio:album": "Ice Cream Castle", "audio:sort-name": "Jungle Love"} {
		if got[key] != want {
			t.Errorf("%s: expected %q, got %q", key, want, got[key])
		}
	}

	// A file's own title, artist and album are kept
	got = entryMap(tag{"title", "The Jungle Love"}, tag{"artist", "The Time"}, tag{"album", "Ice Cream Castle (Remastered)"}, tag{"sort_name", "Jungle Love"}, tag{"sort_artist", "Time"}, tag{"sort_album", "Ice Cream Castle"})
	for key, want := range map[string]string{"dc:title": "The Jungle Love", "dc:creator": "The Time", "audio:album": "Ice Cream Castle (Remastered)"} {
		if got[key] != want {
			t.Errorf("%s: expected %q, got %q", key, want, got[key])
		}
	}

	// The sort album artist is used without a sort artist, and an empty
	// sort tag isn't, and the first of a key wins
	got = entryMap(tag{"sort_album_artist", "Various Artists"}, tag{"sort_name", " "}, tag{"genre", "Funk"}, tag{"music_genre", "Pop"})
	if got["dc:creator"] != "Various Artists" || got["dc:title"] != "" || got["audio:genre"] != "Funk" {
		t.Errorf("unexpected %v", got)
	}
}

func Test_sanitizeKey_id3(t *testing.T) {
	// ID3 frames which ffmpeg doesn't name, and its names of the sort tags,
	// are named as an MP4 file's are
	for in, want := range map[string]string{
		"TSO2":        "audio:sort-album-artist",
		"TCM":         "audio:composer",
		"TMED":        "audio:media",
		"TORY":        "audio:year",
		"TSRC":        "audio:isrc",
		"ISRC":        "audio:isrc",
		"artist-sort": "audio:sort-artist",
		"album-sort":  "audio:sort-album",
		"title-sort":  "audio:sort-name",
	} {
		if got := sanitizeKey(in); got != want {
			t.Errorf("sanitizeKey(%q) = %q, want %q", in, got, want)
		}
	}

	// The sort tags as ffmpeg names them fill in the title and artist
	got := entryMap(tag{"title-sort", "Jungle Love"}, tag{"artist-sort", "Time"})
	if got["dc:title"] != "Jungle Love" || got["dc:creator"] != "Time" {
		t.Errorf("unexpected %v", got)
	}
}

func Test_tagEntries_isrc(t *testing.T) {
	// An ISRC which is valid is the identifier, without hyphens, and one
	// which isn't is as it is
	for _, test := range []struct {
		tag  tag
		key  string
		want string
	}{
		{tag{"TSRC", "GBAYE0601498"}, "dc:identifier", "GBAYE0601498"},
		{tag{"ISRC", "gb-aye-06-01498"}, "dc:identifier", "GBAYE0601498"},
		{tag{"isrc", "not an isrc"}, "audio:isrc", "not an isrc"},
	} {
		got := entryMap(test.tag)
		if got[test.key] != test.want || len(got) != 1 {
			t.Errorf("%v: expected %s %q, got %v", test.tag, test.key, test.want, got)
		}
	}
}

// Test_tagValues checks that tags are flags, numbers, counts, dates, lists
// and ISRCs, and otherwise as they are
func Test_tagValues(t *testing.T) {
	for _, test := range []struct {
		key, value string
		want       []entry
	}{
		{"audio:compilation", "1", []entry{{"audio", "compilation", true}}},
		{"audio:compilation", "0", []entry{{"audio", "compilation", false}}},
		{"audio:compilation", "yes", []entry{{"audio", "compilation", "yes"}}},
		{"audio:bpm", "104", []entry{{"audio", "bpm", 104}}},
		{"audio:track", "4/12", []entry{{"audio", "track", 4}, {"audio", "tracks", 12}}},
		{"audio:track", "4", []entry{{"audio", "track", 4}}},
		{"audio:track", "A1", []entry{{"audio", "track", "A1"}}},
		{"audio:disc", "1 / 2", []entry{{"audio", "disc", 1}, {"audio", "discs", 2}}},
		{"audio:year", "2005", []entry{{"audio", "year", 2005}}},
		{"audio:year", "1992-09-23", []entry{{"audio", "year", 1992}, {"dc", "date", "1992-09-23"}}},
		{"audio:year", "1992-09", []entry{{"audio", "year", 1992}, {"dc", "date", "1992-09"}}},
		{"audio:year", "2009-07-27T20:53:38Z", []entry{{"audio", "year", 2009}, {"dc", "date", "2009-07-27"}}},
		{"audio:year", "circa 1920", []entry{{"audio", "year", "circa 1920"}}},
		{"musicbrainz:artist-id", "a/b", []entry{{"musicbrainz", "artist-id", "a"}, {"musicbrainz", "artist-id", "b"}}},
		{"musicbrainz:artist-id", "a; b", []entry{{"musicbrainz", "artist-id", "a"}, {"musicbrainz", "artist-id", "b"}}},
		{"audio:isrc", "GBBLK0500180/USWB10002196", []entry{{"dc", "identifier", "GBBLK0500180"}, {"dc", "identifier", "USWB10002196"}}},
		{"audio:isrc", "GBBLK0500180/bad", []entry{{"dc", "identifier", "GBBLK0500180"}, {"audio", "isrc", "bad"}}},
		{"audio:genre", "Hip Hop/Rap", []entry{{"audio", "genre", "Hip Hop/Rap"}}},
	} {
		if got := tagValues(test.key, test.value); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s=%q: got %v, want %v", test.key, test.value, got, test.want)
		}
	}
}

// Test_tagEntries_001 checks that a tag's values of a key are all kept, but
// a key from an earlier tag isn't replaced by a later one
func Test_tagEntries_001(t *testing.T) {
	in := []gomedia.Metadata{
		tag{"MusicBrainz Artist Id", "a/b"},
		tag{"track", "4/12"},
		tag{"TRACKTOTAL", "13"},
		tag{"date", "1992-09-23"},
		tag{"originaldate", "1990"},
	}
	var got []string
	for _, e := range tagEntries(in) {
		got = append(got, fmt.Sprint(e.ns, ":", e.key, "=", e.value))
	}
	want := []string{"musicbrainz:artist-id=a", "musicbrainz:artist-id=b", "audio:track=4", "audio:tracks=12", "audio:year=1992", "dc:date=1992-09-23"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Test_sanitizeKey_002 checks that binary, repeated and meaningless tags are
// dropped, and that ID3's disc, disc subtitle, remixer and BPM frames are
// named
func Test_sanitizeKey_002(t *testing.T) {
	for in, want := range map[string]string{
		"id3v2_priv.WM/MediaClassPrimaryID":         "",
		"id3v2_priv.PeakValue":                      "",
		"UFIDhttp://www.cddb.com/id3/taginfo1.html": "",
		"TLEN":            "",
		"length":          "",
		"encoding_params": "",
		"iTunPGAP":        "",
		"rating":          "",
		"TFLT":            "",
		"XXX":             "",
		"TPA":             "audio:disc",
		"TPOS":            "audio:disc",
		"disc":            "audio:disc",
		"TSST":            "audio:discsubtitle",
		"TPE4":            "audio:remixer",
		"TBPM":            "audio:bpm",
		"TRACKTOTAL":      "audio:tracks",
		"DISCTOTAL":       "audio:discs",
	} {
		if got := sanitizeKey(in); got != want {
			t.Errorf("sanitizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}
