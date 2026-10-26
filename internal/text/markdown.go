// Package text reads text documents as UTF-8, from whatever character set
// they're in, and writes plain text as Markdown, so it's shown as it is
package text

import (
	"html"
	"io"
	"regexp"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// escaper escapes the characters which are Markdown anywhere in a line,
	// so they're shown as they are
	escaper = strings.NewReplacer(
		`\`, `\\`, "`", "\\`", `*`, `\*`, `_`, `\_`, `[`, `\[`, `]`, `\]`,
		`<`, `\<`, `>`, `\>`, `|`, `\|`, `~`, `\~`,
	)

	// htmlTagRe matches the tags of text which is HTML
	htmlTagRe = regexp.MustCompile(`<[^>]*>`)

	// reListItem matches a line which starts as a numbered list item, such
	// as "1." or "2)", which is escaped before its punctuation
	reListItem = regexp.MustCompile(`^([0-9]+)([.)])`)
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WriteMarkdown writes the text as Markdown, a line at a time, so it's shown
// as it is: its characters which are Markdown are escaped, its line breaks
// are kept, a blank line separates its paragraphs, and its indentation is
// kept without the line being code. It stops when a write returns an error.
func WriteMarkdown(w io.StringWriter, text string) error {
	return WriteMarkdownFrom(w, strings.NewReader(text))
}

// Escape returns the text as Markdown within a line, such as a cell of a
// table, with its characters which are Markdown escaped, and its line
// breaks and tabs as spaces
func Escape(text string) string {
	text = strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t'
	}), " ")
	return escaper.Replace(text)
}

// PlainText returns text which may be HTML, such as a description, as plain
// text, without
// its tags, with its entities decoded, and its spaces collapsed
func PlainText(value string) string {
	value = html.UnescapeString(htmlTagRe.ReplaceAllString(html.UnescapeString(value), " "))
	return strings.Join(strings.Fields(value), " ")
}
