package postscript

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	// Packages
	charmap "golang.org/x/text/encoding/charmap"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// maxComments is how much of the start, and of the end, of a document
	// is read for its comments, which are in its header, or its trailer
	// when a value in its header is (atend)
	maxComments = 64 << 10

	// maxPreview is the largest preview of an EPS file which is read
	maxPreview = 32 << 20

	// dosHeaderSize is the size of the DOS EPS header
	dosHeaderSize = 30

	// maxLine is the longest line which is read when a document's %%Page
	// comments are counted, as a line of image data can be long
	maxLine = 1 << 20
)

var (
	// ErrNotPostScript is returned for a document which isn't PostScript
	ErrNotPostScript = errors.New("not a PostScript document")

	// dosMagic starts an EPS file with a DOS EPS header
	dosMagic = []byte{0xC5, 0xD0, 0xD3, 0xC6}

	// reVersion matches the first line of a document which follows the
	// conventions, such as %!PS-Adobe-3.0 EPSF-3.0
	reVersion = regexp.MustCompile(`^%!PS-Adobe-([0-9.]+)(?:\s+EPSF-([0-9.]+))?`)

	// dateLayouts are the layouts of the creation dates which applications
	// write, such as dvips's, Illustrator's and Windows's
	dateLayouts = []string{
		"Mon Jan _2 15:04:05 2006",
		"Mon Jan _2 15:04:05 MST 2006",
		"Mon Jan _2 15:04 2006",
		"1/2/2006 3:04 PM",
		"1/2/06 3:04 PM",
		"1/2/2006 15:04",
		"1/2/2006 15:04:05",
		"Monday, January 2, 2006 3:04:05 PM",
		"Monday, January 2, 2006 3:04 PM",
		"Mon, 2 Jan 2006 15:04:05",
		"2 Jan 2006 15:04:05",
		"January 2, 2006 3:04 PM",
		"January 2, 2006",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"1/2/2006",
		"1/2/06",
	}

	// rePDFDate matches a date as PDF writes it, such as
	// D:20010823143000+01'00', which some applications write
	rePDFDate = regexp.MustCompile(`^D:(\d{4})(\d{2})?(\d{2})?(\d{2})?(\d{2})?(\d{2})?`)
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Parse returns what the comments of the PostScript document of the size
// say about it, from its header, and its trailer, for a value which is
// (atend) in its header, or ErrNotPostScript when it isn't a PostScript
// document. An EPS file with a DOS EPS header is read from its PostScript
// section, and its TIFF preview is read too.
func Parse(r io.ReaderAt, size int64) (*Document, error) {
	doc := new(Document)

	// An EPS file with a DOS EPS header, which is followed by its sections
	var header [dosHeaderSize]byte
	if n, _ := r.ReadAt(header[:], 0); n == dosHeaderSize && bytes.Equal(header[:4], dosMagic) {
		psOffset, psSize := int64(binary.LittleEndian.Uint32(header[4:])), int64(binary.LittleEndian.Uint32(header[8:]))
		tiffOffset, tiffSize := int64(binary.LittleEndian.Uint32(header[20:])), int64(binary.LittleEndian.Uint32(header[24:]))
		if psOffset+psSize > size {
			return nil, ErrNotPostScript
		}
		if tiffSize > 0 && tiffSize <= maxPreview && tiffOffset+tiffSize <= size {
			doc.Preview = make([]byte, tiffSize)
			if _, err := r.ReadAt(doc.Preview, tiffOffset); err != nil {
				doc.Preview = nil
			}
		}
		r, size = io.NewSectionReader(r, psOffset, psSize), psSize
	}

	// The header, which ends at %%EndComments, or the first line which isn't
	// a comment
	start := make([]byte, min(size, maxComments))
	if _, err := r.ReadAt(start, 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	lines := splitLines(start)
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "%!PS") {
		return nil, ErrNotPostScript
	}
	if m := reVersion.FindStringSubmatch(lines[0]); m != nil {
		doc.Version, doc.EPS = m[1], m[2]
	}
	atend := doc.comments(lines[1:], true)

	// The trailer, which has the values which were (atend), after the last
	// %%Trailer
	if atend && size > 0 {
		offset := max(0, size-maxComments)
		end := make([]byte, size-offset)
		if _, err := r.ReadAt(end, offset); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if i := bytes.LastIndex(end, []byte("%%Trailer")); i >= 0 {
			doc.comments(splitLines(end[i:]), false)
		}
	}

	// The pages, when the document doesn't say how many it has, from its
	// %%Page comments, unless it's an EPS file, which doesn't have pages
	if doc.Pages == 0 && doc.EPS == "" {
		doc.Pages = countPages(io.NewSectionReader(r, 0, size))
	}
	return doc, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// comments reads the comments of the document's header, which end at
// %%EndComments, or the first line which isn't a comment, or of its trailer,
// and returns true when a value is (atend), which is in its trailer. A
// comment can be continued on the lines after it, which start with %%+.
func (doc *Document) comments(lines []string, header bool) bool {
	atend, last := false, ""
	for _, line := range lines {
		if header && (line == "%%EndComments" || !strings.HasPrefix(line, "%")) {
			break
		}
		if rest, found := strings.CutPrefix(line, "%%+"); found {
			doc.continuation(last, strings.TrimSpace(rest))
			continue
		}
		key, isAtend := doc.comment(line)
		atend, last = atend || isAtend, key
	}
	return atend
}

// continuation reads the continuation of a comment, such as more fonts
func (doc *Document) continuation(key, value string) {
	if key == "DocumentFonts" {
		doc.Fonts = append(doc.Fonts, strings.Fields(value)...)
	}
}

// comment reads a comment of the document, such as %%Title: Report, and
// returns its key, and true when its value is (atend), which is in its
// trailer
func (doc *Document) comment(line string) (string, bool) {
	key, value, found := strings.Cut(strings.TrimPrefix(line, "%%"), ":")
	if !found || !strings.HasPrefix(line, "%%") {
		return "", false
	}
	if value = strings.TrimSpace(value); value == "(atend)" {
		return key, true
	}
	switch key {
	case "Title":
		doc.Title = text(value)
	case "Creator":
		doc.Creator = text(value)
	case "For":
		doc.For = text(value)
	case "CreationDate":
		doc.CreationDate = text(value)
		doc.Created = parseDate(doc.CreationDate)
	case "Pages":
		if fields := strings.Fields(value); len(fields) > 0 {
			doc.Pages, _ = strconv.Atoi(fields[0])
		}
	case "LanguageLevel":
		doc.LanguageLevel, _ = strconv.Atoi(value)
	case "BoundingBox":
		// The high resolution bounding box is preferred
		if doc.BoundingBox == nil {
			doc.BoundingBox = boundingBox(value)
		}
	case "HiResBoundingBox":
		if box := boundingBox(value); box != nil {
			doc.BoundingBox = box
		}
	case "DocumentFonts":
		doc.Fonts = append(doc.Fonts, strings.Fields(value)...)
	case "PaperSize", "DocumentPaperSizes":
		if fields := strings.Fields(text(value)); len(fields) > 0 {
			doc.PaperSize = fields[0]
		}
	case "DocumentMedia":
		// A medium's name, which can be any name, such as Plain, and its
		// width and height, which are its size, unless its name is one
		if fields := strings.Fields(value); len(fields) >= 3 && doc.PaperSize == "" {
			width, errw := strconv.ParseFloat(fields[1], 64)
			height, errh := strconv.ParseFloat(fields[2], 64)
			if errw == nil && errh == nil {
				doc.PaperSize = paperOfSize(width, height)
			}
		}
	}
	return key, false
}

// countPages returns how many %%Page comments the document has, which
// start its pages, other than those of the documents within it, between
// %%BeginDocument and %%EndDocument, such as an EPS file it includes
func countPages(r io.Reader) int {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLine)
	scanner.Split(scanLines)
	pages, depth := 0, 0
	for scanner.Scan() {
		line := scanner.Bytes()
		switch {
		case bytes.HasPrefix(line, []byte("%%BeginDocument")):
			depth++
		case bytes.HasPrefix(line, []byte("%%EndDocument")):
			depth = max(0, depth-1)
		case depth == 0 && bytes.HasPrefix(line, []byte("%%Page:")):
			pages++
		}
	}
	return pages
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

// splitLines returns the lines of the data, which end with a carriage
// return, a line feed, or both, as a Mac's, Unix's or Windows's do
func splitLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

// text returns the value of a comment as text, which is one or more
// PostScript strings, such as (4/24/01) (5:17 PM), joined by spaces, or
// otherwise the value as it is, as UTF-8, or as Windows-1252 when it isn't
// UTF-8, as a document's text is in the character set of the computer which
// made it
func text(value string) string {
	value = toUTF8(value)
	if !strings.HasPrefix(value, "(") {
		return value
	}
	var parts []string
	for value = strings.TrimSpace(value); strings.HasPrefix(value, "("); value = strings.TrimSpace(value) {
		s, rest := psString(value)
		parts = append(parts, s)
		value = rest
	}
	if value != "" {
		parts = append(parts, value)
	}
	return toUTF8(strings.TrimSpace(strings.Join(parts, " ")))
}

// toUTF8 returns the text as it is, when it's UTF-8, or else as Windows-1252
func toUTF8(text string) string {
	if utf8.ValidString(text) {
		return text
	}
	decoded, _ := charmap.Windows1252.NewDecoder().String(text)
	return decoded
}

// psString returns the PostScript string at the start of the value, which
// starts with a parenthesis, without its escapes, and the rest of the value
func psString(value string) (string, string) {
	var b strings.Builder
	depth := 0
	for i := 0; i < len(value); i++ {
		switch c := value[i]; {
		case c == '\\' && i+1 < len(value):
			i++
			switch e := value[i]; e {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '0', '1', '2', '3', '4', '5', '6', '7':
				// An octal character, of up to three digits
				j := i
				for j < len(value) && j < i+3 && value[j] >= '0' && value[j] <= '7' {
					j++
				}
				n, _ := strconv.ParseUint(value[i:j], 8, 8)
				b.WriteByte(byte(n))
				i = j - 1
			default:
				b.WriteByte(e)
			}
		case c == '(':
			if depth > 0 {
				b.WriteByte(c)
			}
			depth++
		case c == ')':
			if depth--; depth == 0 {
				return b.String(), value[i+1:]
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), ""
}

// boundingBox returns the bounding box of a comment's value, which is four
// numbers, or nil when it isn't
func boundingBox(value string) *BoundingBox {
	fields := strings.Fields(value)
	if len(fields) != 4 {
		return nil
	}
	var n [4]float64
	for i, field := range fields {
		var err error
		if n[i], err = strconv.ParseFloat(field, 64); err != nil {
			return nil
		}
	}
	return &BoundingBox{LLX: n[0], LLY: n[1], URX: n[2], URY: n[3]}
}

// parseDate returns a creation date as a time, in UTC, or zero when it
// can't be read. A date without a time zone is in UTC.
func parseDate(value string) time.Time {
	value = strings.Join(strings.Fields(value), " ")
	if m := rePDFDate.FindStringSubmatch(value); m != nil {
		n := [6]int{0, 1, 1, 0, 0, 0}
		for i, s := range m[1:] {
			if s != "" {
				n[i], _ = strconv.Atoi(s)
			}
		}
		return time.Date(n[0], time.Month(n[1]), n[2], n[3], n[4], n[5], 0, time.UTC)
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
