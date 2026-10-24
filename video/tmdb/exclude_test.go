package tmdb

import (
	"testing"
	"time"

	// Packages
	schema "github.com/mutablelogic/go-metadata/pkg/tmdb/schema"
)

func Test_exclude_000(t *testing.T) {
	// The names cameras, phones and DVDs give videos, and of extras
	for _, name := range []string{
		"MOV00062.MP4", "MVI_1234.MOV", "DSC_0001.MOV", "DSCF0001.AVI", "GOPR0001.MP4",
		"GX010001.MP4", "GH010042.MP4", "DJI_0001.MP4", "C0001.MP4", "MAH00001.MP4",
		"P1000123.MOV", "IMG_1234.MOV", "VID_20230101_123456.mp4", "VID-20230101-WA0001.mp4", "VID-20210703-WA0003.mp4",
		"PXL_20230101_123456789.mp4", "20230101_123456.mp4", "00001.MTS",
		"Screen Recording 2023-01-01 at 12.00.00.mov", "Screen_Recording_20230101_123456.mp4", "RPReplay_Final1234.MP4",
		"/Videos/Holiday/img_0042.mov", "/Movies/The Matrix (1999)/VIDEO_TS/VTS_01_1.VOB",
		"VIDEO_TS.VOB", "VTS_02_3.mpg", "The Matrix.vob",
		"The.Matrix.1999.Sample.mkv", "Season 4 Trailer.mp4", "Movies/Round Midnight/Featurettes/Interview.mkv",
	} {
		if !exclude(name, 0) {
			t.Errorf("%q isn't excluded", name)
		}
	}

	// And the names of films and episodes, including those named for a year
	for _, name := range []string{
		"Alien (1979).mp4", "1917.mkv", "2012.mp4", "Clip.mp4", "Movie.mp4",
		"The Office S02E01.mkv", "Blade Runner 2049.mp4", "/Media/Movies/C.R.A.Z.Y. (2005)/C.R.A.Z.Y..mkv",
	} {
		if exclude(name, 0) {
			t.Errorf("%q is excluded", name)
		}
	}

	// A video shorter than minDuration, whose duration is known
	if !exclude("Movie.mp4", time.Minute) || exclude("Movie.mp4", time.Hour) {
		t.Error("duration")
	}
}

func Test_normalizeTitle_000(t *testing.T) {
	for title, want := range map[string]string{
		"The Hunt for Red October":  "hunt for red october",
		"Mission: Impossible":       "mission impossible",
		"Schindler’s List":          "schindlers list",
		"Ascenseur pour l'échafaud": "ascenseur pour lechafaud",
		"Ascenseur pour L'Echafaud": "ascenseur pour lechafaud",
		"Moolaadé":                  "moolaade",
		"A Beautiful Mind":          "a beautiful mind",
		"The":                       "the",
		"...":                       "",
	} {
		if got := normalizeTitle(title); got != want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", title, got, want)
		}
	}
}

func Test_episodeTitle_000(t *testing.T) {
	show := &schema.TVShowDetails{TVShow: schema.TVShow{Name: "Seinfeld"}}
	for _, test := range []struct {
		episode schema.TVEpisode
		want    string
	}{
		{schema.TVEpisode{Name: "The Stake Out", SeasonNumber: 1, EpisodeNumber: 3}, "Seinfeld – Season 1 – The Stake Out"},
		{schema.TVEpisode{SeasonNumber: 1, EpisodeNumber: 3}, "Seinfeld – Season 1 – Episode 3"},
		{schema.TVEpisode{Name: "The Highlights of 100", EpisodeNumber: 1}, "Seinfeld – Specials – The Highlights of 100"},
	} {
		if got := episodeTitle(show, &test.episode); got != test.want {
			t.Errorf("got %q, want %q", got, test.want)
		}
	}
}
