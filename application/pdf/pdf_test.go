package pdf_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	pdf "github.com/mutablelogic/go-metadata/application/pdf"
	api "github.com/pdfcpu/pdfcpu/pkg/api"
	model "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

const (
	// XMP metadata, as Acrobat writes it
	pdfXMP = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
<dc:title><rdf:Alt><rdf:li xml:lang="x-default">Annual Report</rdf:li></rdf:Alt></dc:title>
<dc:creator><rdf:Seq><rdf:li>Jane Doe</rdf:li><rdf:li>John Doe</rdf:li></rdf:Seq></dc:creator>
<dc:description><rdf:Alt><rdf:li xml:lang="x-default">The year in review</rdf:li></rdf:Alt></dc:description>
<xmp:CreatorTool>Writer</xmp:CreatorTool>
<xmp:CreateDate>2026-03-01T12:00:00Z</xmp:CreateDate>
<pdf:Producer>Acrobat</pdf:Producer>
<pdf:Keywords>report, annual</pdf:Keywords>
</rdf:Description>
</rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

	// info is document information
	info = "/Title (Quarterly Figures) /Author (Finance Team) /Subject (Q1) /Keywords (figures) /Creator (Excel) /Producer (macOS Quartz) /CreationDate (D:20260102030405+01'00')"
)

// named is a reader with a name
type named struct {
	io.Reader
	name string
}

func (r named) Name() string { return r.name }

// document returns a PDF document of one A4 page with the document
// information, and the XMP metadata when it's not empty
func document(info, xmp string) []byte {
	return documentWithPage(info, xmp, "/MediaBox [0 0 595 842]")
}

// documentWithPage returns a PDF document of one page, with the entries of
// its page's dictionary, such as its media box, with the document
// information, and the XMP metadata when it's not empty
func documentWithPage(info, xmp, page string) []byte {
	catalog := "<< /Type /Catalog /Pages 2 0 R >>"
	if xmp != "" {
		catalog = "<< /Type /Catalog /Pages 2 0 R /Metadata 5 0 R >>"
	}
	objects := []string{
		catalog,
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R " + page + " >>",
		"<< " + info + " >>",
	}
	if xmp != "" {
		objects = append(objects, fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Length %d >>\nstream\n%s\nendstream", len(xmp), xmp))
	}

	// Write the objects, and the cross-reference table with their offsets
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R /Info 4 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return buf.Bytes()
}

// entries returns the metadata of the PDF document with the name, by key
func entries(t *testing.T, name string, data []byte) (map[string]metadata.Metadata, error) {
	t.Helper()
	var r io.Reader = bytes.NewReader(data)
	if name != "" {
		r = named{r, name}
	}
	_, meta, err := metadata.Extract(context.Background(), r, metadata.WithContentType("application/pdf"))
	result := make(map[string]metadata.Metadata, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m
	}
	return result, err
}

// expect fails when the metadata doesn't have the keys with the values
func expect(t *testing.T, meta map[string]metadata.Metadata, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if m, exists := meta[key]; !exists {
			t.Errorf("%s: expected %q, got nothing", key, value)
		} else if m.Value() != value {
			t.Errorf("%s: expected %q, got %q", key, value, m.Value())
		}
	}
}

func Test_pdf_000(t *testing.T) {
	// The document information
	meta, err := entries(t, "figures.pdf", document(info, ""))
	if err != nil {
		t.Fatal(err)
	}
	expect(t, meta, map[string]string{
		"dc:title":       "Quarterly Figures",
		"dc:creator":     "Finance Team",
		"dc:description": "Q1",
		"dc:subject":     "figures",
		"pdf:creator":    "Excel",
		"pdf:producer":   "macOS Quartz",
		"pdf:version":    "1.7",
		"pdf:pages":      "1",
	})
	if created, ok := meta["dc:date"].Any().(time.Time); !ok || !created.Equal(time.Date(2026, 1, 2, 2, 4, 5, 0, time.UTC)) {
		t.Errorf("dc:date: expected 2026-01-02T02:04:05Z, got %v", meta["dc:date"].Any())
	}
}

func Test_pdf_001(t *testing.T) {
	// The XMP metadata, rather than the document information
	meta, err := entries(t, "report.pdf", document(info, pdfXMP))
	if err != nil {
		t.Fatal(err)
	}
	expect(t, meta, map[string]string{
		"dc:title":       "Annual Report",
		"dc:creator":     "Jane Doe, John Doe",
		"dc:description": "The year in review",
		"dc:subject":     "report, annual",
		"pdf:creator":    "Writer",
		"pdf:producer":   "Acrobat",
		"dc:date":        "2026-03-01T12:00:00Z",
	})
}

func Test_pdf_002(t *testing.T) {
	// When the title isn't set, or is the name of the file the document was
	// made from, it's the document's file name
	for _, info := range []string{"/Author (Jane Doe)", "/Title (Microsoft Word - Document1.docx)"} {
		meta, err := entries(t, "/doc/Scoping_Questionnaire.pdf", document(info, ""))
		if err != nil {
			t.Fatal(err)
		}
		expect(t, meta, map[string]string{
			"dc:title": "Scoping Questionnaire",
		})
	}
}

func Test_pdf_003(t *testing.T) {
	// A document which is too large isn't read, so it has only the title from
	// its name
	defer pdf.SetMaxDocumentSize(100)()
	meta, err := entries(t, "/doc/Big_Report.pdf", document(info, ""))
	if err != nil {
		t.Fatal(err)
	} else if len(meta) != 3 {
		t.Errorf("expected only the title, type and kind, got %d entries", len(meta))
	}
	expect(t, meta, map[string]string{
		"dc:title":      "Big Report",
		"metadata:kind": "document",
	})
}

func Test_pdf_004(t *testing.T) {
	// A document which isn't a PDF document is a bad parameter
	if _, err := entries(t, "", []byte("not a pdf")); !errors.Is(err, metadata.ErrBadParameter) {
		t.Errorf("expected ErrBadParameter, got %v", err)
	}
}

func Test_pdf_005(t *testing.T) {
	// The thumbnails stored for the pages are artwork, as images of the
	// pages, only when the artwork namespace is requested
	f, err := os.Open(filepath.Join("..", "..", "testfiles", "sample_with_thumbnails.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, meta, err := metadata.Extract(context.Background(), f, metadata.WithNamespace(metadata.ArtworkNamespace))
	if err != nil {
		t.Fatal(err)
	}
	artwork := make(map[string]metadata.Metadata, len(meta))
	for _, m := range meta {
		artwork[m.NS()+string(metadata.Separator)+m.Key()] = m
	}

	// The top of the first page is blue, and of the second page is red
	for key, want := range map[string]func(r, g, b uint32) bool{
		"artwork:page0001": func(r, _, b uint32) bool { return b > r },
		"artwork:page0002": func(r, _, b uint32) bool { return r > b },
	} {
		m, exists := artwork[key]
		if !exists || m.Image() == nil {
			t.Errorf("%s: expected an image", key)
			continue
		}
		bounds := m.Image().Bounds()
		if bounds.Dx() != 60 || bounds.Dy() != 85 || len(m.Bytes()) == 0 {
			t.Errorf("%s: expected a 60x85 image, got %v", key, bounds)
		}
		if r, g, b, _ := m.Image().At(bounds.Min.X+30, bounds.Min.Y+5).RGBA(); !want(r, g, b) {
			t.Errorf("%s: unexpected colour %d %d %d", key, r>>8, g>>8, b>>8)
		}
	}

	// A document without thumbnails has no artwork
	_, meta, err = metadata.Extract(context.Background(), bytes.NewReader(document(info, "")), metadata.WithContentType("application/pdf"), metadata.WithNamespace(metadata.ArtworkNamespace))
	if err != nil {
		t.Fatal(err)
	} else if len(meta) != 0 {
		t.Errorf("expected no artwork, got %d entries", len(meta))
	}
}

func Test_pdf_006(t *testing.T) {
	// A document which needs a password to be read is encrypted, with its
	// version, and the title from its name, rather than an error
	var encrypted bytes.Buffer
	conf := model.NewAESConfiguration("user", "owner", 256)
	if err := api.Encrypt(context.Background(), bytes.NewReader(document(info, "")), &encrypted, conf); err != nil {
		t.Fatal(err)
	}
	meta, err := entries(t, "/doc/Bank_Statement.pdf", encrypted.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	expect(t, meta, map[string]string{
		"pdf:encrypted": "true",
		"pdf:version":   "1.7",
		"dc:title":      "Bank Statement",
	})
	if len(meta) != 5 {
		t.Errorf("expected 5 entries, with the type and kind, got %d", len(meta))
	}

	// It has no artwork, rather than an error
	_, artwork, err := metadata.Extract(context.Background(), bytes.NewReader(encrypted.Bytes()), metadata.WithContentType("application/pdf"), metadata.WithNamespace(metadata.ArtworkNamespace))
	if err != nil {
		t.Fatal(err)
	} else if len(artwork) != 0 {
		t.Errorf("expected no artwork, got %d entries", len(artwork))
	}
}

func Test_pdf_007(t *testing.T) {
	// A document which isn't valid, such as one with a malformed name in its
	// document information, has the document information which can be read
	meta, err := entries(t, "memo.pdf", document("/display_urn:schemas-microsoft-com:office:office#Author (Jane) /Title (Mentor Profile) /Author (Jane Doe)", ""))
	if err != nil {
		t.Fatal(err)
	}
	expect(t, meta, map[string]string{
		"dc:title":   "Mentor Profile",
		"dc:creator": "Jane Doe",
		"pdf:pages":  "1",
	})
}

func Test_pdf_008(t *testing.T) {
	// A document which is encrypted is read with its user or owner
	// password, and is encrypted, and with a wrong password, it's only
	// encrypted, with an error
	var encrypted bytes.Buffer
	conf := model.NewAESConfiguration("user", "owner", 256)
	if err := api.Encrypt(context.Background(), bytes.NewReader(document(info, "")), &encrypted, conf); err != nil {
		t.Fatal(err)
	}
	extract := func(password string) (map[string]metadata.Metadata, error) {
		_, meta, err := metadata.Extract(context.Background(), bytes.NewReader(encrypted.Bytes()), metadata.WithContentType("application/pdf"), metadata.WithPassword(password))
		result := make(map[string]metadata.Metadata, len(meta))
		for _, m := range meta {
			result[m.NS()+string(metadata.Separator)+m.Key()] = m
		}
		return result, err
	}
	for _, password := range []string{"user", "owner"} {
		meta, err := extract(password)
		if err != nil {
			t.Fatalf("%s: %v", password, err)
		}
		expect(t, meta, map[string]string{
			"dc:title":      "Quarterly Figures",
			"dc:creator":    "Finance Team",
			"pdf:pages":     "1",
			"pdf:encrypted": "true",
		})
	}
	meta, err := extract("wrong")
	if !errors.Is(err, metadata.ErrBadParameter) {
		t.Errorf("expected ErrBadParameter, got %v", err)
	}
	expect(t, meta, map[string]string{
		"pdf:encrypted": "true",
	})
	if _, exists := meta["dc:creator"]; exists {
		t.Error("expected no dc:creator with a wrong password")
	}
}

func Test_pdf_009(t *testing.T) {
	// A scanned page, which is one large image the shape of the page, has
	// that image downscaled as artwork, to the size of the A4 page at 150 dpi
	// by default, which is larger than the scan, so it's kept, and at most
	// the largest dimension, 1024 pixels by default, or at the dpi which is
	// set, however large, and a page of text has none
	for _, test := range []struct {
		opts          []metadata.Option
		width, height int
	}{
		{nil, 724, 1024},
		{[]metadata.Option{metadata.WithMaxImageDimension(2048)}, 1000, 1414},
		{[]metadata.Option{metadata.WithImageDPI(72)}, 595, 842},
		{[]metadata.Option{metadata.WithImageDPI(300)}, 1000, 1414},
		{[]metadata.Option{metadata.WithMaxImageDimension(640)}, 452, 640},
		{[]metadata.Option{metadata.WithImageDPI(100), metadata.WithMaxImageDimension(640)}, 826, 1169},
	} {
		f, err := os.Open(filepath.Join("..", "..", "testfiles", "sample_scanned.pdf"))
		if err != nil {
			t.Fatal(err)
		}
		_, meta, err := metadata.Extract(context.Background(), f, append(test.opts, metadata.WithNamespace(metadata.ArtworkNamespace))...)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		artwork := make(map[string]metadata.Metadata, len(meta))
		for _, m := range meta {
			artwork[m.NS()+string(metadata.Separator)+m.Key()] = m
		}
		if _, exists := artwork["artwork:page0002"]; exists {
			t.Error("artwork:page0002: expected no artwork for a page of text")
		}

		// The top of the first page is blue, and of the third page is red
		for key, want := range map[string]func(r, g, b uint32) bool{
			"artwork:page0001": func(r, _, b uint32) bool { return b > r },
			"artwork:page0003": func(r, _, b uint32) bool { return r > b },
		} {
			m, exists := artwork[key]
			if !exists || m.Image() == nil {
				t.Errorf("%s: expected an image", key)
				continue
			}
			bounds := m.Image().Bounds()
			if bounds.Dx() != test.width || bounds.Dy() != test.height || m.Value() != "image/jpeg" {
				t.Errorf("%s: expected a %dx%d JPEG image, got %v %s", key, test.width, test.height, bounds, m.Value())
			}
			if r, g, b, _ := m.Image().At(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+bounds.Dy()/10).RGBA(); !want(r, g, b) {
				t.Errorf("%s: unexpected colour %d %d %d", key, r>>8, g>>8, b>>8)
			}
		}
	}

	// The largest dimension, and the dpi, are at least one
	for _, n := range []int{0, -1} {
		if _, _, err := metadata.Extract(context.Background(), bytes.NewReader(nil), metadata.WithMaxImageDimension(n)); !errors.Is(err, metadata.ErrBadParameter) {
			t.Errorf("max image dimension %d: expected ErrBadParameter, got %v", n, err)
		}
		if _, _, err := metadata.Extract(context.Background(), bytes.NewReader(nil), metadata.WithImageDPI(n)); !errors.Is(err, metadata.ErrBadParameter) {
			t.Errorf("image dpi %d: expected ErrBadParameter, got %v", n, err)
		}
	}
}

func Test_pdf_010(t *testing.T) {
	// The text of the pages is Markdown, read a part at a time, and a
	// document without text, or which needs a password, without one, has no
	// Markdown, and with the password, it has
	data, err := os.ReadFile(filepath.Join("..", "..", "testfiles", "sample_with_thumbnails.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	markdown := func(data []byte, opts ...metadata.Option) (map[string]string, error) {
		opts = append(opts, metadata.WithContentType("application/pdf"), metadata.WithNamespace(metadata.MarkdownNamespace))
		_, meta, err := metadata.Extract(context.Background(), bytes.NewReader(data), opts...)
		result := make(map[string]string, len(meta))
		for _, m := range meta {
			result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
		}
		return result, err
	}

	// A part of the Markdown
	got, err := markdown(data, metadata.WithRange("0:8"))
	if err != nil {
		t.Fatal(err)
	} else if got["markdown:body"] != "Page One" || got["markdown:next"] != "8" {
		t.Errorf("expected Page One, and the next part at 8, got %v", got)
	}

	// No Markdown without text, but a note which says so
	if got, err := markdown(document(info, "")); err != nil || len(got) != 1 || !strings.Contains(got["markdown:note"], "no text") {
		t.Errorf("expected only a note, got %v, %v", got, err)
	}

	// An encrypted document has no Markdown without its password, and has
	// with it
	var encrypted bytes.Buffer
	if err := api.Encrypt(context.Background(), bytes.NewReader(data), &encrypted, model.NewAESConfiguration("user", "owner", 256)); err != nil {
		t.Fatal(err)
	}
	if got, err := markdown(encrypted.Bytes()); err != nil || len(got) != 0 {
		t.Errorf("without a password: expected no markdown, got %v, %v", got, err)
	}
	if got, err := markdown(encrypted.Bytes(), metadata.WithPassword("user"), metadata.WithRange(":")); err != nil || got["markdown:body"] != "Page One\n\n---\n\nPage Two" {
		t.Errorf("with the password: got %v, %v", got, err)
	}
	if _, err := markdown(encrypted.Bytes(), metadata.WithPassword("wrong")); !errors.Is(err, metadata.ErrBadParameter) {
		t.Errorf("with a wrong password: expected ErrBadParameter, got %v", err)
	}
}

func Test_pdf_011(t *testing.T) {
	// The width and height of the first page, in millimetres, which is its
	// crop box, or else its media box, turned by its rotation
	for _, test := range []struct {
		page, width, height string
	}{
		{"/MediaBox [0 0 595 842]", "209.9", "297"},
		{"/MediaBox [0 0 612 792] /Rotate 90", "279.4", "215.9"},
		{"/MediaBox [0 0 700 900] /CropBox [50 50 662 842]", "215.9", "279.4"},
	} {
		meta, err := entries(t, "page.pdf", documentWithPage(info, "", test.page))
		if err != nil {
			t.Fatalf("%s: %v", test.page, err)
		}
		expect(t, meta, map[string]string{"pdf:width-mm": test.width, "pdf:height-mm": test.height})
	}
}

func Test_pdf_hints(t *testing.T) {
	// hints returns the hints of the document
	hints := func(t *testing.T, r io.Reader) string {
		t.Helper()
		_, meta, err := metadata.Extract(context.Background(), r, metadata.WithContentType("application/pdf"))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range meta {
			if m.NS() == metadata.MetadataNamespace && m.Key() == metadata.HintKey {
				return m.Value()
			}
		}
		return ""
	}
	file := func(t *testing.T, name string) io.Reader {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "testfiles", name))
		if err != nil {
			t.Fatal(err)
		}
		return bytes.NewReader(data)
	}

	// A page which uses a font has text, and a page which draws an image
	// without one is scanned, and a document with pages has artwork
	if got := hints(t, bytes.NewReader(documentWithPage("", "", "/MediaBox [0 0 612 792] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >>"))); got != "artwork,markdown" {
		t.Errorf("text: expected artwork,markdown, got %q", got)
	}
	if got := hints(t, bytes.NewReader(documentWithPage("", "", "/MediaBox [0 0 612 792]"))); got != "artwork" {
		t.Errorf("blank: expected artwork, got %q", got)
	}
	if got := hints(t, file(t, "sample_scanned.pdf")); got != "artwork,markdown,ocr" {
		t.Errorf("sample_scanned.pdf: expected artwork,markdown,ocr, got %q", got)
	}
	if got := hints(t, file(t, "sample_with_thumbnails.pdf")); !strings.Contains(got, "markdown") || !strings.Contains(got, "artwork") {
		t.Errorf("sample_with_thumbnails.pdf: expected artwork and markdown, got %q", got)
	}
}
