// Package playlist registers a handler of playlists, M3U, HLS and PLS
// files, which returns their format, title, entries and duration
package playlist

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	charmap "golang.org/x/text/encoding/charmap"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// playlist is an M3U, HLS or PLS playlist
type playlist struct {
	format  string
	title   string
	entries []playlistEntry

	// The playlist is longer than the most which is read, so its entries
	// can't all be counted
	truncated bool
}

// playlistEntry is an entry in a playlist, with its location, and its title
// and duration when the playlist has them
type playlistEntry struct {
	location string
	title    string
	duration time.Duration
	timed    bool
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// maxPlaylistSize is the most of a playlist which is read
const maxPlaylistSize = 4 << 20

// maxPlaylistEntries is the most titles and locations which are returned, as
// a playlist, such as of TV channels, can be long. They're all counted.
const maxPlaylistEntries = 1000

// Namespace is the namespace of the metadata of a playlist
const Namespace = "playlist"

// ContentTypes are the content types of playlists, which are text, so a
// handler of audio which reads it as audio doesn't read them
var ContentTypes = regexp.MustCompile(`^(?:audio/(?:x-)?mpegurl|audio/x-scpls|application/(?:vnd\.apple\.mpegurl|x-mpegurl))$`)

// playlistExtensions are the content types of playlists, by their
// extension, which refine text
var playlistExtensions = map[string]string{
	".m3u":  "audio/x-mpegurl",
	".m3u8": "application/vnd.apple.mpegurl",
	".pls":  "audio/x-scpls",
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	for ext, contentType := range playlistExtensions {
		if err := metadata.AddTextExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	// The kind of a playlist, its format and title, the number of its
	// entries, and their titles, locations and duration
	metadata.AddHandler(ContentTypes, "playlist", func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		// Read one more byte than the most which is read, to know whether
		// the playlist is longer
		kind := metadata.KindMetadata(dc.KindData)
		data, err := io.ReadAll(io.LimitReader(r, maxPlaylistSize+1))
		if err != nil {
			return kind, err
		}
		truncated := len(data) > maxPlaylistSize
		if truncated {
			// Drop the line which is cut off
			data = data[:maxPlaylistSize]
			if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
				data = data[:i+1]
			}
		}
		p := parsePlaylist(data)
		p.truncated = truncated
		return append(p.metadata(), kind...), nil
	}, Namespace, dc.Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// parsePlaylist parses a playlist, which is PLS when it starts with a
// [playlist] section, or else M3U, which is HLS when it has HLS tags. Text
// which isn't UTF-8 is Windows-1252, which older M3U playlists are in.
func parsePlaylist(data []byte) *playlist {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		if decoded, err := charmap.Windows1252.NewDecoder().Bytes(data); err == nil {
			data = decoded
		}
	}

	// Read the lines, without their whitespace or blank lines
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), maxPlaylistSize)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) > 0 && strings.EqualFold(lines[0], "[playlist]") {
		return parsePLS(lines[1:])
	}
	return parseM3U(lines)
}

// parseM3U parses an M3U playlist, where an entry is a line which isn't a
// comment or tag, after its #EXTINF tag in an extended playlist
func parseM3U(lines []string) *playlist {
	result := &playlist{format: "m3u"}
	var next playlistEntry
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "#EXT-X-"):
			// HLS, which lists segments or streams rather than tracks
			result.format = "hls"
		case strings.HasPrefix(line, "#PLAYLIST:"):
			result.title = strings.TrimSpace(strings.TrimPrefix(line, "#PLAYLIST:"))
		case strings.HasPrefix(line, "#EXTINF:"):
			next.duration, next.timed, next.title = parseEXTINF(strings.TrimPrefix(line, "#EXTINF:"))
		case strings.HasPrefix(line, "#"):
			// A comment, or another tag
		default:
			next.location = line
			result.entries = append(result.entries, next)
			next = playlistEntry{}
		}
	}
	return result
}

// parseEXTINF parses an #EXTINF tag, which is the duration in seconds, which
// is negative when it's unknown, then attributes such as tvg-name="...", and
// then a comma and the title. A comma in a quoted attribute doesn't end them.
func parseEXTINF(value string) (time.Duration, bool, string) {
	// The duration is up to the first space or comma
	end := strings.IndexAny(value, " ,")
	if end < 0 {
		end = len(value)
	}
	duration, timed := playlistDuration(value[:end])

	// The title is after the first comma which isn't quoted
	quoted := false
	for i := end; i < len(value); i++ {
		switch value[i] {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				return duration, timed, strings.TrimSpace(value[i+1:])
			}
		}
	}
	return duration, timed, ""
}

// parsePLS parses the [playlist] section of a PLS playlist, where entry n is
// Filen, with Titlen and Lengthn, in the order of their numbers
func parsePLS(lines []string) *playlist {
	result := &playlist{format: "pls"}
	entries := make(map[int]*playlistEntry)
	for _, line := range lines {
		if strings.HasPrefix(line, "[") {
			break
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)

		// The key is a name and the number of the entry
		name := strings.TrimRight(key, "0123456789")
		n, err := strconv.Atoi(key[len(name):])
		if err != nil {
			continue
		}
		entry, exists := entries[n]
		if !exists {
			entry = new(playlistEntry)
			entries[n] = entry
		}
		switch name {
		case "file":
			entry.location = value
		case "title":
			entry.title = value
		case "length":
			entry.duration, entry.timed = playlistDuration(value)
		}
	}

	// The entries with a location, in order
	numbers := make([]int, 0, len(entries))
	for n, entry := range entries {
		if entry.location != "" {
			numbers = append(numbers, n)
		}
	}
	slices.Sort(numbers)
	for _, n := range numbers {
		result.entries = append(result.entries, *entries[n])
	}
	return result
}

// playlistDuration returns a duration in seconds, which isn't known when
// it's negative, or zero, which some playlists, such as Zoom's, have for an
// entry whose duration they don't know
func playlistDuration(value string) (time.Duration, bool) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds <= 0 {
		return 0, false
	}
	return time.Duration(seconds * float64(time.Second)), true
}

// metadata returns the metadata of a playlist: its format, title, number of
// entries, and total duration when every entry has one, and for a playlist
// of tracks, the titles and locations of the first entries. The number of
// entries and total duration of a playlist which is too long to read aren't
// known.
func (p *playlist) metadata() []metadata.Metadata {
	result := []metadata.Metadata{meta.New(Namespace, "format", p.format)}
	if !p.truncated {
		result = append(result, meta.New(Namespace, "count", len(p.entries)))
	}
	if p.title != "" {
		result = append(result, meta.New(dc.Namespace, dc.Title, p.title))
	}

	// The total duration, when every entry has a duration
	var total time.Duration
	timed := !p.truncated && len(p.entries) > 0
	for _, entry := range p.entries {
		timed = timed && entry.timed
		total += entry.duration
	}
	if timed {
		result = append(result, meta.New(Namespace, "duration", total))
	}

	// HLS lists segments or streams, which aren't searched
	if p.format == "hls" {
		return result
	}

	var titles, locations []string
	for _, entry := range p.entries[:min(len(p.entries), maxPlaylistEntries)] {
		if entry.title != "" {
			titles = append(titles, entry.title)
		}
		locations = append(locations, entry.location)
	}
	if len(titles) > 0 {
		result = append(result, meta.NewText(Namespace, "titles", titles, strings.Join(titles, "; ")))
	}
	if len(locations) > 0 {
		result = append(result, meta.NewText(Namespace, "entries", locations, strings.Join(locations, "; ")))
	}
	return result
}
