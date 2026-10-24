package html

import (
	"context"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	html "golang.org/x/net/html"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	Namespace = "html"

	// maxTitle is the most of a title which is kept, in characters, as a title
	// whose end tag is broken, such as by an email's quoted-printable line
	// breaks, runs on through the rest of the document
	maxTitle = 256
)

var (
	reContentType = regexp.MustCompile(`^(?:text/html|text/xhtml|application/xhtml\+xml)$`)
)

// dcMeta maps the names of meta elements to Dublin Core element names. A name listed
// earlier for a key is preferred, so the page's own description is preferred
// to the one for social media. The language of the html element, and the
// canonical link, are preferred to any of these.
var dcMeta = []struct {
	name, key string
}{
	{"description", dc.Description},
	{"og:description", dc.Description},
	{"twitter:description", dc.Description},
	{"keywords", dc.Subject},
	{"author", dc.Creator},
	{"article:author", dc.Creator},
	{"og:title", dc.Title},
	{"twitter:title", dc.Title},
	{"date", dc.Date},
	{"article:published_time", dc.Date},
	{"datecreated", dc.Date},
	{"language", dc.Language},
	{"og:locale", dc.Language},
	{"og:url", dc.Identifier},
}

// ignoredMeta are the names of meta elements which aren't metadata of the
// document, but how it's displayed or served, or which verify the ownership
// of its site
var ignoredMeta = map[string]bool{
	"charset":                      true,
	"color-scheme":                 true,
	"content-type":                 true,
	"facebook-domain-verification": true,
	"format-detection":             true,
	"google-site-verification":     true,
	"handheldfriendly":             true,
	"mobile-web-app-capable":       true,
	"mobileoptimized":              true,
	"msvalidate.01":                true,
	"referrer":                     true,
	"robots":                       true,
	"theme-color":                  true,
	"viewport":                     true,
	"x-ua-compatible":              true,
	"yandex-verification":          true,
}

// ignoredMetaPrefixes are the prefixes of the names of meta elements which
// are ignored, as they're how the document is displayed on a platform
var ignoredMetaPrefixes = []string{
	"apple-mobile-web-app-",
	"msapplication-",
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	metadata.AddHandler(reContentType, Namespace, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindWebPage)
		doc, err := parse(r)
		if err != nil {
			return kind, err
		}

		// Collect the language, the title element, the canonical link, and the
		// meta elements, outside of scripts and styles
		var title, lang, canonical string
		fields := make(map[string]string)
		walkHTML(doc, func(node *html.Node) {
			switch strings.ToLower(node.Data) {
			case "html":
				lang = attr(node, "lang")
			case "title":
				if title == "" {
					title = cleanTitle(textContent(node))
				}
			case "link":
				if canonical == "" && slices.Contains(strings.Fields(strings.ToLower(attr(node, "rel"))), "canonical") {
					canonical = attr(node, "href")
				}
			case "meta":
				if key, value := metaField(node); key != "" && !isIgnoredMeta(key) {
					if _, exists := fields[key]; !exists {
						fields[key] = value
					}
				}
			}
		})

		// A locale is a language, with a hyphen rather than an underscore,
		// such as en-US for en_US
		if locale, exists := fields["og:locale"]; exists {
			fields["og:locale"] = strings.ReplaceAll(locale, "_", "-")
		}

		// Map the meta elements to Dublin Core keys, preferring the title
		// element for the title, the language of the html element, and the
		// canonical link for the identifier, and put the rest in the html
		// namespace
		entries := make(map[string]metadata.Metadata)
		for key, value := range map[string]string{
			dc.Title:      title,
			dc.Language:   lang,
			dc.Identifier: canonical,
		} {
			if value != "" {
				entries[entryKey(dc.Namespace, key)] = meta.New(dc.Namespace, key, value)
			}
		}
		mapped := make(map[string]bool)
		for _, m := range dcMeta {
			if value, exists := fields[m.name]; exists {
				mapped[m.name] = true
				if _, exists := entries[entryKey(dc.Namespace, m.key)]; !exists {
					entries[entryKey(dc.Namespace, m.key)] = meta.New(dc.Namespace, m.key, value)
				}
			}
		}

		// In order of name, so the first of the names with the same key is
		// kept, and skipping names which can't be a key
		for _, field := range slices.Sorted(maps.Keys(fields)) {
			if mapped[field] {
				continue
			}
			key := metaKey(field)
			if _, exists := entries[entryKey(Namespace, key)]; exists {
				continue
			}
			if entry := meta.New(Namespace, key, fields[field]); entry != nil {
				entries[entryKey(Namespace, key)] = entry
			}
		}
		return append(slices.Collect(maps.Values(entries)), kind...), nil
	}, dc.Namespace, Namespace, metadata.MetadataNamespace)
}

// entryKey returns the key of an entry in a namespace, such as dc:title
func entryKey(ns, key string) string {
	return ns + string(meta.Separator) + key
}

// metaKey returns the name of a meta element as a key, replacing each
// character which can't be in a key with a hyphen, such as og-image for
// og:image. The key is only valid when the name starts with a letter, and
// isn't longer than a key can be.
func metaKey(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, name)
}

// cleanTitle returns a title with its spaces collapsed
func cleanTitle(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	if runes := []rune(title); len(runes) > maxTitle {
		title = string(runes[:maxTitle])
		if i := strings.LastIndex(title, " "); i > 0 {
			title = title[:i]
		}
		title += "…"
	}
	return title
}

// isIgnoredMeta returns true when a meta element with the name isn't
// metadata of the document
func isIgnoredMeta(name string) bool {
	if ignoredMeta[name] {
		return true
	}
	for _, prefix := range ignoredMetaPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
