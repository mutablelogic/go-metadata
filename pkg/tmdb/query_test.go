package tmdb_test

import (
	"testing"

	// Packages
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
)

func Test_query_000(t *testing.T) {
	tests := []struct {
		path string
		want tmdb.Query
	}{
		// Plain movie title, no year or release tags.
		{"Hunt for Red October.mp4", tmdb.Query{Name: "Hunt for Red October"}},

		// Year in parentheses.
		{"The Matrix (1999).mp4", tmdb.Query{Name: "The Matrix", Year: 1999}},

		// Dot-separated with year and release tags.
		{"The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv", tmdb.Query{Name: "The Matrix", Year: 1999}},

		// Space-separated with year and release tags.
		{"Inception (2010) 1080p BluRay x264-GROUP.mp4", tmdb.Query{Name: "Inception", Year: 2010}},

		// Release tags but no year at all.
		{"Show Name 1080p WEBRip x264-GROUP.mkv", tmdb.Query{Name: "Show Name"}},

		// TV episode marker, dot-separated with release tags.
		{"Breaking.Bad.S01E01.720p.HDTV.x264-GROUP.mkv", tmdb.Query{Name: "Breaking Bad", Season: 1, Episode: 1, TV: true}},

		// TV episode marker, lower case.
		{"the.wire.s04e05.mkv", tmdb.Query{Name: "the wire", Season: 4, Episode: 5, TV: true}},

		// A leading year-shaped number that's actually part of the title
		// isn't mistaken for a release year.
		{"2001 A Space Odyssey (1968).mp4", tmdb.Query{Name: "2001 A Space Odyssey", Year: 1968}},

		// A year after next year is part of the title, not a release year.
		{"/movies/Blade Runner 2049/Blade Runner 2049.mp4", tmdb.Query{Name: "Blade Runner 2049"}},
		{"Blade Runner 2049 (2017).mkv", tmdb.Query{Name: "Blade Runner 2049", Year: 2017}},

		// The release year is the last year, after a title with a year in it.
		{"Summer of 1942 (1971).mp4", tmdb.Query{Name: "Summer of 1942", Year: 1971}},

		// Years separated by dots, which are both found.
		{"Summer.of.1942.1971.mp4", tmdb.Query{Name: "Summer of 1942", Year: 1971}},
		{"Blade.Runner.2049.2017.mkv", tmdb.Query{Name: "Blade Runner 2049", Year: 2017}},

		// A year in the release tags isn't the release year.
		{"The.Matrix.1999.1080p.BluRay.x264.2020-GROUP.mkv", tmdb.Query{Name: "The Matrix", Year: 1999}},

		// A longer number isn't a year.
		{"Area 19999.mp4", tmdb.Query{Name: "Area 19999"}},

		// A hyphenated title isn't mistaken for a trailing release group.
		{"Spider-Man.mkv", tmdb.Query{Name: "Spider-Man"}},

		// A generic disc-rip filename with no usable title falls back to
		// the parent directory, which carries the title/year.
		{"/movies/The Matrix (1999)/VTS_01_1.VOB", tmdb.Query{Name: "The Matrix", Year: 1999}},

		// A directory that itself has no usable title (and the filename
		// didn't either) falls back to the filename-derived query as a
		// last resort, rather than a wrongly "confident" empty result.
		{"/movies/1/VTS_01_1.VOB", tmdb.Query{Name: "VTS 01 1"}},

		// A bare filename with no directory component: the fallback is
		// skipped rather than misreading "." as a parent directory.
		{"VTS_01_1.VOB", tmdb.Query{Name: "VTS 01 1"}},

		// The following cases are drawn from guessit's real-world test
		// fixture (https://github.com/guessit-io/guessit, MIT licensed),
		// trimmed to the fields we extract (name/year/season/episode).
		// guessit itself does much more (episode titles, codecs, release
		// groups, dates, ...); these are only the cases where its expected
		// title/year/season/episode line up with what our simpler,
		// regex-based parser produces.

		// A year embedded in an episode filename is stripped independently
		// of the episode marker.
		{"The.Flash.2014.S01E01.PREAIR.WEBRip.XviD-EVO.avi", tmdb.Query{Name: "The Flash", Year: 2014, Season: 1, Episode: 1, TV: true}},

		// A two-digit season number.
		{"House.Hunters.International.S56E06.720p.hdtv.x264.mp4", tmdb.Query{Name: "House Hunters International", Season: 56, Episode: 6, TV: true}},

		// An episode title following the episode marker is dropped along
		// with the rest of the release tags, not mistaken for the title.
		{"Hostages.S01E01.Pilot.for.Air.720p.WEB-DL.DD5.1.H.264-NTb.nfo", tmdb.Query{Name: "Hostages", Season: 1, Episode: 1, TV: true}},

		// No real file extension (a bare release-tag suffix instead)
		// doesn't throw off the parse.
		{"some.series.S03E14.Title.Here.720p", tmdb.Query{Name: "some series", Season: 3, Episode: 14, TV: true}},

		// An apostrophe in the title survives cleanup.
		{"Jack's.Show.S03E01.blah.1080p", tmdb.Query{Name: "Jack's Show", Season: 3, Episode: 1, TV: true}},

		// A double-digit episode number, with a full path prefix.
		{"/media/live/A/Anger.Management.S02E82.720p.HDTV.X264-DIMENSION.mkv", tmdb.Query{Name: "Anger Management", Season: 2, Episode: 82, TV: true}},

		// The filename has a good title but no year; the parent directory
		// (a "Title (Year)" convention) supplies the year alone, without
		// its title overriding the filename's.
		{"Movies/Fear and Loathing in Las Vegas (1998)/Fear.and.Loathing.in.Las.Vegas.720p.HDDVD.DTS.x264-ESiR.mkv", tmdb.Query{Name: "Fear and Loathing in Las Vegas", Year: 1998}},

		// The following cases are drawn from a real media library, which
		// has a folder for each movie or show, named for its title and
		// sometimes its year, and a season folder for most shows.

		// A dot-separated filename takes its year from a "Title (Year)"
		// folder.
		{"Movies/Close Encounters of the Third Kind (1977)/Close.Encounters.of.the.Third.Kind.mp4", tmdb.Query{Name: "Close Encounters of the Third Kind", Year: 1977}},

		// A year after the title without parentheses, which the folder
		// also has.
		{"Movies/The Girl with the Dragon Tattoo (2011)/The Girl With The Dragon Tattoo 2011.mp4", tmdb.Query{Name: "The Girl With The Dragon Tattoo", Year: 2011}},

		// A colon in a title, which macOS saves as an underscore, is a
		// separator.
		{"Movies/Alien_ Covenant/Alien_ Covenant.mp4", tmdb.Query{Name: "Alien Covenant"}},
		{"Movies/Mission_ Impossible - Fallout/Mission_ Impossible - Fallout.mp4", tmdb.Query{Name: "Mission Impossible - Fallout"}},

		// A title which starts with a year-shaped number, after which the
		// colon is an underscore, has no release year.
		{"Movies/2001_ A Space Odyssey/2001_ A Space Odyssey.m4v", tmdb.Query{Name: "2001 A Space Odyssey"}},
		{"Movies/2010_ The Year We Make Contact/2010_ The Year We Make Contact.mp4", tmdb.Query{Name: "2010 The Year We Make Contact"}},

		// A title which is only numbers isn't a generic name.
		{"Movies/50_50/50_50.m4v", tmdb.Query{Name: "50 50"}},

		// A title which ends with a dot, before the extension.
		{"Movies/Monsters, Inc_/Monsters, Inc..m4v", tmdb.Query{Name: "Monsters, Inc"}},

		// Accented letters, which the folder doesn't have, are kept.
		{"Movies/Moolaade/Moolaadé.m4v", tmdb.Query{Name: "Moolaadé"}},
		{"Movies/Leon_ The Professional/Léon_ The Professional.mp4", tmdb.Query{Name: "Léon The Professional"}},

		// A featurette in a folder of a movie's extras has its own title
		// and year.
		{"Movies/Round Midnight/Featurettes/Dexter Gordon - Fried Bananas (1969).mkv", tmdb.Query{Name: "Dexter Gordon - Fried Bananas", Year: 1969}},

		// An episode of a show takes its year from a "Title (Year)"
		// folder.
		{"TV/Collateral (2018)/Collateral S01E04.mp4", tmdb.Query{Name: "Collateral", Year: 2018, Season: 1, Episode: 4, TV: true}},

		// An episode in a season folder, space- and dot-separated.
		{"TV/The Bear/Season 2/The Bear S02E06.mkv", tmdb.Query{Name: "The Bear", Season: 2, Episode: 6, TV: true}},
		{"TV/The Split/Season 3/The.Split.S03E01.mkv", tmdb.Query{Name: "The Split", Season: 3, Episode: 1, TV: true}},

		// A year between the title and the episode marker.
		{"TV/The Morning Show/Season 1/The.Morning.Show.2019.S01E10.mp4", tmdb.Query{Name: "The Morning Show", Year: 2019, Season: 1, Episode: 10, TV: true}},

		// An episode marker between hyphens, followed by its title.
		{"TV/Once Upon a Time in Northern Ireland/Season 1/Once.Upon.a.Time.in.Northern.Ireland-S01E03-So.Many.Broken.Hearts.mkv", tmdb.Query{Name: "Once Upon a Time in Northern Ireland", Season: 1, Episode: 3, TV: true}},

		// An episode title after the marker, dot-separated.
		{"TV/Seinfeld/Season 8/Seinfeld.S08E17.The.English.Patient.mkv", tmdb.Query{Name: "Seinfeld", Season: 8, Episode: 17, TV: true}},

		// A lower-case title and marker, where the folder's title has a
		// dot, which the filename's title keeps as a separator.
		{"TV/Mrs. Fletcher/Season 1/mrs.fletcher.s01e04.mp4", tmdb.Query{Name: "mrs fletcher", Season: 1, Episode: 4, TV: true}},

		// A title with a release tag which is also a word, which is only a
		// tag in upper case.
		{"Movies/The Darjeeling Limited/The Darjeeling Limited.mp4", tmdb.Query{Name: "The Darjeeling Limited"}},
		{"The.Darjeeling.Limited.LIMITED.DVDRip.mkv", tmdb.Query{Name: "The Darjeeling Limited"}},

		// A folder's title with a dot, which isn't an extension.
		{"TV/Mrs. America/Season 1/3 Shirley.mp4", tmdb.Query{Name: "Mrs America", Season: 1, Episode: 3, TV: true}},

		// The show's year is from the show's folder, which is the season
		// folder's parent.
		{"TV/Adventure Time (2010)/Season 2/Adventure Time S02E11 - The Chamber of Frozen Blades.mp4", tmdb.Query{Name: "Adventure Time", Year: 2010, Season: 2, Episode: 11, TV: true}},

		// An episode number at the start of the filename, in a season
		// folder, and the episode's title, which isn't the show's.
		{"TV/Seinfeld/Season 7/12 The Caddy.m4v", tmdb.Query{Name: "Seinfeld", Season: 7, Episode: 12, TV: true}},
		{"TV/Horizon/Season 23/16. Life Story.m4v", tmdb.Query{Name: "Horizon", Season: 23, Episode: 16, TV: true}},

		// A season and episode number, such as 203, at the start of the
		// filename, which is of the season folder's season.
		{"TV/Battlestar Galactica/Season 2/203 Fragged.mp4", tmdb.Query{Name: "Battlestar Galactica", Season: 2, Episode: 3, TV: true}},
		{"TV/The West Wing/Season 1/111 Lord John Marbury.m4v", tmdb.Query{Name: "The West Wing", Season: 1, Episode: 11, TV: true}},

		// A number which isn't of the season isn't an episode, and the
		// filename is the episode's title, which isn't the show's.
		{"TV/Battlestar Galactica/Season 1/100 Miniseries.m4v", tmdb.Query{Name: "Battlestar Galactica", TV: true}},
		{"TV/Seinfeld/Season 1/Pilot.m4v", tmdb.Query{Name: "Seinfeld", TV: true}},

		// An episode marker at the start of the filename, which can be
		// without a leading zero, and followed by an underscore.
		{"TV/Silicon Valley/Season 2/S2E07 Adult Content.mp4", tmdb.Query{Name: "Silicon Valley", Season: 2, Episode: 7, TV: true}},
		{"TV/Stranger Things/Season 2/S02E05 Chapter Five_ Dig Dug.mp4", tmdb.Query{Name: "Stranger Things", Season: 2, Episode: 5, TV: true}},
		{"TV/Ninjago/S10 - March of the Oni/S10E01_The Darkness Comes.mkv", tmdb.Query{Name: "Ninjago", Season: 10, Episode: 1, TV: true}},

		// A number before the marker, which is the episode's number in the
		// whole show, in a season folder which has a title.
		{"TV/Ninjago/S15 - Crystalized/187 - S15E07 - Ninjago City vs. Ninja.mkv", tmdb.Query{Name: "Ninjago", Season: 15, Episode: 7, TV: true}},

		// A marker for two episodes, of which the first is used.
		{"TV/Seinfeld/Season 7/Seinfeld.S07E21E22.The.Bottle.Deposit.mkv", tmdb.Query{Name: "Seinfeld", Season: 7, Episode: 21, TV: true}},

		// A marker without an E, and a marker with an x.
		{"TV/Adventure Time (2010)/Season 5/Adventure Time S0520 - Shh!.mkv", tmdb.Query{Name: "Adventure Time", Year: 2010, Season: 5, Episode: 20, TV: true}},
		{"TV/Coupling/Season 4/Coupling - (4x06) - Nine and a Half Months.mkv", tmdb.Query{Name: "Coupling", Season: 4, Episode: 6, TV: true}},

		// "Episode N", in a season folder, and after the title.
		{"TV/Happy Valley/Season 2/Episode 3.mp4", tmdb.Query{Name: "Happy Valley", Season: 2, Episode: 3, TV: true}},
		{"TV/The Day of the Triffids (1981)/Season 1/The Day of The Triffids - Episode 06.mp4", tmdb.Query{Name: "The Day of The Triffids", Year: 1981, Season: 1, Episode: 6, TV: true}},

		// A season folder with a word for its number.
		{"TV/Rev/Season One/Episode 2.mp4", tmdb.Query{Name: "Rev", Season: 1, Episode: 2, TV: true}},

		// An episode number without a season folder is of the first season.
		{"Episode 4.mp4", tmdb.Query{Season: 1, Episode: 4, TV: true}},

		// An extra in an extras folder, which has a generic name, is the
		// show's, and an extra with a title keeps it, and the number at its
		// start, which isn't an episode.
		{"TV/Hit & Miss/Extras/Extra 10.mp4", tmdb.Query{Name: "Hit & Miss"}},
		{"TV/The West Wing/Season 7/Extras/21 Institutional Memory.m4v", tmdb.Query{Name: "21 Institutional Memory"}},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := tmdb.ExtractQuery(test.path); got != test.want {
				t.Errorf("ExtractQuery(%q) = %+v, want %+v", test.path, got, test.want)
			}
		})
	}
}
