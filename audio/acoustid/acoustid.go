// Package acoustid registers a handler of audio which looks up a recording
// on AcoustID, when an AcoustID client is set (see metadata.WithAcoustID),
// for its MusicBrainz recording and artist identifiers, when they aren't in
// its tags: by its AcoustID identifier, when it's in its tags, or else, or
// when it isn't linked to a recording, by the fingerprint of its audio, when
// it's built with cgo and Chromaprint (see the chromaprint package), whose
// match is its AcoustID identifier. When
// it can't, such as when nothing matches, its note, acoustid:note, says why.
package acoustid

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	playlist "github.com/mutablelogic/go-metadata/audio/playlist"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	acoustid "github.com/mutablelogic/go-metadata/pkg/acoustid"
	schema "github.com/mutablelogic/go-metadata/pkg/acoustid/schema"
	chromaprint "github.com/mutablelogic/go-metadata/pkg/chromaprint"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of a recording's AcoustID identifier, which
	// the handler of audio returns from its tags
	Namespace = "acoustid"

	// MusicBrainzNamespace is the namespace of a recording's MusicBrainz
	// identifiers, which the handler of audio returns from its tags
	MusicBrainzNamespace = "musicbrainz"

	// audioNamespace is the namespace of audio's metadata, which ffmpeg's
	// handler reads, such as whether it's protected with DRM, as audio:drm
	audioNamespace = "audio"

	// minScore is the least score of a match which is the recording, from 0
	// to 1, as a lower score can be another recording
	minScore = 0.5

	// lookupMeta is the metadata of a match which is looked up: its
	// recordings, with their artists, and how many sources each has, which
	// is how sure AcoustID is of them
	lookupMeta = acoustid.MetaRecordings | acoustid.MetaSources

	// priority is the priority of the handler, so its kind, a recording,
	// is in place of the kind of the handler of audio
	priority = 10
)

var (
	reContentType = regexp.MustCompile(`^audio/.+$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The AcoustID identifier of a recording, and its MusicBrainz recording
	// and artist identifiers, when there's an AcoustID client, and they
	// aren't in its tags, which runs after the handler of audio, for its
	// tags, and whether it's protected with DRM
	metadata.AddHandlerAfterWithPriority(reContentType, Namespace, priority, []string{Namespace, MusicBrainzNamespace, audioNamespace}, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		// A playlist, which is text, isn't audio which can be looked up
		client := o.AcoustID()
		if client == nil || playlist.ContentTypes.MatchString(o.ContentType()) || o.Get(MusicBrainzNamespace, "recording-id") != nil {
			return nil, nil
		}

		// The AcoustID identifier is in the tags, so look it up for its
		// recordings, and when it isn't known, or has none, look up the
		// fingerprint instead, which can match another identifier
		var reason string
		if id := o.Get(Namespace, "id"); id != nil {
			results, err := client.LookupTrack(ctx, id.Value(), lookupMeta)
			if err != nil {
				return nil, err
			}
			best := bestResult(results)
			switch {
			case best == nil:
				reason = "The AcoustID identifier in the tags isn't known to AcoustID"
			case !hasRecording(best):
				reason = "The AcoustID identifier in the tags isn't linked to a MusicBrainz recording"
			default:
				return recordingMetadata(o, best.Recordings), nil
			}
		}

		// Audio which is protected with DRM, such as an iTunes Store song,
		// can't be decoded, so it isn't fingerprinted, which isn't an error,
		// though its AcoustID identifier, when it's tagged with one, is looked
		// up, which doesn't decode it
		if drm := o.Get(audioNamespace, "drm"); drm != nil && drm.Any() == true {
			if reason != "" {
				return note(reason + ", and the audio wasn't fingerprinted, as it's protected with DRM."), nil
			}
			return note("The audio wasn't looked up on AcoustID, as it's protected with DRM, so it can't be fingerprinted."), nil
		}

		// Otherwise, look up the fingerprint of the audio, and its duration,
		// for the AcoustID identifier of the best match, when it's good
		// enough, and its recordings, when it's built with Chromaprint
		if !chromaprint.Enabled {
			if reason != "" {
				return note(reason + ", and the audio wasn't fingerprinted, as metadata was built without Chromaprint."), nil
			}
			return note("The audio wasn't looked up on AcoustID, as metadata was built without Chromaprint, which fingerprints it."), nil
		}
		print, duration, err := chromaprint.Fingerprint(ctx, r)
		if err != nil {
			return nil, err
		} else if print == "" || duration < time.Second {
			return note("The audio wasn't looked up on AcoustID, as it's shorter than a second."), nil
		}
		results, err := client.Lookup(ctx, print, duration, lookupMeta)
		if err != nil {
			return nil, err
		}
		best := bestResult(results)
		switch {
		case best == nil:
			return note("No recording on AcoustID matches the fingerprint of the audio."), nil
		case best.Score < minScore:
			return note(fmt.Sprintf("The best match on AcoustID, %s, has a score of %.2f, which is less than %.1f, so it may be another recording.", best.ID, best.Score, minScore)), nil
		}
		return append([]metadata.Metadata{
			meta.New(Namespace, "id", best.ID),
			meta.New(Namespace, "score", best.Score),
		}, recordingMetadata(o, best.Recordings)...), nil
	}, Namespace, MusicBrainzNamespace, dc.Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// bestResult returns the match with the highest score, or nil when there
// are none
func bestResult(results []schema.Result) *schema.Result {
	var best *schema.Result
	for i, result := range results {
		if best == nil || result.Score > best.Score {
			best = &results[i]
		}
	}
	if best == nil || best.ID == "" {
		return nil
	}
	return best
}

// hasRecording returns true when a match has a MusicBrainz recording
func hasRecording(result *schema.Result) bool {
	return slices.ContainsFunc(result.Recordings, func(r schema.Recording) bool { return r.ID != "" })
}

// note returns a note of why a recording's MusicBrainz identifiers weren't
// looked up
func note(text string) []metadata.Metadata {
	return []metadata.Metadata{meta.New(Namespace, "note", text)}
}

// recordingMetadata returns the MusicBrainz identifiers of the recording of
// a match which most sources have, which is the one AcoustID is surest of,
// and of its artists, unless they're in the tags, and its kind, a
// recording, or a note when the match has no recordings
func recordingMetadata(o *metadata.Opts, recordings []schema.Recording) []metadata.Metadata {
	var best *schema.Recording
	for i, recording := range recordings {
		if recording.ID != "" && (best == nil || recording.Sources > best.Sources) {
			best = &recordings[i]
		}
	}
	if best == nil {
		return note("The match on AcoustID isn't linked to a MusicBrainz recording.")
	}
	result := append(metadata.KindMetadata(dc.KindRecording), meta.New(MusicBrainzNamespace, "recording-id", best.ID))
	if o.Get(MusicBrainzNamespace, "artist-id") == nil {
		for _, artist := range best.Artists {
			if artist.ID != "" {
				result = append(result, meta.New(MusicBrainzNamespace, "artist-id", artist.ID))
			}
		}
	}
	return result
}
