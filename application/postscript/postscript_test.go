package postscript_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"maps"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/application/postscript"
)

// named is a reader with a name, for its extension and title
type named struct {
	*bytes.Reader
	name string
}

func (r named) Name() string { return r.name }

// extract returns the content type and metadata of the named document, by
// key, with the options
func extract(t *testing.T, name string, data []byte, opts ...metadata.Option) (string, map[string]string) {
	t.Helper()
	contentType, meta, err := metadata.Extract(context.Background(), named{bytes.NewReader(data), name}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return contentType, result
}

// paletteAlphaTIFF returns a TIFF image, two pixels wide and one high, which
// is a palette image with an alpha channel, as Illustrator writes the
// preview of an EPS file
func paletteAlphaTIFF() []byte {
	order := binary.BigEndian
	const ifd, colorMap, pixels = 8, 200, 1800
	data := make([]byte, pixels+4)
	copy(data, "MM\x00*")
	order.PutUint32(data[4:], ifd)
	entries := [][4]uint32{
		{256, 3, 1, 2}, {257, 3, 1, 1}, {258, 3, 1, 8}, {259, 3, 1, 1},
		{262, 3, 1, 3}, {277, 3, 1, 2}, {284, 3, 1, 1},
		{273, 4, 1, pixels}, {279, 4, 1, 4}, {320, 3, 768, colorMap},
	}
	order.PutUint16(data[ifd:], uint16(len(entries)))
	for i, e := range entries {
		entry := data[ifd+2+i*12:]
		order.PutUint16(entry, uint16(e[0]))
		order.PutUint16(entry[2:], uint16(e[1]))
		order.PutUint32(entry[4:], e[2])
		if e[1] == 3 && e[2] == 1 {
			order.PutUint16(entry[8:], uint16(e[3]))
		} else {
			order.PutUint32(entry[8:], e[3])
		}
	}
	order.PutUint16(data[colorMap+2:], 0xFFFF)
	copy(data[pixels:], []byte{1, 255, 1, 255})
	return data
}

// dosEPS returns an EPS file with a DOS EPS header, with the PostScript and
// the TIFF preview
func dosEPS(ps string, tiff []byte) []byte {
	header := make([]byte, 30)
	copy(header, []byte{0xC5, 0xD0, 0xD3, 0xC6})
	binary.LittleEndian.PutUint32(header[4:], 30)
	binary.LittleEndian.PutUint32(header[8:], uint32(len(ps)))
	binary.LittleEndian.PutUint32(header[20:], uint32(30+len(ps)))
	binary.LittleEndian.PutUint32(header[24:], uint32(len(tiff)))
	return append(append(header, ps...), tiff...)
}

func Test_postscript_000(t *testing.T) {
	// A document, whose title is the name of the file it was made from, is
	// the title from its own name, and its size is from its bounding box
	contentType, got := extract(t, "paper.ps", []byte("%!PS-Adobe-2.0\n"+
		"%%Creator: dvips 5.54\n%%Title: paper.dvi\n%%CreationDate: Fri Oct 24 07:32:48 1997\n"+
		"%%Pages: 58\n%%BoundingBox: 0 0 612 792\n%%EndComments\n"))
	want := map[string]string{
		"metadata:kind":          "document",
		"dc:type":                "Text",
		"dc:title":               "paper",
		"dc:date":                "1997-10-24T07:32:48Z",
		"postscript:application": "dvips 5.54",
		"postscript:pages":       "58",
		"postscript:width-mm":    "215.9",
		"postscript:height-mm":   "279.4",
		"postscript:paper":       "Letter",
	}
	if contentType != "application/postscript" || !maps.Equal(got, want) {
		t.Errorf("got %q, %v\nwant %v", contentType, got, want)
	}
}

func Test_postscript_001(t *testing.T) {
	// An EPS file, or a document from Illustrator, is a drawing, by who it
	// was made for, with its title
	for _, test := range []struct{ name, ps string }{
		{"logo.eps", "%!PS-Adobe-3.0 EPSF-3.0\n%%For: (Ada Lovelace)\n%%Title: (Company Logo)\n%%EndComments\n"},
		{"logo.ai", "%!PS-Adobe-3.0\n%%Creator: Adobe Illustrator(TM) 7.0\n%%For: (Ada Lovelace)\n%%Title: (Company Logo)\n%%EndComments\n"},
	} {
		_, got := extract(t, test.name, []byte(test.ps))
		if got["metadata:kind"] != "drawing" || got["dc:type"] != "StillImage" || got["dc:creator"] != "Ada Lovelace" || got["dc:title"] != "Company Logo" {
			t.Errorf("%s: got %v", test.name, got)
		}
	}
}

func Test_postscript_002(t *testing.T) {
	// An EPS file with a DOS EPS header, which isn't text, is detected by
	// its extension, and its preview is artwork, only when the artwork
	// namespace is requested, and it has the artwork hint
	data := dosEPS("%!PS-Adobe-3.0 EPSF-3.0\r\n%%BoundingBox: 0 0 2 1\r\n%%EndComments\r\n", paletteAlphaTIFF())
	contentType, got := extract(t, "logo.eps", data)
	if contentType != "application/postscript" || got["metadata:kind"] != "drawing" || got["artwork:preview"] != "" || got["metadata:hint"] != "artwork" {
		t.Errorf("got %q, %v", contentType, got)
	}
	_, meta, err := metadata.Extract(context.Background(), named{bytes.NewReader(data), "logo.eps"}, metadata.WithNamespace(metadata.ArtworkNamespace))
	if err != nil {
		t.Fatal(err)
	} else if len(meta) != 1 || meta[0].Key() != "preview" || meta[0].Image() == nil || meta[0].Image().Bounds().Dx() != 2 {
		t.Errorf("got %v", meta)
	}
}

func Test_postscript_003(t *testing.T) {
	// A document which isn't PostScript is a bad parameter
	_, _, err := metadata.Extract(context.Background(), bytes.NewReader([]byte("not PostScript")), metadata.WithContentType("application/postscript"))
	if !errors.Is(err, metadata.ErrBadParameter) {
		t.Errorf("expected ErrBadParameter, got %v", err)
	}
}

func Test_postscript_004(t *testing.T) {
	// The fonts, and a note that the content can't be read, only when the
	// markdown namespace is requested
	data := []byte("%!PS-Adobe-3.0\n%%DocumentFonts: Times-Roman\n%%+ Courier\n%%EndComments\n")
	if _, got := extract(t, "fonts.ps", data); got["postscript:fonts"] != "Times-Roman, Courier" || got["markdown:note"] != "" {
		t.Errorf("got %v", got)
	}
	_, meta, err := metadata.Extract(context.Background(), named{bytes.NewReader(data), "fonts.ps"}, metadata.WithNamespace(metadata.MarkdownNamespace))
	if err != nil {
		t.Fatal(err)
	} else if len(meta) != 1 || meta[0].Key() != "note" {
		t.Errorf("got %v", meta)
	}
}
