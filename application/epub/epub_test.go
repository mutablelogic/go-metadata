package epub_test

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/application/epub"
	testassert "github.com/mutablelogic/go-metadata/internal/testassert"
)

var (
	assert  = testassert.Assert
	require = testassert.Require
)

const (
	epubType = "application/epub+zip"

	// The container, which has the path of the package document
	epubContainerXML = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

	// An EPUB 2 package, with a contributor which isn't an author, and the
	// author in its sorting form, as some books have it, a cover meta, and
	// dates, the publication's second
	epub2OPF = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="bookid">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
<dc:identifier id="bookid">urn:uuid:273fd756-62f2-4858-8d67-99e08f24bba9</dc:identifier>
<dc:identifier opf:scheme="ASIN">B07BW911F7</dc:identifier>
<dc:identifier opf:scheme="ISBN">urn:isbn:978-1-5098-6807-0</dc:identifier>
<dc:contributor opf:role="bkp">eCore v0.9.12.751</dc:contributor>
<dc:creator opf:file-as="Carreyrou, John" opf:role="aut">Carreyrou, John</dc:creator>
<dc:creator opf:role="ill">Jane Doe</dc:creator>
<dc:date opf:event="modification">2020-01-01</dc:date>
<dc:date opf:event="publication">2018-05-31</dc:date>
<dc:language>en</dc:language>
<dc:publisher>Pan Macmillan</dc:publisher>
<dc:subject>Business</dc:subject>
<dc:subject>Medicine</dc:subject>
<dc:description>&lt;p&gt;The full inside story of the breathtaking rise &amp;amp; fall of Theranos.&lt;/p&gt;</dc:description>
<dc:title>Bad Blood: Secrets and Lies in a Silicon Valley Startup</dc:title>
<meta name="cover" content="cover-image"/>
</metadata>
<manifest>
<item href="Image00003.png" id="cover-image" media-type="image/png"/>
<item href="text00000.html" id="text" media-type="application/xhtml+xml"/>
</manifest>
</package>`

	// An EPUB 3 package, whose authors' roles refine them, whose cover is a
	// cover image in a directory with a space, and whose date is a time
	epub3OPF = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="pub-id">9780306406157</dc:identifier>
<dc:title>The Phoenix Project</dc:title>
<dc:creator id="creator1">Gene Kim</dc:creator>
<meta refines="#creator1" property="role" scheme="marc:relators">aut</meta>
<dc:creator id="creator2">Kevin Behr</dc:creator>
<meta refines="#creator2" property="role" scheme="marc:relators">aut</meta>
<dc:creator id="creator3">An Editor</dc:creator>
<meta refines="#creator3" property="role" scheme="marc:relators">edt</meta>
<dc:date>2013-01-10T00:00:00Z</dc:date>
<meta property="dcterms:modified">2020-01-01T00:00:00Z</meta>
</metadata>
<manifest>
<item href="cover%20art/front.png" id="c" media-type="image/png" properties="cover-image"/>
</manifest>
</package>`
)

// document returns a zip archive with the files
func document(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// named is a reader with a name
type named struct {
	io.Reader
	name string
}

func (r named) Name() string { return r.name }

// coverPNG returns a picture, as a cover
func coverPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 40, 60))))
	return buf.Bytes()
}

// epubEntries returns the metadata of the book, by key
func epubEntries(t *testing.T, name string, data []byte, opts ...metadata.Option) (map[string]metadata.Metadata, error) {
	t.Helper()
	_, meta, err := metadata.Extract(t.Context(), named{bytes.NewReader(data), name}, append(opts, metadata.WithContentType(epubType))...)
	result := make(map[string]metadata.Metadata)
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m
	}
	return result, err
}

func TestEpubMetadata(t *testing.T) {
	cover := string(coverPNG(t))

	t.Run("EPUB2", func(t *testing.T) {
		// Its authors, rather than its other creators and contributors, its
		// publication date, its ISBN rather than its other identifiers, and
		// its description as plain text
		book := document(t, map[string]string{"META-INF/container.xml": epubContainerXML, "OEBPS/content.opf": epub2OPF, "OEBPS/Image00003.png": cover})
		meta, err := epubEntries(t, "/books/Bad Blood.epub", book)
		require.NoError(t, err)
		assert.Equal(t, "Bad Blood: Secrets and Lies in a Silicon Valley Startup", meta["dc:title"].Value())
		assert.Equal(t, "Carreyrou, John", meta["dc:creator"].Value())
		assert.Equal(t, time.Date(2018, 5, 31, 0, 0, 0, 0, time.UTC), meta["dc:date"].Any())
		assert.Equal(t, "9781509868070", meta["dc:identifier"].Value())
		assert.Equal(t, "en", meta["dc:language"].Value())
		assert.Equal(t, "Pan Macmillan", meta["dc:publisher"].Value())
		assert.Equal(t, "Business, Medicine", meta["dc:subject"].Value())
		assert.Equal(t, "The full inside story of the breathtaking rise & fall of Theranos.", meta["dc:description"].Value())
		assert.NotContains(t, meta, "artwork:cover")

		// The cover, when artwork is requested
		meta, err = epubEntries(t, "/books/Bad Blood.epub", book, metadata.WithNamespace("artwork"))
		require.NoError(t, err)
		require.Contains(t, meta, "artwork:cover")
		assert.Equal(t, 40, meta["artwork:cover"].Image().Bounds().Dx())
		assert.Len(t, meta, 1)
	})

	t.Run("EPUB3", func(t *testing.T) {
		// Its authors by their refined roles, and its cover image, whose path
		// is escaped
		book := document(t, map[string]string{"META-INF/container.xml": epubContainerXML, "OEBPS/content.opf": epub3OPF, "OEBPS/cover art/front.png": cover})
		meta, err := epubEntries(t, "/books/phoenix.epub", book, metadata.WithNamespace("dc", "artwork"))
		require.NoError(t, err)
		assert.Equal(t, "The Phoenix Project", meta["dc:title"].Value())
		assert.Equal(t, "Gene Kim, Kevin Behr", meta["dc:creator"].Value())
		assert.Equal(t, time.Date(2013, 1, 10, 0, 0, 0, 0, time.UTC), meta["dc:date"].Any())
		assert.Equal(t, "9780306406157", meta["dc:identifier"].Value())
		assert.Contains(t, meta, "artwork:cover")
	})

	t.Run("Renditions", func(t *testing.T) {
		// A book with several renditions has the first's metadata, which is
		// its default rendition
		container := `<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles>
<rootfile full-path="default/content.opf" media-type="application/oebps-package+xml"/>
<rootfile full-path="other/content.opf" media-type="application/oebps-package+xml"/>
</rootfiles></container>`
		opf := func(title string) string {
			return `<package xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>` + title + `</dc:title></metadata></package>`
		}
		meta, err := epubEntries(t, "/b.epub", document(t, map[string]string{"META-INF/container.xml": container, "default/content.opf": opf("Default"), "other/content.opf": opf("Other")}))
		require.NoError(t, err)
		assert.Equal(t, "Default", meta["dc:title"].Value())
	})

	t.Run("Roles", func(t *testing.T) {
		// A creator with several roles, one of which is author, is an author,
		// whatever their order, and one with only another role isn't, unless
		// no creator is an author, such as an anthology's editor
		book := func(creators string) []byte {
			opf := `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title>` + creators + `</metadata></package>`
			return document(t, map[string]string{"META-INF/container.xml": epubContainerXML, "OEBPS/content.opf": opf})
		}
		for _, roles := range [][2]string{{"aut", "ill"}, {"ill", "aut"}} {
			meta, err := epubEntries(t, "/b.epub", book(`<dc:creator id="a">Author Illustrator</dc:creator>
<meta refines="#a" property="role">`+roles[0]+`</meta><meta refines="#a" property="role">`+roles[1]+`</meta>
<dc:creator id="b">Other Author</dc:creator><meta refines="#b" property="role">aut</meta>
<dc:creator id="c">An Editor</dc:creator><meta refines="#c" property="role">edt</meta>`))
			require.NoError(t, err, roles)
			assert.Equal(t, "Author Illustrator, Other Author", meta["dc:creator"].Value(), roles)
		}
		meta, err := epubEntries(t, "/b.epub", book(`<dc:creator id="c">An Editor</dc:creator><meta refines="#c" property="role">edt</meta>`))
		require.NoError(t, err)
		assert.Equal(t, "An Editor", meta["dc:creator"].Value())
	})

	t.Run("Untitled", func(t *testing.T) {
		// A book without a title has the title from its name, and one
		// without metadata has only that
		opf := `<package xmlns="http://www.idpf.org/2007/opf" version="2.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"/><manifest/></package>`
		book := document(t, map[string]string{"META-INF/container.xml": epubContainerXML, "OEBPS/content.opf": opf})
		meta, err := epubEntries(t, "/books/A Book.epub", book)
		require.NoError(t, err)
		require.Len(t, meta, 3)
		assert.Equal(t, "A Book", meta["dc:title"].Value())
		assert.Equal(t, "book", meta["metadata:kind"].Value())
		assert.Equal(t, "Text", meta["dc:type"].Value())
	})

	t.Run("Invalid", func(t *testing.T) {
		// An ISBN whose check digit is wrong, or which is a placeholder,
		// isn't one
		for _, id := range []string{"978194278355", "9781509868075", "NOID", "9781xxxxxxxxx", "4006381333931"} {
			opf := `<package xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title><dc:identifier>` + id + `</dc:identifier></metadata></package>`
			meta, err := epubEntries(t, "/b.epub", document(t, map[string]string{"META-INF/container.xml": epubContainerXML, "OEBPS/content.opf": opf}))
			require.NoError(t, err, id)
			assert.NotContains(t, meta, "dc:identifier", id)
		}

		// An ISBN after another product's EAN, whose check digit is right
		opf := `<package xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title><dc:identifier>4006381333931</dc:identifier><dc:identifier>9780306406157</dc:identifier></metadata></package>`
		meta, err := epubEntries(t, "/b.epub", document(t, map[string]string{"META-INF/container.xml": epubContainerXML, "OEBPS/content.opf": opf}))
		require.NoError(t, err)
		assert.Equal(t, "9780306406157", meta["dc:identifier"].Value())

		// An archive which isn't a book is an error
		_, err = epubEntries(t, "/b.epub", document(t, map[string]string{"mimetype": "application/epub+zip"}))
		assert.ErrorIs(t, err, metadata.ErrBadParameter)
	})
}

func TestEpubMarkdown(t *testing.T) {
	// The text of the documents of the spine, in order, without those which
	// aren't linear, with a rule between them, only when it's requested
	opf := `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title></metadata>
<manifest>
<item id="c1" href="text/one.xhtml" media-type="application/xhtml+xml"/>
<item id="c2" href="text/two%20b.xhtml" media-type="application/xhtml+xml"/>
<item id="notes" href="text/notes.xhtml" media-type="application/xhtml+xml"/>
<item id="css" href="style.css" media-type="text/css"/>
</manifest>
<spine><itemref idref="c2"/><itemref idref="notes" linear="no"/><itemref idref="c1"/><itemref idref="missing"/></spine></package>`
	xhtml := func(body string) string {
		return `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>x</title><style>p{}</style></head><body>` + body + `</body></html>`
	}
	book := document(t, map[string]string{
		"META-INF/container.xml": epubContainerXML,
		"OEBPS/content.opf":      opf,
		"OEBPS/text/one.xhtml":   xhtml(`<h1>Chapter One</h1><p>The <em>end</em>.</p>`),
		"OEBPS/text/two b.xhtml": xhtml(`<h1>Prologue</h1><p>It began.</p>`),
		"OEBPS/text/notes.xhtml": xhtml(`<p>A note</p>`),
	})
	meta, err := epubEntries(t, "/b.epub", book, metadata.WithNamespace("markdown"), metadata.WithRange(":"))
	require.NoError(t, err)
	require.Contains(t, meta, "markdown:body")
	assert.Equal(t, "# Prologue\n\nIt began.\n\n---\n\n# Chapter One\n\nThe *end*.", meta["markdown:body"].Value())

	meta, err = epubEntries(t, "/b.epub", book)
	require.NoError(t, err)
	assert.NotContains(t, meta, "markdown:body")
}

func TestEpubHints(t *testing.T) {
	hint := func(meta map[string]metadata.Metadata) string {
		if m, exists := meta["metadata:hint"]; exists {
			return m.Value()
		}
		return ""
	}

	// A book with a cover in its archive has artwork, and without a spine,
	// no text, and without the cover's image, no artwork
	meta, err := epubEntries(t, "/books/Bad Blood.epub", document(t, map[string]string{"META-INF/container.xml": epubContainerXML, "OEBPS/content.opf": epub2OPF, "OEBPS/Image00003.png": string(coverPNG(t))}))
	require.NoError(t, err)
	assert.Equal(t, "artwork", hint(meta))
	meta, err = epubEntries(t, "/books/Bad Blood.epub", document(t, map[string]string{"META-INF/container.xml": epubContainerXML, "OEBPS/content.opf": epub2OPF}))
	require.NoError(t, err)
	assert.Equal(t, "", hint(meta))

	// A book with a spine has text
	data, err := os.ReadFile(filepath.Join("..", "..", "testfiles", "sample.epub"))
	require.NoError(t, err)
	meta, err = epubEntries(t, "sample.epub", data)
	require.NoError(t, err)
	assert.Contains(t, hint(meta), "markdown")
}
