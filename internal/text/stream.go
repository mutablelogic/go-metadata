package text

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	// Packages
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// utf8Reader reads text which is UTF-8, with any sequences which aren't
// valid UTF-8 replaced, keeping a character which is cut off at the end of
// a read until the next
type utf8Reader struct {
	r       io.Reader
	pending []byte
	out     []byte
	err     error
}

// errReader returns an error when it's read
type errReader struct {
	err error
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// sniffSize is how much of the start of a text is read to know whether
	// it's UTF-8, or else Windows-1252
	sniffSize = 64 << 10

	// readSize is how much of a text is read at a time
	readSize = 32 << 10

	// hasTextSize is how much of the start of a text is read to know
	// whether it has text
	hasTextSize = 4 << 10
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// NewReader returns a reader of the text as UTF-8, as it's read, so a text
// of any size is read a part at a time, rather than in memory: from the
// character set of its byte order mark, or UTF-8 when its start is valid
// UTF-8, or else Windows-1252, as Decode does. Sequences of UTF-8 which
// aren't valid, after its start, are replaced. When its start can't be
// read, the reader returns the error.
func NewReader(r io.Reader) io.Reader {
	b := bufio.NewReaderSize(r, sniffSize)
	head, err := b.Peek(sniffSize)
	if err != nil && !errors.Is(err, io.EOF) {
		// The text can't be read, which the reader returns
		return errReader{err}
	}
	switch {
	case bytes.HasPrefix(head, bomUTF8):
		b.Discard(len(bomUTF8))
		return &utf8Reader{r: b}
	case bytes.HasPrefix(head, bomUTF16LE), bytes.HasPrefix(head, bomUTF16BE):
		return transform.NewReader(b, unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder())
	case len(head) == sniffSize && utf8.Valid(head[:validEnd(head)]), len(head) < sniffSize && utf8.Valid(head):
		return &utf8Reader{r: b}
	default:
		return transform.NewReader(b, charmap.Windows1252.NewDecoder())
	}
}

// WriteCodeBlock writes the text read from r as a fenced Markdown code block
// in the language, as CodeBlock returns it, as it's read, so it stops
// reading when a write returns an error. Its fence is longer than any run
// of backticks in the text, which it's read for first, when r can seek, or
// else it's read in memory, up to MaxBytes.
func WriteCodeBlock(w io.StringWriter, language string, r io.Reader) error {
	seeker, ok := r.(io.Seeker)
	if !ok {
		text, err := Read(r)
		if err != nil {
			return err
		}
		_, err = w.WriteString(codeBlock(language, text))
		return err
	}

	// The longest run of backticks, and whether the text is empty, or ends
	// with a line feed
	start, err := seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	longest, run, empty, lastLF := 0, 0, true, false
	buf := make([]byte, readSize)
	text := NewReader(r)
	for {
		n, err := text.Read(buf)
		for _, c := range buf[:n] {
			if c == '`' {
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		}
		if n > 0 {
			empty, lastLF = false, buf[n-1] == '\n'
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	if _, err := seeker.Seek(start, io.SeekStart); err != nil {
		return err
	}

	// The block, with the text's carriage returns before line feeds left
	// out, and a line feed at its end
	fence := strings.Repeat("`", max(3, longest+1))
	if _, err := w.WriteString(fence + language + "\n"); err != nil {
		return err
	}
	if err := WriteLF(w, NewReader(r)); err != nil {
		return err
	}
	if !empty && !lastLF {
		if _, err := w.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = w.WriteString(fence + "\n")
	return err
}

// Read returns the error
func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}

// Read implements io.Reader, with the sequences which aren't valid UTF-8
// replaced
func (u *utf8Reader) Read(p []byte) (int, error) {
	for len(u.out) == 0 {
		if u.err != nil {
			if len(u.pending) > 0 {
				u.out, u.pending = bytes.ToValidUTF8(u.pending, []byte("�")), nil
				break
			}
			return 0, u.err
		}
		buf := make([]byte, readSize)
		n, err := u.r.Read(buf)
		data := append(u.pending, buf[:n]...)
		end := len(data)
		if err == nil {
			end = validEnd(data)
		}
		u.out, u.pending, u.err = bytes.ToValidUTF8(data[:end], []byte("�")), clone(data[end:]), err
	}
	n := copy(p, u.out)
	u.out = u.out[n:]
	return n, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// WriteLF writes the text read from r to w, without its carriage returns
// before line feeds, as it's read, so it stops reading when a write returns
// an error
func WriteLF(w io.StringWriter, r io.Reader) error {
	buf := make([]byte, readSize)
	cr := false
	for {
		n, err := r.Read(buf)
		var b strings.Builder
		for _, c := range buf[:n] {
			if cr && c != '\n' {
				b.WriteByte('\r')
			}
			cr = c == '\r'
			if !cr {
				b.WriteByte(c)
			}
		}
		if b.Len() > 0 {
			if _, werr := w.WriteString(b.String()); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			if cr {
				_, werr := w.WriteString("\r")
				return werr
			}
			return nil
		} else if err != nil {
			return err
		}
	}
}

// codeBlock returns the text as a fenced Markdown code block, as CodeBlock
// in the markdown package does
func codeBlock(language, text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	longest, run := 0, 0
	for i := 0; i < len(text); i++ {
		if text[i] == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	return fence + language + "\n" + text + fence + "\n"
}

// clone returns a copy of the data, which is kept after the buffer it's
// from is used again
func clone(data []byte) []byte {
	return append([]byte(nil), data...)
}

// HasText returns true when the start of the text has a character which
// isn't white space, and no NUL, as text which is binary has, which is
// read without moving r, when it can be read at an offset, as a handler's
// input can
func HasText(r io.Reader) bool {
	buf := make([]byte, hasTextSize)
	var n int
	if ra, ok := r.(io.ReaderAt); ok {
		n, _ = ra.ReadAt(buf, 0)
	} else {
		n, _ = io.ReadFull(r, buf)
	}
	data := buf[:n]
	return bytes.IndexByte(data, 0) < 0 && len(bytes.TrimSpace(data)) > 0
}
