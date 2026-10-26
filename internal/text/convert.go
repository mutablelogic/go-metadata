package text

import (
	"bufio"
	"io"
	"strings"
	"unicode"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// converter writes plain text as Markdown a character at a time, as it's
// read, holding back only what it can't write yet: the white space of a
// line which may be all white space, which decides whether the line before
// is followed by a blank line; the spaces and tabs at the end of what's
// been read of a line, which aren't written when they're at its end; the
// digits at the start of a line, which are escaped when they're a number of
// a list; and the line break after a line, which is decided by the line
// after it
type converter struct {
	w   *bufferedWriter
	err error

	// The line break after the line before, which is written once it's
	// known whether this line is blank, and whether the line before was
	// empty, without its spaces and tabs at its end
	breakPending bool
	beforeEmpty  bool

	// The characters of this line while it's all white space, which are
	// written once a character which isn't is read, or the line ends
	blank     []rune
	lineBlank bool

	// The spaces and tabs which aren't written yet, whether the body of the
	// line, after its indentation, has started, and whether its start is
	// being decided, as it's digits so far
	spaces   []rune
	body     bool
	deciding bool
	digits   []rune
}

// bufferedWriter writes to a writer in parts, so a character isn't a write
type bufferedWriter struct {
	w   io.StringWriter
	buf strings.Builder
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// bufferSize is how much Markdown is kept before it's written
	bufferSize = 4 << 10

	// nbsp is a non-breaking space, which indentation is written as, so a
	// line isn't code
	nbsp = " "
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WriteMarkdownFrom writes the text read from r as Markdown, as
// WriteMarkdown does, a character at a time, as it's read, so it stops
// reading when a write returns an error, such as when the writer has the
// part of the Markdown it keeps. Only white space, and digits at the start
// of a line, are held in memory, rather than a line, so the memory used
// doesn't grow with the length of a line.
func WriteMarkdownFrom(w io.StringWriter, r io.Reader) error {
	c := &converter{w: &bufferedWriter{w: w}, lineBlank: true}
	b := bufio.NewReaderSize(r, readSize)
	cr := false
	for c.err == nil {
		ch, _, err := b.ReadRune()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		switch {
		case ch == '\n' && cr:
			// The line feed of a carriage return and a line feed
			cr = false
		case ch == '\n' || ch == '\r':
			cr = ch == '\r'
			c.lineEnd()
		default:
			cr = false
			c.char(ch)
		}
	}
	if c.err == nil {
		c.end()
	}
	if c.err == nil {
		c.err = c.w.flush()
	}
	return c.err
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// char reads a character of a line, which isn't a line break. While the
// line is all white space, it's held, and once it isn't, the line break
// before it is written, followed by a line break which isn't a hard one,
// and the line so far.
func (c *converter) char(ch rune) {
	if c.lineBlank {
		if unicode.IsSpace(ch) {
			c.blank = append(c.blank, ch)
			return
		}
		c.lineBlank = false
		c.writeBreak(false)
		for _, b := range c.blank {
			c.bodyChar(b)
		}
		c.blank = c.blank[:0]
	}
	c.bodyChar(ch)
}

// lineEnd ends a line: the line break before it is written, as it's known
// whether this line is blank, and this line, without the spaces and tabs at
// its end, and then its line break is held, until the next line is read
func (c *converter) lineEnd() {
	blank := c.lineBlank
	if blank {
		c.writeBreak(true)
		for _, b := range c.blank {
			c.bodyChar(b)
		}
		c.blank = c.blank[:0]
	}
	empty := c.endBody()
	c.breakPending, c.beforeEmpty, c.lineBlank = true, empty, true
}

// end ends the text, whose last line has no line break after it
func (c *converter) end() {
	if c.lineBlank {
		c.writeBreak(true)
		for _, b := range c.blank {
			c.bodyChar(b)
		}
		c.blank = c.blank[:0]
	}
	c.endBody()
}

// writeBreak writes the line break after the line before, when there's one
// held: a blank line, when the line before was empty, or this line is
// blank, or else a hard line break, so the lines aren't joined
func (c *converter) writeBreak(blank bool) {
	if !c.breakPending {
		return
	}
	c.breakPending = false
	if c.beforeEmpty || blank {
		c.write("\n")
	} else {
		c.write("  \n")
	}
}

// bodyChar writes a character of a line: its indentation as non-breaking
// spaces, the spaces and tabs after its start once a character which isn't
// one follows them, the characters which are Markdown escaped, and the
// start of the line escaped when it's a heading, a list item or a rule
func (c *converter) bodyChar(ch rune) {
	if ch == ' ' || ch == '\t' {
		if c.deciding {
			c.decided()
		}
		c.spaces = append(c.spaces, ch)
		return
	}

	// The spaces and tabs before it, which are its indentation, or within
	// the line
	for _, s := range c.spaces {
		switch {
		case c.body:
			c.write(string(s))
		case s == '\t':
			c.write(strings.Repeat(nbsp, 4))
		default:
			c.write(nbsp)
		}
	}
	c.spaces = c.spaces[:0]

	// The start of the line, which is escaped when it's a heading, a list
	// item or a rule, and which is decided once it isn't digits
	if !c.body {
		c.body = true
		switch {
		case ch >= '0' && ch <= '9':
			c.deciding = true
			c.digits = append(c.digits, ch)
			return
		case strings.ContainsRune("#-+=", ch):
			c.write(`\`)
		}
	} else if c.deciding {
		switch {
		case ch >= '0' && ch <= '9':
			c.digits = append(c.digits, ch)
			return
		case ch == '.' || ch == ')':
			// A number of a list, which is escaped before its punctuation
			c.write(string(c.digits) + `\`)
			c.digits, c.deciding = c.digits[:0], false
		default:
			c.decided()
		}
	}
	c.write(escaper.Replace(string(ch)))
}

// decided writes the digits at the start of a line, which aren't a number
// of a list
func (c *converter) decided() {
	c.write(string(c.digits))
	c.digits, c.deciding = c.digits[:0], false
}

// endBody ends the body of a line, and returns true when it was empty: its
// digits are written, and the spaces and tabs at its end aren't
func (c *converter) endBody() bool {
	if c.deciding {
		c.decided()
	}
	empty := !c.body
	c.spaces, c.body = c.spaces[:0], false
	return empty
}

// write writes Markdown, which is kept until there's enough of it
func (c *converter) write(s string) {
	if c.err == nil {
		c.err = c.w.write(s)
	}
}

// write keeps Markdown, which is written once there's enough of it
func (w *bufferedWriter) write(s string) error {
	w.buf.WriteString(s)
	if w.buf.Len() >= bufferSize {
		return w.flush()
	}
	return nil
}

// flush writes the Markdown which is kept
func (w *bufferedWriter) flush() error {
	if w.buf.Len() == 0 {
		return nil
	}
	_, err := w.w.WriteString(w.buf.String())
	w.buf.Reset()
	return err
}
