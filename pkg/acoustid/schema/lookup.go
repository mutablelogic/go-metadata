// Package schema is the responses of the AcoustID API
package schema

///////////////////////////////////////////////////////////////////////////////
// TYPES

// LookupResponse is the response of a lookup of a fingerprint, which is the
// tracks which it matches, or an error
type LookupResponse struct {
	Status  string   `json:"status"`
	Error   Error    `json:"error"`
	Results []Result `json:"results"`
}

// Error is the error of a response, when its status isn't ok
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Result is an AcoustID track which a fingerprint matches, by its
// identifier, how well it matches, from 0 to 1, and its recordings, which
// are returned when they're requested
type Result struct {
	ID         string      `json:"id"`
	Score      float64     `json:"score"`
	Recordings []Recording `json:"recordings,omitempty"`
}

// Recording is a MusicBrainz recording of a track, with its title, duration
// in seconds, artists, and the release groups and releases it's on, which are
// returned when they're requested
type Recording struct {
	ID            string         `json:"id"`
	Title         string         `json:"title,omitempty"`
	Duration      float64        `json:"duration,omitempty"`
	Artists       []Artist       `json:"artists,omitempty"`
	ReleaseGroups []ReleaseGroup `json:"releasegroups,omitempty"`
	Releases      []Release      `json:"releases,omitempty"`
	Sources       int            `json:"sources,omitempty"`
}

// Artist is a MusicBrainz artist, with the phrase which joins it to the next
// artist, such as " & "
type Artist struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	JoinPhrase string `json:"joinphrase,omitempty"`
}

// ReleaseGroup is a MusicBrainz release group, such as an album, with its
// type, such as Album or Single, and its secondary types, such as
// Compilation
type ReleaseGroup struct {
	ID             string    `json:"id"`
	Type           string    `json:"type,omitempty"`
	SecondaryTypes []string  `json:"secondarytypes,omitempty"`
	Title          string    `json:"title,omitempty"`
	Artists        []Artist  `json:"artists,omitempty"`
	Releases       []Release `json:"releases,omitempty"`
}

// Release is a MusicBrainz release, with its mediums, which have the
// recording's tracks
type Release struct {
	ID          string   `json:"id"`
	Title       string   `json:"title,omitempty"`
	Country     string   `json:"country,omitempty"`
	Date        *Date    `json:"date,omitempty"`
	TrackCount  int      `json:"track_count,omitempty"`
	MediumCount int      `json:"medium_count,omitempty"`
	Artists     []Artist `json:"artists,omitempty"`
	Mediums     []Medium `json:"mediums,omitempty"`
}

// Date is the date of a release, whose month and day are zero when they
// aren't known
type Date struct {
	Year  int `json:"year,omitempty"`
	Month int `json:"month,omitempty"`
	Day   int `json:"day,omitempty"`
}

// Medium is a medium of a release, such as a CD, with its position, and the
// recording's tracks on it
type Medium struct {
	Format     string  `json:"format,omitempty"`
	Position   int     `json:"position,omitempty"`
	Title      string  `json:"title,omitempty"`
	TrackCount int     `json:"track_count,omitempty"`
	Tracks     []Track `json:"tracks,omitempty"`
}

// Track is a track of a medium, with its position on it
type Track struct {
	ID       string   `json:"id"`
	Title    string   `json:"title,omitempty"`
	Position int      `json:"position,omitempty"`
	Artists  []Artist `json:"artists,omitempty"`
}
