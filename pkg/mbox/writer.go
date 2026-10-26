package mbox

import (
	"bytes"
	"io"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Writer writes the messages of a mailbox, a message at a time, which the
// Reader reads as they were written
type Writer struct {
	w io.Writer
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// fromLayout is the layout of the date of a From line, as C's ctime writes
// it, in UTC
const fromLayout = "Mon Jan _2 15:04:05 2006"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewWriter returns a writer of the messages of a mailbox to w
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Write writes a message: a From line, with its sender, or ???@??? when it
// has none, and its date, in UTC, then its data, whose lines end with line
// feeds, as any line ending is one, and whose lines which start with From,
// or one or more > and From, have a > before them, and a blank line after
// it, as the mboxrd format has them. A sender's spaces are underscores, as
// a From line's sender is one word.
func (w *Writer) Write(m *Message) error {
	var b bytes.Buffer

	// The From line
	sender := strings.Join(strings.Fields(m.Sender), "_")
	if sender == "" {
		sender = unknownSender
	}
	b.WriteString("From " + sender + " " + m.Date.UTC().Format(fromLayout) + "\n")

	// The lines of the message, which end with line feeds, and those which
	// would be read as From lines, or are escaped already, escaped once more
	data := m.Data
	for len(data) > 0 {
		i := bytes.IndexAny(data, "\r\n")
		line, next := data, []byte(nil)
		if i >= 0 {
			line, next = data[:i], data[i+1:]
			if data[i] == '\r' && len(next) > 0 && next[0] == '\n' {
				next = next[1:]
			}
		}
		if reEscaped.Match(line) || bytes.HasPrefix(line, []byte("From ")) {
			b.WriteByte('>')
		}
		b.Write(line)
		b.WriteByte('\n')
		data = next
	}

	// The blank line after the message
	b.WriteByte('\n')
	_, err := w.w.Write(b.Bytes())
	return err
}
