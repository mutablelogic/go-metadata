package text

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// limitWriter keeps what's written, up to a limit, and then returns an
// error, as a writer of a part of the Markdown does
type limitWriter struct {
	strings.Builder
	limit int
}

var errLimit = errors.New("limit")

func (w *limitWriter) WriteString(s string) (int, error) {
	if w.limit > 0 && w.Len()+len(s) > w.limit {
		w.Builder.WriteString(s[:w.limit-w.Len()])
		return 0, errLimit
	}
	return w.Builder.WriteString(s)
}

// countingReader counts what's read
type countingReader struct {
	io.ReadSeeker
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.ReadSeeker.Read(p)
	r.n += int64(n)
	return n, err
}

func Test_stream_000(t *testing.T) {
	// The text is read as UTF-8, from its byte order mark, or UTF-8 when its
	// start is, or else Windows-1252, and UTF-8 which isn't valid later is
	// replaced
	for _, test := range []struct {
		name string
		data string
		want string
	}{
		{"utf-8", "Café", "Café"},
		{"utf-8 bom", "\xEF\xBB\xBFCafé", "Café"},
		{"utf-16le bom", "\xFF\xFEC\x00a\x00f\x00\xE9\x00", "Café"},
		{"utf-16be bom", "\xFE\xFF\x00C\x00a\x00f\x00\xE9", "Café"},
		{"windows-1252", "Caf\xe9 \x93quoted\x94", "Café “quoted”"},
		{"invalid later", strings.Repeat("a", sniffSize) + "\xff", strings.Repeat("a", sniffSize) + "�"},
		{"empty", "", ""},
	} {
		data, err := io.ReadAll(NewReader(strings.NewReader(test.data)))
		if err != nil {
			t.Fatal(err)
		} else if string(data) != test.want {
			t.Errorf("%s: expected %q, got %q", test.name, test.want, data)
		}
	}

	// A character which is cut off between reads isn't replaced
	long := strings.Repeat("é", readSize)
	data, err := io.ReadAll(NewReader(strings.NewReader(long)))
	if err != nil {
		t.Fatal(err)
	} else if string(data) != long {
		t.Error("expected the text as it is")
	}
}

func Test_stream_001(t *testing.T) {
	// A code block written as it's read is the code block of the text, with
	// a fence longer than its backticks, carriage returns before line feeds
	// left out, and a line feed at its end
	for _, text := range []string{
		"", "a", "a\n", "a\r\nb", "a\rb\n", "```\nquoted\n```", "a ```` b", strings.Repeat("x`", readSize) + "\n",
	} {
		var b strings.Builder
		if err := WriteCodeBlock(&b, "go", strings.NewReader(text)); err != nil {
			t.Fatal(err)
		}
		if want := codeBlock("go", text); b.String() != want {
			t.Errorf("%q: expected %q, got %q", text, want, b.String())
		}
	}

	// It isn't read further than the part which is kept, after the text is
	// read for its backticks
	text := strings.Repeat("line\n", 1<<20)
	r := &countingReader{ReadSeeker: strings.NewReader(text)}
	w := &limitWriter{limit: 100}
	if err := WriteCodeBlock(w, "", r); !errors.Is(err, errLimit) {
		t.Fatalf("expected the limit, got %v", err)
	}
	if r.n > int64(len(text))+sniffSize+readSize {
		t.Errorf("read %d bytes of %d", r.n, len(text))
	}
}

func Test_stream_002(t *testing.T) {
	// Plain text written as it's read, a line at a time, stops reading once
	// the writer has its part
	text := strings.Repeat("a line of text\n", 1<<20)
	r := &countingReader{ReadSeeker: strings.NewReader(text)}
	w := &limitWriter{limit: 100}
	if err := WriteMarkdownFrom(w, NewReader(r)); !errors.Is(err, errLimit) {
		t.Fatalf("expected the limit, got %v", err)
	}
	if r.n > sniffSize+2*readSize {
		t.Errorf("read %d bytes, for %d of Markdown", r.n, w.Len())
	}

	// The lines are those of WriteMarkdown
	for _, text := range []string{
		"", "a", "a\n", "a\r\nb", "a\rb", "a\r", "\n\n", "a\n\nb\n", " \na", "a \n \nb", "a\t \nb",
		"  indented\n\tand a tab", "# heading\n- list\n+ plus\n= rule", "12. item\n3) item\n12 not\n42", "1.5 times",
		"some *emphasis* and a [link](x) and <tag>", "\f\na", "a\n\u00a0\nb", "  \u00a0x", "a  b  ",
	} {
		var streamed, whole strings.Builder
		if err := WriteMarkdownFrom(&streamed, strings.NewReader(text)); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
		// As WriteMarkdown wrote a line at a time, which is without the
		// spaces and tabs at its end before it's known whether it's empty
		for i, line := range lines {
			line = strings.TrimRight(line, " \t")
			end := "  \n"
			switch {
			case i == len(lines)-1:
				end = ""
			case line == "" || strings.TrimSpace(lines[i+1]) == "":
				end = "\n"
			}
			whole.WriteString(markdownLine(line) + end)
		}
		if streamed.String() != whole.String() {
			t.Errorf("%q: expected %q, got %q", text, whole.String(), streamed.String())
		}
	}
}

func Test_stream_004(t *testing.T) {
	// A text without line breaks stops being read once the writer has its
	// part, rather than being read a line at a time
	text := strings.Repeat("words and more words ", 1<<20)
	r := &countingReader{ReadSeeker: strings.NewReader(text)}
	w := &limitWriter{limit: 100}
	if err := WriteMarkdownFrom(w, NewReader(r)); !errors.Is(err, errLimit) {
		t.Fatalf("expected the limit, got %v", err)
	}
	if r.n > sniffSize+2*readSize {
		t.Errorf("read %d bytes of %d, for %d of Markdown", r.n, len(text), w.Len())
	}
}

func Test_stream_005(t *testing.T) {
	// When the start of the text can't be read, the reader returns the error
	errRead := errors.New("read")
	data, err := io.ReadAll(NewReader(io.MultiReader(strings.NewReader("partial"), errorReader{errRead})))
	if !errors.Is(err, errRead) {
		t.Errorf("expected the error, got %q, %v", data, err)
	}
}

// errorReader returns an error when it's read
type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func Test_stream_003(t *testing.T) {
	// The text is written without its carriage returns before line feeds,
	// including one between reads
	text := strings.Repeat("x", readSize-1) + "\r\nend\r"
	var b strings.Builder
	if err := WriteLF(&b, bytes.NewReader([]byte(text))); err != nil {
		t.Fatal(err)
	}
	if want := strings.Repeat("x", readSize-1) + "\nend\r"; b.String() != want {
		t.Errorf("expected the text without its carriage return before a line feed")
	}
}

// markdownLine returns a line of text as Markdown, as WriteMarkdown did a
// line at a time, which the conversion a character at a time is tested with,, with its characters which
// are Markdown escaped, and its indentation as non-breaking spaces, so it
// isn't code
func markdownLine(line string) string {
	body := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(body)]
	indent = strings.ReplaceAll(strings.ReplaceAll(indent, "\t", "    "), " ", " ")

	// Escape the characters which are Markdown anywhere, and then those
	// which are only Markdown at the start of a line, such as a heading,
	// a list item or a rule
	body = escaper.Replace(body)
	if m := reListItem.FindStringSubmatch(body); m != nil {
		body = m[1] + `\` + body[len(m[1]):]
	} else if body != "" && strings.ContainsRune("#-+=", rune(body[0])) {
		body = `\` + body
	}
	return indent + body
}
