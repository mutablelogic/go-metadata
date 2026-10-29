//go:build cgo

// Package ffmpeg registers handlers of videos, such as MP4, QuickTime and
// AVI videos, which return the tags of their container, their duration,
// the codec and size of their video, and the codec of their audio, and
// their cover art as artwork, which use ffmpeg, so need cgo
package ffmpeg

import (
	"context"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	gomedia "github.com/mutablelogic/go-media"
	profile "github.com/mutablelogic/go-media/profile/schema"
	reader "github.com/mutablelogic/go-media/reader"
	ff "github.com/mutablelogic/go-media/sys/ffmpeg80"
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// The namespaces of the metadata of a video: its tags, duration and the
	// codec and size of its video, the codec of its audio, and the tags
	// Apple's QuickTime writes, such as the camera of an iPhone
	VideoNamespace     = "video"
	AudioNamespace     = "audio"
	QuickTimeNamespace = "quicktime"
	AndroidNamespace   = "android"

	// The tags of a QuickTime video which are when it was recorded, in its
	// own time zone, and where, in ISO 6709
	quicktimeCreationDate = "creationdate"
	quicktimeLocation     = "location-iso6709"

	// locationKey is the key of the position of a video, in ISO 6709, such as
	// an Android phone writes, which is a geo.LatLong, as QuickTime's is
	locationKey = "location"

	// drmiTag and drmsTag are the codec tags of video and audio protected
	// with Apple's FairPlay, as ffmpeg reads them, little-endian fourccs
	drmiTag = uint32('d') | uint32('r')<<8 | uint32('m')<<16 | uint32('i')<<24
	drmsTag = uint32('d') | uint32('r')<<8 | uint32('m')<<16 | uint32('s')<<24
)

var (
	reContentType = regexp.MustCompile(`^video/.+$`)

	// reNotWord matches what isn't a letter or a digit in the name of a tag
	reNotWord = regexp.MustCompile(`[^a-z0-9]+`)

	// reLanguage matches the name of a tag with a language, such as
	// model-eng, which ffmpeg adds to a tag with a language
	reLanguage = regexp.MustCompile(`^(.+)-[a-z]{3}$`)

	// quicktimeDates are the layouts of a QuickTime creation date, which is
	// ISO 8601, such as 2023-03-16T07:23:27+0100, as Apple writes it, or
	// 2023-03-16T07:23:27+01:00, with or without a fraction of a second
	quicktimeDates = []string{"2006-01-02T15:04:05Z0700", time.RFC3339Nano}

	// creationTimes are the layouts of the creation time of a container,
	// which is RFC 3339, or a date and time without a time zone, as an AVI
	// video has, which is in UTC
	creationTimes = []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05"}

	// vendorNamespaces are the namespaces of the tags of the vendors of
	// phones and cameras, by the prefixes of their tags
	vendorNamespaces = map[string]string{
		"com-apple-quicktime-": QuickTimeNamespace,
		"com-android-":         AndroidNamespace,
	}

	// ignoredTags are tags of a container which aren't metadata of a video
	ignoredTags = map[string]bool{
		"compatible-brands": true, "major-brand": true, "minor-version": true,
		"comment": true, "itunes-cddb-1": true, "itunmovi": true,
		"gapless-playback": true, "itunextc": true, "encoder": true,
		"handler-name": true, "vendor-id": true,
	}

	// dcTags are tags of a container which are Dublin Core elements
	dcTags = map[string]string{
		"title":       dc.Title,
		"director":    dc.Creator,
		"artist":      dc.Creator,
		"description": dc.Description,
		"copyright":   dc.Rights,
		"language":    dc.Language,
	}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// ffmpeg logs to stderr, which isn't wanted
	ff.AVUtil_log_set_level(ff.AV_LOG_QUIET)

	metadata.AddHandler(reContentType, "ffmpeg", func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindVideo)
		rd, err := reader.NewReader(r)
		if err != nil {
			// A video which can't be read has no more metadata, which isn't
			// an error, as another handler may read it
			return kind, nil
		}
		defer rd.Close()
		result := tagMetadata(rd.Metadata())
		if d := rd.Duration(); d > 0 {
			result = append(result, meta.New(VideoNamespace, "duration", d))
		}
		result = append(result, streamMetadata(rd.Streams())...)
		var name string
		if named, ok := r.(metadata.NamedReader); ok {
			name = named.Name()
		}
		cover := len(rd.Metadata(gomedia.MetaArtwork)) > 0
		return append(append(result, kind...), metadata.HintMetadata(videoHints(result, name, cover)...)...), nil
	}, VideoNamespace, AudioNamespace, QuickTimeNamespace, dc.Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// tagMetadata returns the tags of a container as metadata: the tags which
// are Dublin Core elements, such as its title, in the dc namespace, and the
// synopsis as dc:description, when there's no description; the tags of
// vendors, such as com.apple.quicktime.model, in their namespaces, such as
// quicktime; a location, in ISO 6709, as a geo.LatLong, and dc:coverage;
// and the others in the video namespace, without those which aren't metadata,
// and those repeated with a language, which have the same value. The date
// it was recorded, which is QuickTime's creation date, in its own time
// zone, or else its creation time, in UTC, is dc:date, and a creation time
// which is the Unix epoch isn't known, so it isn't returned.
func tagMetadata(tags []gomedia.Metadata) []metadata.Metadata {
	values := make(map[string]string, len(tags))
	for _, tag := range tags {
		values[strings.ToLower(tag.Key())] = tag.Value()
	}

	var result []metadata.Metadata
	has := make(map[string]bool)
	add := func(ns, key string, value any) {
		if has[ns+":"+key] {
			return
		}
		if entry := meta.New(ns, key, value); entry != nil {
			result = append(result, entry)
			has[ns+":"+key] = true
		}
	}

	var synopsis, creationTime, creationDate string
	for _, tag := range tags {
		name, value := strings.ToLower(tag.Key()), strings.TrimSpace(tag.Value())
		if value == "" {
			continue
		} else if match := reLanguage.FindStringSubmatch(name); match != nil && values[match[1]] == tag.Value() {
			continue
		}
		key := strings.Trim(reNotWord.ReplaceAllString(name, "-"), "-")

		// The tags of a vendor, such as com.apple.quicktime.model
		ns := VideoNamespace
		for prefix, vendor := range vendorNamespaces {
			if name, found := strings.CutPrefix(key, prefix); found && name != "" {
				ns, key = vendor, name
			}
		}
		if ns == QuickTimeNamespace {
			switch key {
			case quicktimeCreationDate:
				creationDate = value
			case quicktimeLocation:
				key = locationKey
			}
		}

		switch {
		case ns == VideoNamespace && ignoredTags[key]:
			continue
		case ns == VideoNamespace && key == "synopsis":
			synopsis = value
			continue
		case ns == VideoNamespace && key == "creation-time":
			// A time which is the Unix epoch isn't known, as it's written
			// when there's no time
			if t, ok := parseCreationTime(value); ok && t.Unix() == 0 {
				continue
			}
			creationTime = value
		case key == locationKey:
			// A position in ISO 6709, which is dc:coverage too
			if ll, err := geo.ParseISO6709(value); err == nil {
				add(ns, key, ll)
				add(dc.Namespace, dc.Coverage, ll)
				continue
			}
		}
		if element, exists := dcTags[key]; exists && ns == VideoNamespace {
			add(dc.Namespace, element, value)
		} else {
			add(ns, key, value)
		}
	}
	if synopsis != "" {
		add(dc.Namespace, dc.Description, synopsis)
	}
	if t, ok := parseQuicktimeDate(creationDate); ok {
		add(dc.Namespace, dc.Date, t)
	} else if t, ok := parseCreationTime(creationTime); ok {
		add(dc.Namespace, dc.Date, t)
	}
	return result
}

// videoHints returns the hints of a video, from its metadata, its name, and
// whether it has cover art: its artwork, when it has a video stream, as a
// frame can be its thumbnail, unless it's protected with DRM, so a frame
// can't be decoded, when it's only its cover art, its place, when it has a
// position, and that it's TV, when it's an episode of a show, by its tags,
// or a name such as Show.S01E02.mkv
func videoHints(meta []metadata.Metadata, name string, cover bool) []string {
	var hints []string
	stream := slices.ContainsFunc(meta, func(m metadata.Metadata) bool { return m.NS() == VideoNamespace && m.Key() == "codec" })
	drm := slices.ContainsFunc(meta, func(m metadata.Metadata) bool { return m.NS() == VideoNamespace && m.Key() == "drm" })
	if (stream && !drm) || cover {
		hints = append(hints, metadata.HintArtwork)
	}
	if slices.ContainsFunc(meta, func(m metadata.Metadata) bool {
		ll, ok := m.Any().(geo.LatLong)
		return ok && ll.Validate() == nil
	}) {
		hints = append(hints, metadata.HintGeo)
	}
	show := slices.ContainsFunc(meta, func(m metadata.Metadata) bool {
		return m.NS() == VideoNamespace && m.Key() == "show" && strings.TrimSpace(m.Value()) != ""
	})
	if show || (name != "" && tmdb.ExtractQuery(name).Episode > 0) {
		hints = append(hints, metadata.HintTV)
	}
	return hints
}

// streamMetadata returns the codec, width and height of the first video
// stream, and the codec, sample rate and number of channels of the first
// audio stream, by the index of the streams, which don't include cover art,
// and whether each is protected with DRM, as video:drm and audio:drm
func streamMetadata(streams map[int]profile.Profile) []metadata.Metadata {
	var result []metadata.Metadata
	var video, audio bool
	for _, index := range slices.Sorted(maps.Keys(streams)) {
		par := streams[index].Par()
		if par == nil {
			continue
		}
		switch par.CodecType() {
		case ff.AVMEDIA_TYPE_VIDEO:
			if video {
				continue
			}
			video = true
			result = append(result,
				meta.New(VideoNamespace, "codec", par.CodecID().Name()),
				meta.New(VideoNamespace, "width", par.Width()),
				meta.New(VideoNamespace, "height", par.Height()),
			)
			if isProtected(par.CodecTag()) {
				result = append(result, meta.New(VideoNamespace, "drm", true))
			}
		case ff.AVMEDIA_TYPE_AUDIO:
			if audio {
				continue
			}
			audio = true
			result = append(result, meta.New(AudioNamespace, "codec", par.CodecID().Name()))
			if isProtected(par.CodecTag()) {
				result = append(result, meta.New(AudioNamespace, "drm", true))
			}
			if rate := par.SampleRate(); rate > 0 {
				result = append(result, meta.New(AudioNamespace, "sample-rate", rate))
			}
			if layout := par.ChannelLayout(); layout.NumChannels() > 0 {
				result = append(result, meta.New(AudioNamespace, "channels", layout.NumChannels()))
			}
		}
	}
	return result
}

// isProtected returns true when a stream's codec tag is of a stream which is
// protected with Apple's FairPlay DRM: drmi for video, and drms for audio,
// as iTunes Store films and episodes are, as .m4v, which ffmpeg reads as a
// little-endian fourcc
func isProtected(tag uint32) bool {
	return tag == drmiTag || tag == drmsTag
}

// parseCreationTime returns the creation time of a container, or false when
// it isn't one
func parseCreationTime(value string) (time.Time, bool) {
	for _, layout := range creationTimes {
		if t, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseQuicktimeDate returns the time of a QuickTime creation date, or false
// when it isn't one
func parseQuicktimeDate(value string) (time.Time, bool) {
	for _, layout := range quicktimeDates {
		if t, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
