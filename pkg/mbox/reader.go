// Package mbox reads mailboxes in the mbox format, such as Eudora's .mbx
// files, and Thunderbird's and Unix mail's mailboxes, which are messages,
// each after a line which starts with From, the sender, and the date it was
// received, a message at a time
package mbox

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Message is a message in a mailbox
type Message struct {
	// The sender of the message, from its From line, which is empty when it
	// isn't known, as Eudora writes it, ???@???
	Sender string

	// When the message was received, from its From line, in UTC, as the
	// time zone isn't known
	Date time.Time

	// The message, with line feeds, and the lines which start with From,
	// which are escaped as >From, as they are. A message which is longer
	// than MaxMessageSize is cut short.
	Data []byte
}

// Reader reads the messages of a mailbox, a message at a time
type Reader struct {
	scanner *bufio.Scanner
	next    *Message
	err     error
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// ContentType is the content type of a mailbox
	ContentType = "application/mbox"

	// MaxMessageSize is the most of a message which is read
	MaxMessageSize = 32 << 20

	// maxLine is the longest line which is read
	maxLine = 4 << 20

	// unknownSender is the sender of a message whose sender isn't known, as
	// Eudora writes it
	unknownSender = "???@???"
)

var (
	// reFrom matches the line before a message, which is From, the sender,
	// and the date, such as From ???@??? Tue Feb 12 10:02:33 2002, which
	// is only that line when its date can be read, as a line of a message
	// can start with From too
	reFrom = regexp.MustCompile(`^From (\S+) +(.+?)\s*$`)

	// reEscaped matches a line of a message which starts with From, escaped
	// with one or more >
	reEscaped = regexp.MustCompile(`^>+From `)

	// dateLayouts are the layouts of the dates of From lines, which are
	// usually as C's ctime writes them
	dateLayouts = []string{
		"Mon Jan _2 15:04:05 2006",
		"Mon Jan _2 15:04:05 -0700 2006",
		"Mon Jan _2 15:04:05 MST 2006",
		"Mon Jan _2 15:04 2006",
		"Mon, _2 Jan 2006 15:04:05 -0700",
	}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewReader returns a reader of the messages of the mailbox, which can have
// any line endings, such as a Mac's carriage returns
func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLine)
	scanner.Split(scanLines)
	return &Reader{scanner: scanner}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Next returns the next message of the mailbox, or io.EOF when there are no
// more. Text before the first From line isn't a message.
func (r *Reader) Next() (*Message, error) {
	if r.err != nil {
		return nil, r.err
	}
	var data bytes.Buffer
	for r.scanner.Scan() {
		line := r.scanner.Bytes()
		if m, ok := fromLine(line); ok {
			current := r.next
			r.next = m
			if current != nil {
				current.Data = data.Bytes()
				return current, nil
			}
			continue
		}
		if r.next == nil || data.Len() >= MaxMessageSize {
			continue
		}
		if reEscaped.Match(line) {
			line = line[1:]
		}
		data.Write(line[:min(len(line), MaxMessageSize-data.Len())])
		data.WriteByte('\n')
	}

	// The last message
	if r.err = r.scanner.Err(); r.err == nil {
		r.err = io.EOF
	}
	if current := r.next; current != nil {
		r.next = nil
		current.Data = data.Bytes()
		return current, nil
	}
	return nil, r.err
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// fromLine returns the message which the line is before, with its sender
// and date, or false when it isn't a From line, as its date can't be read
func fromLine(line []byte) (*Message, bool) {
	if !bytes.HasPrefix(line, []byte("From ")) {
		return nil, false
	}
	m := reFrom.FindSubmatch(line)
	if m == nil {
		return nil, false
	}
	date, ok := parseDate(string(m[2]))
	if !ok {
		return nil, false
	}
	message := &Message{Date: date}
	if sender := string(m[1]); sender != unknownSender {
		message.Sender = sender
	}
	return message, true
}

// parseDate returns the date of a From line, which can be followed by other
// words, such as remote from host, in UTC, or false when it can't be read
func parseDate(value string) (time.Time, bool) {
	fields := strings.Fields(value)
	for n := len(fields); n >= 4; n-- {
		candidate := strings.Join(fields[:n], " ")
		for _, layout := range dateLayouts {
			if date, err := time.Parse(layout, candidate); err == nil {
				return date.UTC(), true
			}
		}
	}
	return time.Time{}, false
}

// scanLines splits data into lines, which end with a carriage return, a
// line feed, or both, as a Mac's, Unix's or Windows's do
func scanLines(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		advance := i + 1
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			advance++
		} else if data[i] == '\r' && i+1 == len(data) && !atEOF {
			// The line feed of a CRLF may be in the next data
			return 0, nil, nil
		}
		return advance, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
