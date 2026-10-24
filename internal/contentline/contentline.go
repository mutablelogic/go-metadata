// Package contentline reads and writes the content lines of vCard and
// iCalendar, which have the same rules: a property's name, its parameters,
// and its value, folded so no line is more than 75 octets, with the
// characters of text values escaped.
// Reference: https://www.rfc-editor.org/rfc/rfc6350#section-3.2
package contentline

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Property is a content line, which is a property's group, such as item1,
// its name, in upper case, such as EMAIL, its parameters, by their names in
// upper case, such as TYPE, and its value, which is still escaped
type Property struct {
	Group  string
	Name   string
	Params map[string][]string
	Value  string
}

// Reader reads content lines, unfolded
type Reader struct {
	scanner *bufio.Scanner
	pending string
	more    bool
}

// Writer writes content lines, each ending with CRLF, and folded so no line
// is more than 75 octets
type Writer struct {
	w   io.Writer
	err error
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// lineLength is the most octets of a line, before it's folded
	lineLength = 75
)

var (
	// escaper escapes a text value, whose backslashes, commas, semicolons
	// and line breaks must be escaped
	escaper = strings.NewReplacer(`\`, `\\`, ",", `\,`, ";", `\;`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewReader returns a reader of content lines from r, which are at most the
// size, once they're unfolded
func NewReader(r io.Reader, size int) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), size)
	return &Reader{scanner: scanner}
}

// NewWriter returns a writer of content lines to w
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - WRITER

// Write writes a property, with its parameters, such as TYPE=home, and its
// value, which must be escaped, folded, and returns the first error of a
// write, so the lines can be written and the error checked once
func (w *Writer) Write(name string, params []string, value string) error {
	line := name
	for _, param := range params {
		line += ";" + param
	}
	return w.fold(line + ":" + value)
}

// Err returns the first error of a write
func (w *Writer) Err() error {
	return w.err
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Escape escapes a text value, whose backslashes, commas, semicolons and
// line breaks must be escaped
func Escape(value string) string {
	return escaper.Replace(value)
}

// Unescape unescapes a text value, whose backslashes, commas, semicolons and
// line breaks are escaped
func Unescape(value string) string {
	var result strings.Builder
	escaped := false
	for _, c := range value {
		switch {
		case escaped && (c == 'n' || c == 'N'):
			result.WriteRune('\n')
			escaped = false
		case escaped:
			result.WriteRune(c)
			escaped = false
		case c == '\\':
			escaped = true
		default:
			result.WriteRune(c)
		}
	}
	return result.String()
}

// Split returns the parts of a value, which are separated with the
// separator where it isn't escaped, such as the semicolons between the
// components of a structured value, or the commas of a list, unescaped
func Split(value string, separator rune) []string {
	var result []string
	var part strings.Builder
	escaped := false
	for _, c := range value {
		switch {
		case escaped:
			part.WriteRune('\\')
			part.WriteRune(c)
			escaped = false
		case c == '\\':
			escaped = true
		case c == separator:
			result = append(result, Unescape(part.String()))
			part.Reset()
		default:
			part.WriteRune(c)
		}
	}
	return append(result, Unescape(part.String()))
}

// Next returns the next content line, unfolded, as a line which continues
// on the next starts with a space or tab, which is removed, or io.EOF when
// there are no more. Lines which are empty aren't returned.
func (r *Reader) Next() (string, error) {
	line := r.pending
	r.pending = ""
	for r.scanner.Scan() {
		text := strings.TrimRight(r.scanner.Text(), "\r")
		switch {
		case strings.HasPrefix(text, " ") || strings.HasPrefix(text, "\t"):
			line += text[1:]
		case line == "":
			line = text
		default:
			r.pending = text
			return line, nil
		}
	}
	if err := r.scanner.Err(); err != nil {
		return "", err
	} else if line == "" {
		return "", io.EOF
	}
	return line, nil
}

// Unfold calls the function with each content line, unfolded (see
// Reader.Next). Lines are at most the size, and the function's error stops
// reading.
func Unfold(r io.Reader, size int, fn func(string) error) error {
	lines := NewReader(r, size)
	for {
		line, err := lines.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		} else if err := fn(line); err != nil {
			return err
		}
	}
}

// Parse returns the property of a content line, or false when it isn't one.
// The value starts after the first colon which isn't in a quoted parameter.
// A parameter without a name, such as HOME in vCard 2.1's TEL;HOME, is a
// TYPE, and a parameter's values are split at commas which aren't quoted.
func Parse(line string) (Property, bool) {
	quoted := false
	for i, c := range line {
		switch {
		case c == '"':
			quoted = !quoted
		case c == ':' && !quoted:
			return parseName(line[:i], line[i+1:]), true
		}
	}
	return Property{}, false
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// parseName returns the property of the name, with its group and
// parameters, and the value
func parseName(name, value string) Property {
	parts := splitQuoted(name, ';')
	property := Property{Name: strings.ToUpper(strings.TrimSpace(parts[0])), Value: value}
	if group, name, found := strings.Cut(property.Name, "."); found {
		property.Group, property.Name = group, name
	}
	for _, param := range parts[1:] {
		key, values, found := strings.Cut(param, "=")
		if !found {
			key, values = "TYPE", param
		}
		key = strings.ToUpper(strings.TrimSpace(key))
		if property.Params == nil {
			property.Params = make(map[string][]string)
		}
		for _, value := range splitQuoted(values, ',') {
			if value = strings.Trim(strings.TrimSpace(value), `"`); value != "" {
				property.Params[key] = append(property.Params[key], value)
			}
		}
	}
	return property
}

// splitQuoted returns the parts of the text, separated with the separator
// where it isn't quoted
func splitQuoted(text string, separator rune) []string {
	var result []string
	quoted, start := false, 0
	for i, c := range text {
		switch {
		case c == '"':
			quoted = !quoted
		case c == separator && !quoted:
			result = append(result, text[start:i])
			start = i + 1
		}
	}
	return append(result, text[start:])
}

// fold writes the line, folded with CRLF and a space so no line is more
// than 75 octets, without splitting a character
func (w *Writer) fold(line string) error {
	if w.err != nil {
		return w.err
	}
	var b strings.Builder
	limit := lineLength
	for len(line) > limit {
		i := limit
		for i > 0 && !utf8.RuneStart(line[i]) {
			i--
		}
		b.WriteString(line[:i])
		b.WriteString("\r\n ")
		line = line[i:]
		limit = lineLength - 1
	}
	b.WriteString(line)
	b.WriteString("\r\n")
	_, w.err = io.WriteString(w.w, b.String())
	return w.err
}
