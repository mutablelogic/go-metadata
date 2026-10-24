package postscript_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"time"

	// Packages
	postscript "github.com/mutablelogic/go-metadata/pkg/postscript"
)

// parse returns what the document's comments say about it
func parse(t *testing.T, data string) *postscript.Document {
	t.Helper()
	doc, err := postscript.Parse(bytes.NewReader([]byte(data)), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// dosEPS returns an EPS file with a DOS EPS header, with the PostScript and
// the TIFF preview
func dosEPS(ps, tiff string) []byte {
	header := make([]byte, 30)
	copy(header, []byte{0xC5, 0xD0, 0xD3, 0xC6})
	binary.LittleEndian.PutUint32(header[4:], 30)
	binary.LittleEndian.PutUint32(header[8:], uint32(len(ps)))
	binary.LittleEndian.PutUint32(header[20:], uint32(30+len(ps)))
	binary.LittleEndian.PutUint32(header[24:], uint32(len(tiff)))
	binary.LittleEndian.PutUint16(header[28:], 0xFFFF)
	return append(append(header, ps...), tiff...)
}

func Test_postscript_000(t *testing.T) {
	// A document from dvips, whose comments' values are text, and whose
	// header ends at %%EndComments
	doc := parse(t, "%!PS-Adobe-2.0\n"+
		"%%Creator: dvips 5.54 Copyright 1986, 1994 Radical Eye Software\n"+
		"%%Title: kddj.dvi\n"+
		"%%CreationDate: Fri Oct 24 07:32:48 1997\n"+
		"%%Pages: 58\n"+
		"%%BoundingBox: 0 0 612 792\n"+
		"%%EndComments\n"+
		"%%Title: Not the title\n")
	want := postscript.Document{
		Version: "2.0", Title: "kddj.dvi",
		Creator:      "dvips 5.54 Copyright 1986, 1994 Radical Eye Software",
		CreationDate: "Fri Oct 24 07:32:48 1997",
		Created:      time.Date(1997, 10, 24, 7, 32, 48, 0, time.UTC),
		Pages:        58,
	}
	if doc.Version != want.Version || doc.EPS != "" || doc.Title != want.Title || doc.Creator != want.Creator || doc.CreationDate != want.CreationDate || !doc.Created.Equal(want.Created) || doc.Pages != want.Pages {
		t.Errorf("got %+v", doc)
	}
	if box := doc.BoundingBox; box == nil || box.Width() != 612 || box.Height() != 792 {
		t.Errorf("bounding box %v", box)
	}
}

func Test_postscript_001(t *testing.T) {
	// An EPS file from Illustrator, with Mac line endings, whose values are
	// PostScript strings, with escapes, whose date is two of them, and
	// whose high resolution bounding box is preferred
	doc := parse(t, "%!PS-Adobe-3.0 EPSF-3.0\r"+
		"%%Creator: Adobe Illustrator(R) 8.0\r"+
		"%%For: (Ada Lovelace) (Analytical \\(Engines\\) Ltd)\r"+
		"%%Title: (Caf\\351 menu)\r"+
		"%%CreationDate: (4/24/01) (5:17 PM)\r"+
		"%%BoundingBox: 34 347 567 622\r"+
		"%%HiResBoundingBox: 34.5 347.25 566.5 621.75\r"+
		"%%LanguageLevel: 2\r"+
		"%%+ procset Adobe_level2_AI5 1.2 0\r"+
		"%%EndComments\r")
	if doc.Version != "3.0" || doc.EPS != "3.0" || doc.For != "Ada Lovelace Analytical (Engines) Ltd" || doc.Title != "Café menu" || doc.LanguageLevel != 2 {
		t.Errorf("got %+v", doc)
	}
	if want := time.Date(2001, 4, 24, 17, 17, 0, 0, time.UTC); !doc.Created.Equal(want) {
		t.Errorf("created %v, want %v", doc.Created, want)
	}
	if box := doc.BoundingBox; box == nil || box.Width() != 532 || box.Height() != 274.5 {
		t.Errorf("bounding box %v", box)
	}
}

func Test_postscript_002(t *testing.T) {
	// Values which are (atend) are in the trailer, after the last %%Trailer
	doc := parse(t, "%!PS-Adobe-3.0\n%%Pages: (atend)\n%%BoundingBox: (atend)\n%%EndComments\n"+
		"showpage\n%%Trailer\n%%Pages: 1\n%%Trailer\n%%Pages: 3\n%%BoundingBox: 0 0 100 50\n%%EOF\n")
	if doc.Pages != 3 || doc.BoundingBox == nil || doc.BoundingBox.Width() != 100 {
		t.Errorf("got %+v", doc)
	}
}

func Test_postscript_003(t *testing.T) {
	// Dates in the layouts which applications write
	for value, want := range map[string]time.Time{
		"Fri Oct 24 07:32:48 1997":                time.Date(1997, 10, 24, 7, 32, 48, 0, time.UTC),
		"Fri Oct  3 07:32:48 1997":                time.Date(1997, 10, 3, 7, 32, 48, 0, time.UTC),
		"9/20/2001 11:17 AM":                      time.Date(2001, 9, 20, 11, 17, 0, 0, time.UTC),
		"(4/24/01) (5:17 PM)":                     time.Date(2001, 4, 24, 17, 17, 0, 0, time.UTC),
		"Monday, August 20, 2001 3:04:05 PM":      time.Date(2001, 8, 20, 15, 4, 5, 0, time.UTC),
		"(D:20010823143000+01'00')":               time.Date(2001, 8, 23, 14, 30, 0, 0, time.UTC),
		"2001-08-23T14:30:00":                     time.Date(2001, 8, 23, 14, 30, 0, 0, time.UTC),
		"not a date":                              {},
		"(Thu Aug 23 2001) (unknown time of day)": {},
	} {
		doc := parse(t, "%!PS-Adobe-3.0\n%%CreationDate: "+value+"\n")
		if !doc.Created.Equal(want) {
			t.Errorf("%q: got %v, want %v", value, doc.Created, want)
		}
	}
}

func Test_postscript_004(t *testing.T) {
	// An EPS file with a DOS EPS header is read from its PostScript section,
	// with its TIFF preview
	data := dosEPS("%!PS-Adobe-3.0 EPSF-3.0\r\n%%Title: (Logo)\r\n%%EndComments\r\n", "MM\x00*preview")
	doc, err := postscript.Parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if doc.EPS != "3.0" || doc.Title != "Logo" || string(doc.Preview) != "MM\x00*preview" {
		t.Errorf("got %+v", doc)
	}
}

func Test_postscript_005(t *testing.T) {
	// A document which isn't PostScript, or whose DOS EPS header is outside
	// it, isn't one
	bad := dosEPS("%!PS-Adobe-3.0\n", "")
	binary.LittleEndian.PutUint32(bad[8:], 1000)
	for _, data := range [][]byte{[]byte("%PDF-1.4\n"), []byte(""), bad} {
		if _, err := postscript.Parse(bytes.NewReader(data), int64(len(data))); !errors.Is(err, postscript.ErrNotPostScript) {
			t.Errorf("%q: expected ErrNotPostScript, got %v", data, err)
		}
	}
}

func Test_postscript_006(t *testing.T) {
	// Without %%Pages, the pages are the %%Page comments, other than those of
	// a document within it, with any line endings
	doc := parse(t, "%!PS-Adobe-3.0\r\n%%EndComments\r\n%%Page: 1 1\r\n"+
		"%%BeginDocument: logo.eps\r%%Page: 1 1\r%%EndDocument\r"+
		"%%Page: 2 2\n%%Page: 3 3\n%%Trailer\n")
	if doc.Pages != 3 {
		t.Errorf("pages %d", doc.Pages)
	}

	// An EPS file has no pages
	if doc := parse(t, "%!PS-Adobe-3.0 EPSF-3.0\n%%EndComments\n%%Page: 1 1\n"); doc.Pages != 0 {
		t.Errorf("EPS pages %d", doc.Pages)
	}
}

func Test_postscript_007(t *testing.T) {
	// The fonts, which are continued with %%+, or are (atend)
	doc := parse(t, "%!PS-Adobe-3.0\n%%DocumentFonts: Times-Roman Times-Bold\n%%+ Courier\n%%Title: Fonts\n%%+ not a font\n%%EndComments\n")
	if strings.Join(doc.Fonts, ",") != "Times-Roman,Times-Bold,Courier" || doc.Title != "Fonts" {
		t.Errorf("fonts %q, title %q", doc.Fonts, doc.Title)
	}
	doc = parse(t, "%!PS-Adobe-3.0\n%%DocumentFonts: (atend)\n%%EndComments\n%%Trailer\n%%DocumentFonts: Helvetica\n%%+ Symbol\n")
	if strings.Join(doc.Fonts, ",") != "Helvetica,Symbol" {
		t.Errorf("fonts %q", doc.Fonts)
	}
}

func Test_postscript_008(t *testing.T) {
	// The paper, which the document says, or which its media's size is, or
	// which its bounding box is, in either orientation, when it's a page
	for ps, want := range map[string]string{
		"%%PaperSize: a4\n":                        "A4",
		"%%DocumentPaperSizes: Letter\n":           "Letter",
		"%%PaperSize: Foolscap\n":                  "Foolscap",
		"%%DocumentMedia: Plain 595 842 0 () ()\n": "A4",
		"%%DocumentMedia: Plain 100 100 0 () ()\n": "",
		"%%BoundingBox: 0 0 612 792\n":             "Letter",
		"%%BoundingBox: 0 0 842 595\n":             "A4",
		"%%HiResBoundingBox: 0 0 595.28 841.89\n":  "A4",
		"%%BoundingBox: 34 347 567 622\n":          "",
		"%%BoundingBox: 0 0 300 300\n":             "",
	} {
		if got := parse(t, "%!PS-Adobe-3.0\n"+ps).Paper(); got != want {
			t.Errorf("%q: got %q, want %q", ps, got, want)
		}
	}

	// An EPS file's bounding box is its drawing's
	if got := parse(t, "%!PS-Adobe-3.0 EPSF-3.0\n%%BoundingBox: 0 0 612 792\n").Paper(); got != "" {
		t.Errorf("EPS: got %q", got)
	}
}
