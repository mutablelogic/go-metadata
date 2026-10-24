// Package acoustid registers a handler of audio which looks up a recording
// on AcoustID, when an AcoustID client is set (see metadata.WithAcoustID),
// for its MusicBrainz recording and artist identifiers, when they aren't in
// its tags: by its AcoustID identifier, when it's in its tags, or else by
// the fingerprint of its audio, when it's built with cgo and Chromaprint
// (see the chromaprint package), which is its AcoustID identifier too
package acoustid

import (
	"context"
	"io"
	"regexp"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	acoustid "github.com/mutablelogic/go-metadata/pkg/acoustid"
	schema "github.com/mutablelogic/go-metadata/pkg/acoustid/schema"
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

	// minScore is the least score of a match which is the recording, from 0
	// to 1, as a lower score can be another recording
	minScore = 0.5

	// lookupMeta is the metadata of a match which is looked up: its
	// recordings, with their artists, and how many sources each has, which
	// is how sure AcoustID is of them
	lookupMeta = acoustid.MetaRecordings | acoustid.MetaSources
)

var (
	reContentType = regexp.MustCompile(`^audio/.+$`)

	// fingerprint returns the fingerprint of audio, and its duration, which
	// is set when it's built with cgo and Chromaprint
	fingerprint func(context.Context, io.Reader) (string, time.Duration, error)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The AcoustID identifier of a recording, and its MusicBrainz recording
	// and artist identifiers, when there's an AcoustID client, and they
	// aren't in its tags, which runs after the handler of audio, for its
	// tags
	metadata.AddHandlerAfter(reContentType, Namespace, []string{Namespace, MusicBrainzNamespace}, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		client := o.AcoustID()
		if client == nil || o.Get(MusicBrainzNamespace, "recording-id") != nil {
			return nil, nil
		}

		// The AcoustID identifier is in the tags, so look it up for its
		// recordings
		if id := o.Get(Namespace, "id"); id != nil {
			results, err := client.LookupTrack(ctx, id.Value(), lookupMeta)
			if err != nil {
				return nil, err
			}
			if best := bestResult(results, 0); best != nil {
				return recordingMetadata(o, best.Recordings), nil
			}
			return nil, nil
		}

		// Otherwise, look up the fingerprint of the audio, and its duration,
		// for the AcoustID identifier of the best match, when it's good
		// enough, and its recordings
		if fingerprint == nil {
			return nil, nil
		}
		print, duration, err := fingerprint(ctx, r)
		if err != nil {
			return nil, err
		} else if print == "" || duration < time.Second {
			return nil, nil
		}
		results, err := client.Lookup(ctx, print, duration, lookupMeta)
		if err != nil {
			return nil, err
		}
		best := bestResult(results, minScore)
		if best == nil {
			return nil, nil
		}
		return append([]metadata.Metadata{
			meta.New(Namespace, "id", best.ID),
			meta.New(Namespace, "score", best.Score),
		}, recordingMetadata(o, best.Recordings)...), nil
	}, Namespace, MusicBrainzNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// bestResult returns the match with the highest score, when its score is at
// least min, or nil
func bestResult(results []schema.Result, min float64) *schema.Result {
	var best *schema.Result
	for i, result := range results {
		if best == nil || result.Score > best.Score {
			best = &results[i]
		}
	}
	if best == nil || best.ID == "" || best.Score < min {
		return nil
	}
	return best
}

// recordingMetadata returns the MusicBrainz identifiers of the recording of
// a match which most sources have, which is the one AcoustID is surest of,
// and of its artists, unless they're in the tags
func recordingMetadata(o *metadata.Opts, recordings []schema.Recording) []metadata.Metadata {
	var best *schema.Recording
	for i, recording := range recordings {
		if recording.ID != "" && (best == nil || recording.Sources > best.Sources) {
			best = &recordings[i]
		}
	}
	if best == nil {
		return nil
	}
	result := []metadata.Metadata{meta.New(MusicBrainzNamespace, "recording-id", best.ID)}
	if o.Get(MusicBrainzNamespace, "artist-id") == nil {
		for _, artist := range best.Artists {
			if artist.ID != "" {
				result = append(result, meta.New(MusicBrainzNamespace, "artist-id", artist.ID))
			}
		}
	}
	return result
}
