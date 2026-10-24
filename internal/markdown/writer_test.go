package markdown_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	// Packages
	. "github.com/mutablelogic/go-metadata"
	markdown "github.com/mutablelogic/go-metadata/internal/markdown"
)

// markdownDocument is converted a line at a time by the markdown handler,
// with characters of two, three and four bytes
const markdownDocument = "# Café\n\nÜber naïve façades — 日本語 🎬 and more.\n\n- one\n- two\n"

// markdownLines counts the lines the handler has written
var markdownLines int

func init() {
	// The handler converts the document a line at a time, and stops when the
	// writer has the part. A document which is "fail" is an error.
	AddHandler(regexp.MustCompile("^x-test/markdown$"), "markdown", func(_ context.Context, r io.Reader, o *Opts) ([]Metadata, error) {
		if !o.HasNamespace(MarkdownNamespace) {
			return nil, nil
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		} else if string(data) == "fail" {
			return nil, errors.New("conversion failed")
		}
		w := markdown.NewWriter(o)
		for _, line := range strings.SplitAfter(string(data), "\n") {
			markdownLines++
			if _, err = w.WriteString(line); err != nil {
				break
			}
		}
		return w.Metadata(err)
	}, MarkdownNamespace)
}

// markdownPart returns the body, offset and next of the document's Markdown
// in the range
func markdownPart(t *testing.T, document string, opts ...Option) (string, uint64, *uint64, error) {
	t.Helper()
	opts = append(opts, WithNamespace(MarkdownNamespace))
	opts = append(opts, WithContentType("x-test/markdown"))
	_, meta, err := Extract(context.Background(), strings.NewReader(document), opts...)
	if err != nil {
		return "", 0, nil, err
	}
	var body string
	var offset uint64
	var next *uint64
	for _, entry := range meta {
		switch entry.NS() + string(Separator) + entry.Key() {
		case "markdown:body":
			body = entry.Any().(string)
		case "markdown:offset":
			offset = entry.Any().(uint64)
		case "markdown:next":
			n := entry.Any().(uint64)
			next = &n
		default:
			t.Errorf("unexpected key %q", entry.Key())
		}
	}
	return body, offset, next, nil
}

func Test_markdown_000(t *testing.T) {
	// Without a range, the body is all of the Markdown
	body, offset, next, err := markdownPart(t, markdownDocument)
	if err != nil {
		t.Fatal(err)
	} else if body != markdownDocument || offset != 0 || next != nil {
		t.Errorf("got %q from %d, next %v", body, offset, next)
	}

	// Without the namespace, there's none
	_, meta, err := Extract(context.Background(), strings.NewReader(markdownDocument), WithContentType("x-test/markdown"))
	if err != nil || len(meta) != 0 {
		t.Errorf("without the namespace: %v, %v", meta, err)
	}
}

func Test_markdown_001(t *testing.T) {
	// Reading a part at a time, from each next, reads all of the Markdown,
	// in whole characters, for every limit
	for limit := uint64(utf8.UTFMax); limit <= uint64(len(markdownDocument))+1; limit++ {
		var result strings.Builder
		offset := uint64(0)
		for parts := 0; ; parts++ {
			if parts > len(markdownDocument) {
				t.Fatalf("limit %d: too many parts", limit)
			}
			body, start, next, err := markdownPart(t, markdownDocument, WithRange(rng(offset, limit)))
			if err != nil {
				t.Fatalf("limit %d, offset %d: %v", limit, offset, err)
			} else if start != offset {
				t.Fatalf("limit %d: part starts at %d, not %d", limit, start, offset)
			} else if !utf8.ValidString(body) || uint64(len(body)) > limit {
				t.Fatalf("limit %d, offset %d: part %q", limit, offset, body)
			}
			result.WriteString(body)
			if next == nil {
				break
			} else if *next != offset+uint64(len(body)) || *next <= offset {
				t.Fatalf("limit %d, offset %d: next is %d", limit, offset, *next)
			}
			offset = *next
		}
		if result.String() != markdownDocument {
			t.Errorf("limit %d: read %q", limit, result.String())
		}
	}
}

func Test_markdown_002(t *testing.T) {
	// The handler stops once the writer has the part
	markdownLines = 0
	body, _, next, err := markdownPart(t, markdownDocument, WithRange(rng(0, 4)))
	if err != nil {
		t.Fatal(err)
	} else if body != "# Ca" || next == nil || *next != 4 {
		t.Errorf("got %q, next %v", body, next)
	} else if markdownLines != 1 {
		t.Errorf("the handler wrote %d lines, not 1", markdownLines)
	}
}

func Test_markdown_003(t *testing.T) {
	// An offset within a character starts at the next character
	offset := uint64(strings.Index(markdownDocument, "é") + 1)
	body, start, _, err := markdownPart(t, markdownDocument, WithRange(rng(offset, 0)))
	if err != nil {
		t.Fatal(err)
	} else if start != offset+1 || body != markdownDocument[offset+1:] {
		t.Errorf("got %q from %d", body, start)
	}

	// The end of the Markdown has an empty body, and beyond it is an error
	end := uint64(len(markdownDocument))
	if body, _, next, err := markdownPart(t, markdownDocument, WithRange(rng(end, 0))); err != nil || body != "" || next != nil {
		t.Errorf("at the end: %q, %v, %v", body, next, err)
	}
	if _, _, _, err := markdownPart(t, markdownDocument, WithRange(rng(end+1, 0))); !errors.Is(err, ErrBadParameter) {
		t.Errorf("beyond the end: %v", err)
	}
}

func Test_markdown_004(t *testing.T) {
	// A handler's error is returned
	if _, _, _, err := markdownPart(t, "fail", WithRange(rng(0, 16))); err == nil || err.Error() != "conversion failed" {
		t.Errorf("got %v", err)
	}

	// A limit holds a whole character
	if _, _, _, err := markdownPart(t, markdownDocument, WithRange(rng(0, utf8.UTFMax-1))); !errors.Is(err, ErrBadParameter) {
		t.Errorf("got %v", err)
	}

	// The largest limits are all of the Markdown
	for _, limit := range []uint64{math.MaxInt64, math.MaxUint64} {
		if body, _, next, err := markdownPart(t, markdownDocument, WithRange(rng(0, limit))); err != nil || body != markdownDocument || next != nil {
			t.Errorf("limit %d: got %q, %v, %v", limit, body, next, err)
		}
	}
}

// rng returns the range of the limit bytes from the offset, or the rest
// when the limit is zero
func rng(offset, limit uint64) string {
	if limit == 0 {
		return fmt.Sprintf("%d:", offset)
	}
	return fmt.Sprintf("%d:%d", offset, offset+limit)
}

func Test_markdown_005(t *testing.T) {
	// A range which isn't valid is a bad parameter
	for _, r := range []string{"100", "x:200", "200:100"} {
		if _, _, _, err := markdownPart(t, "text", WithRange(r)); !errors.Is(err, ErrBadParameter) {
			t.Errorf("%q: expected ErrBadParameter, got %v", r, err)
		}
	}

	// With no range, the Markdown is the first kilobyte, and : is all of it
	document := strings.Repeat("0123456789", 200)
	if body, _, next, err := markdownPart(t, document); err != nil || body != document[:1024] || next == nil || *next != 1024 {
		t.Errorf("no range: got %d bytes, next %v, %v", len(body), next, err)
	}
	if body, _, next, err := markdownPart(t, document, WithRange(":")); err != nil || body != document || next != nil {
		t.Errorf("all: got %d bytes, next %v, %v", len(body), next, err)
	}
}
