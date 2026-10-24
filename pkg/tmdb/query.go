package tmdb

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Query is a search query extracted from a media file's path: the title to
// search for, its release year if present, and (for a TV episode) its
// season/episode number.
type Query struct {
	Name    string
	Year    int  // 0 if no year was found
	Season  int  // 0 unless Episode is also set
	Episode int  // 0 unless this looks like a TV episode
	TV      bool // true for a TV show's video, even without an Episode
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// An episode marker, such as S01E02, which can be at the start of the
	// name, and can be for more than one episode, such as S01E02E03, of
	// which the first is used
	episodeRe = regexp.MustCompile(`(?i)(?:^|[\s._-])s(\d{1,2})e(\d{1,3})(?:-?e\d{1,3})*(?:\D|$)`)

	// An episode marker without an E, such as S0102, which is two digits
	// for the season and two for the episode
	shortEpisodeRe = regexp.MustCompile(`(?i)(?:^|[\s._-])s(\d{2})(\d{2})(?:[\s._-]|$)`)

	// An episode marker with an x, such as 1x02 or (1x02)
	crossEpisodeRe = regexp.MustCompile(`(?i)(?:^|[\s._(\[-])(\d{1,2})x(\d{2,3})(?:\D|$)`)

	// An episode number without a season, such as "Episode 3"
	episodeWordRe = regexp.MustCompile(`(?i)(?:^|[\s._-])(?:episode|ep)[\s._-]*(\d{1,3})(?:\D|$)`)

	// An episode number at the start of the name of a file in a season
	// folder, such as "03 Title", or "203 Title" for the third episode of
	// the second season
	leadingEpisodeRe = regexp.MustCompile(`^(\d{1,3})(?:[\s._-]|$)`)

	// A season folder, such as "Season 2", "Series 2" or "S02 - Title",
	// whose parent is the show's folder
	seasonRe = regexp.MustCompile(`(?i)^(?:(?:season|series)[\s._-]*(\d{1,2}|[a-z]+)|s(\d{1,2})(?:[\s._-].*)?)$`)

	// A folder of a show's or movie's extras, whose parent is its folder
	extrasRe = regexp.MustCompile(`(?i)^(?:extras|featurettes|specials|bonus(?:[\s._-]features)?|behind[\s._-]the[\s._-]scenes|deleted[\s._-]scenes)$`)

	// Release tags, after which the rest of the name is dropped. The tags
	// which are also English words, such as Limited, are only tags in upper
	// case, as release names have them, so a title such as The Darjeeling
	// Limited keeps them.
	noiseRe = regexp.MustCompile(`[\s._-](?:(?i:2160p|1080p|720p|480p|blu-?ray|bdrip|brrip|webrip|web-?dl|hdtv|dvdrip|remux|hdr10?|dolby.?vision|atmos|x264|x265|h\.?264|h\.?265|hevc|avc|xvid|divx|aac\d?|ac3|dts(?:-hd)?|truehd|\d\.\d(?:ch)?|director.?s.?cut|dual.?audio)|EXTENDED|UNRATED|THEATRICAL|PROPER|REPACK|LIMITED|INTERNAL|MULTI)\b.*$`)

	yearRe        = regexp.MustCompile(`[\s._(\[](19\d{2}|20\d{2})`)
	sepRe         = regexp.MustCompile(`[._]+`)
	spaceRe       = regexp.MustCompile(`\s{2,}`)
	genericNameRe = regexp.MustCompile(`(?i)^(video[\s._-]?ts(?:[\s._-]?\d+)*|vts[\s._-]?\d+(?:[\s._-]?\d+)*|title[\s._-]?\d*|track[\s._-]?\d*|extra[\s._-]?\d*|movie|main|cd[\s._-]?\d+|disc[\s._-]?\d+|\d+)$`)

	// The numbers of seasons which are words, such as "Season One"
	seasonWords = map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	}
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ExtractQuery derives a search query - title, year, and (for a TV episode)
// season/episode - from a media file's path. The title and year which the
// filename doesn't have are from the folder of the movie or show, which is
// the file's folder, or its parent when that's a season or extras folder,
// such as "Show (Year)/Season 2/03 Episode Title.mkv", in which the season
// is from the season folder, and the episode from the start of the
// filename. A video with an episode, or in a season folder, is a TV show's,
// even when its episode isn't known, such as "Show/Season 1/Pilot.mkv",
// whose title is the show's.
func ExtractQuery(path string) Query {
	base := filepath.Base(path)
	q := parseName(strings.TrimSuffix(base, filepath.Ext(base)))

	// The folder of the movie or show, which is the parent of an extras
	// folder, and of a season folder, which can have an extras folder
	dir := filepath.Dir(path)
	isExtras := extrasRe.MatchString(filepath.Base(dir))
	if isExtras {
		dir = filepath.Dir(dir)
	}
	season, isSeason := parseSeason(filepath.Base(dir))
	if isSeason {
		dir = filepath.Dir(dir)
	}

	// In a season folder, the episode is from the filename, which is its
	// number, or a marker, and otherwise from the start of the filename,
	// before the episode's title. A file without either is named for the
	// episode, such as "Pilot", which isn't the show's title. An extra
	// keeps its name, and a number at its start isn't an episode.
	if isSeason {
		if q.Episode == 0 && !isExtras {
			if episode, ok := leadingEpisode(q.Name, season); ok {
				q.Episode = episode
			}
			q.Name = ""
		}
		if q.Episode != 0 && q.Season == 0 {
			q.Season = season
		}
	}

	// An episode number without a season is of the first season
	if q.Episode != 0 && q.Season == 0 {
		q.Season = 1
	}

	// A video with an episode, or in a season folder, except an extra, is
	// a TV show's
	q.TV = q.Episode != 0 || (isSeason && !isExtras)

	// There's no folder for a bare filename, rather than "." or "/"
	if dir == "." || dir == string(filepath.Separator) {
		return q
	}

	// The title is from the folder when the filename's isn't usable, and
	// the year is from the folder when the filename has none
	dq := parseName(filepath.Base(dir))
	if isGenericName(q.Name) && !isGenericName(dq.Name) {
		q.Name = dq.Name
	}
	if q.Year == 0 {
		q.Year = dq.Year
	}

	return q
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// isGenericName reports whether name carries no real title information -
// either empty, or matched by genericNameRe.
func isGenericName(name string) bool {
	return name == "" || genericNameRe.MatchString(name)
}

// parseSeason returns the season of a season folder, such as "Season 2",
// "Season Two" or "S02 - Title", or false when it isn't one
func parseSeason(name string) (int, bool) {
	m := seasonRe.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	value := strings.ToLower(m[1] + m[2])
	if season, exists := seasonWords[value]; exists {
		return season, true
	}
	season, err := strconv.Atoi(value)
	return season, err == nil
}

// leadingEpisode returns the episode number at the start of a name in a
// season folder, which is the episode, such as 03, or the season and
// episode, such as 203 in the second season, or false when there isn't one
func leadingEpisode(name string, season int) (int, bool) {
	m := leadingEpisodeRe.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	n, _ := strconv.Atoi(m[1])
	switch {
	case n < 100 && n > 0:
		return n, true
	case n >= 100 && n/100 == season && n%100 > 0:
		return n % 100, true
	}
	return 0, false
}

// parseName extracts a Query from a single filename, without its extension,
// or directory name (not a full path), which can have a dot, such as
// "Mrs. America". It doesn't attempt to strip a trailing "-GROUP"
// release-group tag on its own, since that's indistinguishable from a
// genuinely hyphenated title (e.g. "Spider-Man"); a release group is only
// removed when it trails a recognized quality/source/codec tag matched by
// noiseRe.
func parseName(name string) Query {
	var q Query

	// Check for a season/episode marker, or an episode number, first, and
	// remove it and what follows it, which is the episode's title
	if m := episodeRe.FindStringSubmatchIndex(name); m != nil {
		q.Season, _ = strconv.Atoi(name[m[2]:m[3]])
		q.Episode, _ = strconv.Atoi(name[m[4]:m[5]])
		name = name[:m[0]]
	} else if m := shortEpisodeRe.FindStringSubmatchIndex(name); m != nil {
		q.Season, _ = strconv.Atoi(name[m[2]:m[3]])
		q.Episode, _ = strconv.Atoi(name[m[4]:m[5]])
		name = name[:m[0]]
	} else if m := crossEpisodeRe.FindStringSubmatchIndex(name); m != nil {
		q.Season, _ = strconv.Atoi(name[m[2]:m[3]])
		q.Episode, _ = strconv.Atoi(name[m[4]:m[5]])
		name = name[:m[0]]
	} else if m := episodeWordRe.FindStringSubmatchIndex(name); m != nil {
		q.Episode, _ = strconv.Atoi(name[m[2]:m[3]])
		name = name[:m[0]]
	}

	// Remove the release tags, and what follows them, which can have a year,
	// such as the year of a release
	name = noiseRe.ReplaceAllString(name, "")

	// Check for a release year, which is the last year in the name which
	// isn't later than next year, as a later one is part of the title, such
	// as Blade Runner 2049, and a release year comes after the title, which
	// can have a year in it, such as Summer of 1942 (1971). A year isn't
	// followed by another digit, and its separators aren't part of it, so a
	// year after another, such as 1942.1971, is found.
	latest := time.Now().Year() + 1
	matches := yearRe.FindAllStringSubmatchIndex(name, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		if m[3] < len(name) && name[m[3]] >= '0' && name[m[3]] <= '9' {
			continue
		} else if year, _ := strconv.Atoi(name[m[2]:m[3]]); year <= latest {
			q.Year = year
			name = name[:m[0]]
			break
		}
	}

	// Cleanup, including a separator before a removed marker, such as the
	// hyphen in "Title - S01E02"
	name = sepRe.ReplaceAllString(name, " ")
	name = spaceRe.ReplaceAllString(name, " ")
	q.Name = strings.Trim(name, " -")

	// Return the query even if the title is empty
	return q
}
