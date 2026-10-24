// Package markdown is the writer of the content of a document as Markdown,
// which a handler of the markdown namespace writes, and which keeps the part
// of it in the range set via metadata.WithRange
package markdown

import (
	"errors"
	"strings"
	"unicode/utf8"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Writer is written the content of a document as Markdown, by a handler of
// the markdown namespace, and keeps the part of it in the range set via
// metadata.WithRange. Once it has the part, a write returns an error, so the
// handler can stop, rather than converting the rest of the document. A
// handler returns the part with Metadata, which also takes the handler's
// error.
type Writer struct {
	offset, limit uint64
	pos           uint64
	buf           []byte
	more          bool
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// errFull is returned by a write once the writer has the part of the
// Markdown in its range
var errFull = errors.New("the markdown's range is full")

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewWriter returns a writer which keeps the part of the Markdown in the
// range set via metadata.WithRange, or the first kilobyte when it wasn't set
func NewWriter(o *metadata.Opts) *Writer {
	offset, limit := o.Range()
	return &Writer{offset: offset, limit: limit}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Write keeps the part of the data in the range. It returns an error once it
// has the part, and there's more of the Markdown, so the handler can stop.
func (w *Writer) Write(data []byte) (int, error) {
	if w.more {
		return 0, errFull
	}
	n := 0

	// Skip the data before the offset
	if w.pos < w.offset {
		skip := int(min(uint64(len(data)), w.offset-w.pos))
		w.pos += uint64(skip)
		data, n = data[skip:], skip
	}

	// Keep the data up to the limit, and one more byte, so there's more.
	// What's kept is at most the data, as the buffer is within the limit.
	if w.limit > 0 && uint64(len(w.buf))+uint64(len(data)) > w.limit {
		keep := int(min(w.limit-uint64(len(w.buf))+1, uint64(len(data))))
		w.buf = append(w.buf, data[:keep]...)
		w.pos += uint64(keep)
		w.more = true
		return n + keep, errFull
	}
	w.buf = append(w.buf, data...)
	w.pos += uint64(len(data))
	return n + len(data), nil
}

// WriteString keeps the part of the string in the range, as Write does
func (w *Writer) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Metadata returns the part of the Markdown, and the handler's error, which
// is nil when it stopped as the writer had the part. The part is
// markdown:body; markdown:offset is the byte it starts at, when the range
// has an offset, which is moved forward to the start of a character; and
// markdown:next is the offset of the next part, when there's more. Returns
// an error when the offset is beyond the end of the Markdown.
func (w *Writer) Metadata(err error) ([]metadata.Metadata, error) {
	if err != nil && !errors.Is(err, errFull) {
		return nil, err
	} else if w.pos < w.offset {
		return nil, metadata.ErrBadParameter.Withf("offset %d is beyond the end of the markdown, at %d", w.offset, w.pos)
	}

	// The part starts and ends with whole characters
	data := w.buf
	start := 0
	if w.offset > 0 {
		for start < len(data) && start < utf8.UTFMax && !utf8.RuneStart(data[start]) {
			start++
		}
	}
	end := len(data)
	if w.more {
		end = start + validEnd(data[start:w.limit])
	}

	result := []metadata.Metadata{
		meta.New(metadata.MarkdownNamespace, "body", string(data[start:end])),
	}
	if w.offset > 0 {
		result = append(result, meta.New(metadata.MarkdownNamespace, "offset", w.offset+uint64(start)))
	}
	if w.more {
		result = append(result, meta.New(metadata.MarkdownNamespace, "next", w.offset+uint64(end)))
	}
	return result, nil
}

// CodeBlock returns the text as a fenced Markdown code block in the
// language, with a fence of more backticks than any run of them in the text,
// so the text can't end it
func CodeBlock(language, text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	fence := strings.Repeat("`", max(3, longestRun(text, '`')+1))
	return fence + language + "\n" + text + fence + "\n"
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// longestRun returns the length of the longest run of the character in the
// text
func longestRun(text string, c byte) int {
	longest, run := 0, 0
	for i := 0; i < len(text); i++ {
		if text[i] == c {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return longest
}

// validEnd returns the end of the data without a character which is cut off
// at its end, which is that character's start
func validEnd(data []byte) int {
	for i := len(data) - 1; i >= 0 && i >= len(data)-utf8.UTFMax; i-- {
		if utf8.RuneStart(data[i]) {
			if utf8.FullRune(data[i:]) {
				return len(data)
			}
			return i
		}
	}
	return len(data)
}
