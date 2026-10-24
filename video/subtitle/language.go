package subtitle

import (
	"path/filepath"
	"strings"
	"sync"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	language "golang.org/x/text/language"
	display "golang.org/x/text/language/display"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of subtitles, such as
	// whether they're forced
	Namespace = "subtitle"
)

var (
	// flags are the parts of the name of subtitles, after the language,
	// which are what they're for, as players name them, by their keys, or
	// "" for those which aren't metadata
	flags = map[string]string{
		"forced":  "forced",
		"sdh":     "hearing_impaired",
		"hi":      "hearing_impaired",
		"cc":      "hearing_impaired",
		"default": "",
		"full":    "",
	}

	// languageNames are the languages, by their names in English and in
	// themselves, in lower case, such as english and français
	languageNames     map[string]language.Tag
	languageNamesOnce sync.Once
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// nameMetadata returns the metadata of subtitles from their name, as
// players name them, which is the name of the video, then the language, and
// then what they're for, such as Film.en.srt, Film.pt-BR.forced.srt or
// Film.English.sdh.srt: the language as dc:language, a BCP 47 tag, such as
// en or pt-BR, whether they're forced, or for the deaf and hard of hearing,
// and the name without them as dc:title, or none without a language
func nameMetadata(name string) []metadata.Metadata {
	name = filepath.Base(filepath.ToSlash(name))
	parts := strings.Split(strings.TrimSuffix(name, filepath.Ext(name)), ".")

	// The flags at the end of the name, and the language before them
	var result []metadata.Metadata
	i := len(parts) - 1
	for ; i > 0; i-- {
		key, isFlag := flags[strings.ToLower(parts[i])]
		if !isFlag {
			break
		}
		if key != "" {
			result = append(result, meta.New(Namespace, key, true))
		}
	}
	if i < 1 {
		return nil
	}
	tag, ok := parseLanguage(parts[i])
	if !ok {
		return nil
	}
	result = append(result, meta.New(dc.Namespace, dc.Language, tag.String()))

	// The title is the name of the video
	if title := strings.Join(strings.Fields(strings.ReplaceAll(strings.Join(parts[:i], "."), "_", " ")), " "); title != "" {
		result = append(result, meta.New(dc.Namespace, dc.Title, title))
	}
	return result
}

// parseLanguage returns the language of a part of the name of subtitles,
// which is a code, such as en, eng or en-GB, or a name, in English or in
// itself, such as English or Français, or false when it isn't one
func parseLanguage(part string) (language.Tag, bool) {
	if tag, exists := names()[strings.ToLower(part)]; exists {
		return tag, true
	}
	tag, err := language.Parse(part)
	if err != nil || tag == language.Und {
		return language.Und, false
	}
	return tag, true
}

// names returns the languages, by their names in English and in
// themselves, in lower case, of the languages with two-letter codes
func names() map[string]language.Tag {
	languageNamesOnce.Do(func() {
		languageNames = make(map[string]language.Tag)
		english := display.English.Languages()
		for a := 'a'; a <= 'z'; a++ {
			for b := 'a'; b <= 'z'; b++ {
				base, err := language.ParseBase(string([]rune{a, b}))
				if err != nil {
					continue
				}
				tag := language.Make(base.String())
				for _, name := range []string{english.Name(tag), display.Self.Name(tag)} {
					if name = strings.ToLower(name); name != "" {
						if _, exists := languageNames[name]; !exists {
							languageNames[name] = tag
						}
					}
				}
			}
		}
	})
	return languageNames
}
