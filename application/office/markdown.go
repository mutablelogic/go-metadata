package office

import (
	"archive/zip"
	"io"
	"regexp"
	"slices"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// markdownTable is a table of a document, by row and cell, which is written
// as Markdown, with its first row as its header, with its name, such as a
// sheet's, the number of times its last cell, and row, are repeated, or the
// columns its last cell spans, its last cell's value when it has no text,
// and the size of its text
type markdownTable struct {
	rows      [][]string
	name      string
	repeat    int
	rowRepeat int
	value     string
	size      int
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// markdownEscapes escapes the characters of text which are Markdown's
var markdownEscapes = strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "`", "\\`", "[", `\[`, "]", `\]`, "<", `\<`, ">", `\>`)

// lineStartRe matches the start of a line which Markdown would take for a
// heading, a quote, or a bulleted list item, and orderedRe a numbered list
// item, whose number is followed by its delimiter
var (
	lineStartRe = regexp.MustCompile(`^(#|>|[-+*]\s)`)
	orderedRe   = regexp.MustCompile(`^\d+[.)]\s`)
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// add adds the text to the last cell of the table, after a space when the
// cell has text, such as a cell's second paragraph
func (t *markdownTable) add(text string) {
	if len(t.rows) == 0 || len(t.rows[len(t.rows)-1]) == 0 || text == "" {
		return
	}
	row := t.rows[len(t.rows)-1]
	if cell := row[len(row)-1]; cell != "" {
		text = cell + " " + text
	}
	row[len(row)-1] = text
}

// markdown returns the table as Markdown, with its first row as its header,
// and every row with as many cells as the widest, or an empty string when
// it has no text
func (t *markdownTable) markdown() string {
	width, empty := 0, true
	for _, row := range t.rows {
		width = max(width, len(row))
		for _, cell := range row {
			empty = empty && cell == ""
		}
	}
	if width == 0 || empty {
		return ""
	}
	var b strings.Builder
	line := func(cells []string) {
		b.WriteString("|")
		for i := range width {
			cell := ""
			if i < len(cells) {
				cell = strings.ReplaceAll(cells[i], "|", `\|`)
			}
			b.WriteString(" " + cell + " |")
		}
		b.WriteString("\n")
	}
	line(t.rows[0])
	line(slices.Repeat([]string{"---"}, width))
	for _, row := range t.rows[1:] {
		line(row)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// plain returns the table's text, its cells separated by spaces, and its
// rows by semicolons, as a table within a table's cell has it
func (t *markdownTable) plain() string {
	rows := make([]string, 0, len(t.rows))
	for _, row := range t.rows {
		if text := strings.Join(strings.Fields(strings.Join(row, " ")), " "); text != "" {
			rows = append(rows, text)
		}
	}
	return strings.Join(rows, "; ")
}

// addMarkdown adds a document's content as Markdown to the entries, a part
// at a time, by converting the archive, when it's requested, and there's a
// converter
func addMarkdown(entries map[string]metadata.Metadata, archive *zip.Reader, o *metadata.Opts, convert func(*zip.Reader, io.StringWriter) error) error {
	if convert == nil || !o.HasNamespace(metadata.MarkdownNamespace) {
		return nil
	}
	w := imarkdown.NewWriter(o)
	parts, err := w.Metadata(convert(archive, w))
	if err != nil {
		return err
	}
	for _, part := range parts {
		entries[part.NS()+string(metadata.Separator)+part.Key()] = part
	}
	return nil
}

// escapeMarkdown returns the text with the characters which are Markdown's
// escaped
func escapeMarkdown(text string) string {
	return markdownEscapes.Replace(text)
}

// escapeLineStart returns the text with its start escaped, when Markdown
// would take it for a heading, a quote or a list item: its first
// character, or a number's delimiter, such as 1\. for 1., as a digit can't
// be escaped
func escapeLineStart(text string) string {
	if loc := orderedRe.FindStringIndex(text); loc != nil {
		delimiter := loc[1] - 2
		return text[:delimiter] + `\` + text[delimiter:]
	} else if lineStartRe.MatchString(text) {
		return `\` + text
	}
	return text
}

// last returns the last of the items, or nil when there are none
func last[T any](items []*T) *T {
	if len(items) == 0 {
		return nil
	}
	return items[len(items)-1]
}
