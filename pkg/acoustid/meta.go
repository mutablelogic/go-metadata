package acoustid

import (
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Meta selects the metadata of the tracks which a lookup returns, such as
// their recordings and releases
type Meta uint

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// MetaRecordings is the recordings of a track, their titles, durations
	// and artists
	MetaRecordings Meta = 1 << iota

	// MetaRecordingIDs is the MusicBrainz identifiers of the recordings of a
	// track, only
	MetaRecordingIDs

	// MetaReleases is the releases of the recordings
	MetaReleases

	// MetaReleaseIDs is the MusicBrainz identifiers of the releases, only
	MetaReleaseIDs

	// MetaReleaseGroups is the release groups of the recordings, such as
	// albums and singles
	MetaReleaseGroups

	// MetaReleaseGroupIDs is the MusicBrainz identifiers of the release
	// groups, only
	MetaReleaseGroupIDs

	// MetaTracks is the tracks of the releases, with their positions on the
	// releases' mediums
	MetaTracks

	// MetaCompress is a smaller response, without the duplicate metadata of
	// recordings and releases
	MetaCompress

	// MetaUserMeta is the metadata which users submitted
	MetaUserMeta

	// MetaSources is how many times each recording was submitted
	MetaSources

	// MetaNone is no metadata, only the tracks' identifiers and scores
	MetaNone Meta = 0

	// MetaAll is all the metadata
	MetaAll = MetaRecordings | MetaRecordingIDs | MetaReleases | MetaReleaseIDs | MetaReleaseGroups | MetaReleaseGroupIDs | MetaTracks | MetaCompress | MetaUserMeta | MetaSources
)

// metaNames are the names of the metadata which AcoustID's meta parameter
// has, in the order of the flags
var metaNames = []string{
	"recordings", "recordingids", "releases", "releaseids", "releasegroups",
	"releasegroupids", "tracks", "compress", "usermeta", "sources",
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// String returns the names of the metadata which the flags select, as
// AcoustID's meta parameter has them, separated by spaces, such as
// "recordings releasegroups"
func (m Meta) String() string {
	var names []string
	for i, name := range metaNames {
		if m&(1<<i) != 0 {
			names = append(names, name)
		}
	}
	return strings.Join(names, " ")
}
