package subtitle

import (
	"maps"
	"testing"
)

func Test_language_000(t *testing.T) {
	// The language of subtitles, and what they're for, from their name, and
	// the name of the video as the title, as players name them
	for name, want := range map[string]map[string]string{
		"Film.en.srt":                      {"dc:language": "en", "dc:title": "Film"},
		"Film.eng.srt":                     {"dc:language": "en", "dc:title": "Film"},
		"Film.en-GB.srt":                   {"dc:language": "en-GB", "dc:title": "Film"},
		"Film.English.srt":                 {"dc:language": "en", "dc:title": "Film"},
		"Film.Français.srt":                {"dc:language": "fr", "dc:title": "Film"},
		"Film.ger.srt":                     {"dc:language": "de", "dc:title": "Film"},
		"Film.pt-BR.forced.srt":            {"dc:language": "pt-BR", "dc:title": "Film", "subtitle:forced": "true"},
		"Film.en.sdh.srt":                  {"dc:language": "en", "dc:title": "Film", "subtitle:hearing_impaired": "true"},
		"The.Office.S01E01.fr.default.srt": {"dc:language": "fr", "dc:title": "The.Office.S01E01"},
		"/videos/My_Film.de.srt":           {"dc:language": "de", "dc:title": "My Film"},
	} {
		got := map[string]string{}
		for _, m := range nameMetadata(name) {
			got[m.NS()+":"+m.Key()] = m.Value()
		}
		if !maps.Equal(got, want) {
			t.Errorf("%s: expected %v, got %v", name, want, got)
		}
	}

	// A name without a language, or which is only a language, has none
	for _, name := range []string{"Film.srt", "Film.2019.srt", "Film.DVDRip.x264.srt", "en.srt", "forced.srt", "Film.S01E01.srt"} {
		if got := nameMetadata(name); got != nil {
			t.Errorf("%s: expected none, got %v", name, got)
		}
	}
}
