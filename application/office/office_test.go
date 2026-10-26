package office_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	office "github.com/mutablelogic/go-metadata/application/office"
)

const (
	wordType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	// The properties of a Word document, as Word writes them
	coreXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<dc:title>Scoping Questionnaire</dc:title>
<dc:subject>Web application testing</dc:subject>
<dc:creator>Jane Doe</dc:creator>
<cp:keywords>scoping</cp:keywords>
<dc:description></dc:description>
<cp:lastModifiedBy>John Doe</cp:lastModifiedBy>
<dcterms:created xsi:type="dcterms:W3CDTF">2026-04-01T09:30:00Z</dcterms:created>
<dcterms:modified xsi:type="dcterms:W3CDTF">2026-04-02T10:26:00Z</dcterms:modified>
</cp:coreProperties>`
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

// entries returns the metadata of the document, by key
func entries(t *testing.T, data []byte, opts ...metadata.Option) (map[string]metadata.Metadata, error) {
	t.Helper()
	return namedEntries(t, nil, data, opts...)
}

// namedEntries returns the metadata of the document with the name, by key
func namedEntries(t *testing.T, name *string, data []byte, opts ...metadata.Option) (map[string]metadata.Metadata, error) {
	t.Helper()
	var r io.Reader = bytes.NewReader(data)
	if name != nil {
		r = named{r, *name}
	}
	meta, err := getMetadata(context.Background(), r, wordType, opts...)
	result := make(map[string]metadata.Metadata)
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m
	}
	return result, err
}

func TestOfficeMetadata(t *testing.T) {
	t.Run("Properties", func(t *testing.T) {
		// The properties which are set, and the type and kind
		meta, err := entries(t, document(t, map[string]string{"docProps/core.xml": coreXML, "word/document.xml": "<w:document/>"}))
		require.NoError(t, err)
		require.Len(t, meta, 7)
		assert.Equal(t, "Text", meta["dc:type"].Value())
		assert.Equal(t, "markdown", meta["metadata:hint"].Value())
		assert.Equal(t, "document", meta["metadata:kind"].Value())
		assert.Equal(t, "Scoping Questionnaire", meta["dc:title"].Value())
		assert.Equal(t, "Web application testing", meta["dc:subject"].Value())
		assert.Equal(t, "Jane Doe", meta["dc:creator"].Value())
		assert.Equal(t, time.Date(2026, 4, 1, 9, 30, 0, 0, time.UTC), meta["dc:date"].Any())
		assert.Equal(t, "2026-04-01T09:30:00Z", meta["dc:date"].Value())
	})

	t.Run("NoProperties", func(t *testing.T) {
		// A document without properties or a name has only its type and kind
		meta, err := entries(t, document(t, map[string]string{"word/document.xml": "<w:document/>"}))
		require.NoError(t, err)
		require.Len(t, meta, 3)
		assert.Equal(t, "document", meta["metadata:kind"].Value())
	})

	t.Run("NameAsTitle", func(t *testing.T) {
		// When the title isn't set, it's the document's file name without
		// its extension, with underscores as spaces
		name := "/doc/Polito_Scoping__Questionnaire - WAPT_V2.docx"
		meta, err := namedEntries(t, &name, document(t, map[string]string{"word/document.xml": "<w:document/>"}))
		require.NoError(t, err)
		require.Contains(t, meta, "dc:title")
		assert.Equal(t, "Polito Scoping Questionnaire - WAPT V2", meta["dc:title"].Value())

		// An empty title is not set
		untitled := strings.Replace(coreXML, "<dc:title>Scoping Questionnaire</dc:title>", "<dc:title> </dc:title>", 1)
		meta, err = namedEntries(t, &name, document(t, map[string]string{"docProps/core.xml": untitled}))
		require.NoError(t, err)
		assert.Equal(t, "Polito Scoping Questionnaire - WAPT V2", meta["dc:title"].Value())
		assert.Equal(t, "Jane Doe", meta["dc:creator"].Value())

		// A title which is the name of the file the document was made from
		// is not set
		leftover := strings.Replace(coreXML, "<dc:title>Scoping Questionnaire</dc:title>", "<dc:title>Microsoft Word - Document1.docx</dc:title>", 1)
		meta, err = namedEntries(t, &name, document(t, map[string]string{"docProps/core.xml": leftover}))
		require.NoError(t, err)
		assert.Equal(t, "Polito Scoping Questionnaire - WAPT V2", meta["dc:title"].Value())

		// A title which is set is kept
		meta, err = namedEntries(t, &name, document(t, map[string]string{"docProps/core.xml": coreXML}))
		require.NoError(t, err)
		assert.Equal(t, "Scoping Questionnaire", meta["dc:title"].Value())
	})

	t.Run("Namespace", func(t *testing.T) {
		// The properties are in the dc namespace
		meta, err := entries(t, document(t, map[string]string{"docProps/core.xml": coreXML}), metadata.WithNamespace("exif"))
		require.NoError(t, err)
		assert.Empty(t, meta)
	})

	t.Run("Large", func(t *testing.T) {
		// A document which is larger than is read into memory is read in
		// place, so has its properties
		defer office.SetMaxDocumentSize(100)()
		name := "/doc/Big_Report.docx"
		meta, err := namedEntries(t, &name, document(t, map[string]string{"docProps/core.xml": coreXML, "word/document.xml": strings.Repeat("x", 1000)}))
		require.NoError(t, err)
		assert.Equal(t, "Scoping Questionnaire", meta["dc:title"].Value())
	})

	t.Run("LargeProperties", func(t *testing.T) {
		// Properties which are too large aren't decompressed in full
		large := strings.Replace(coreXML, "<dc:description></dc:description>", "<dc:description>"+strings.Repeat("x", 2<<20)+"</dc:description>", 1)
		_, err := entries(t, document(t, map[string]string{"docProps/core.xml": large}))
		assert.ErrorIs(t, err, metadata.ErrBadParameter)
	})

	t.Run("NotDocument", func(t *testing.T) {
		// Content which isn't a zip archive, or properties which aren't XML
		_, err := entries(t, []byte("not a document"))
		assert.ErrorIs(t, err, metadata.ErrBadParameter)
		_, err = entries(t, document(t, map[string]string{"docProps/core.xml": "<cp:coreProperties"}))
		assert.ErrorIs(t, err, metadata.ErrBadParameter)
	})
}

func TestOfficeThumbnail(t *testing.T) {
	thumbnail := string(coverPNG(t))
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail" Target="docProps/preview.png"/>
</Relationships>`

	// thumbnail returns the document's artwork, by key
	artwork := func(t *testing.T, files map[string]string) map[string]metadata.Metadata {
		t.Helper()
		meta, err := entries(t, document(t, files), metadata.WithNamespace("artwork"))
		require.NoError(t, err)
		return meta
	}

	t.Run("Relationship", func(t *testing.T) {
		// The thumbnail which the package's relationships have, only when
		// artwork is requested
		files := map[string]string{"_rels/.rels": rels, "docProps/preview.png": thumbnail, "docProps/core.xml": coreXML}
		meta := artwork(t, files)
		require.Len(t, meta, 1)
		assert.Equal(t, 40, meta["artwork:thumbnail"].Image().Bounds().Dx())
		meta, err := entries(t, document(t, files))
		require.NoError(t, err)
		assert.NotContains(t, meta, "artwork:thumbnail")
	})

	t.Run("RelativeTarget", func(t *testing.T) {
		// A target relative to the package, with dot segments, or absolute,
		// is the thumbnail's path within it
		for _, target := range []string{"./docProps/preview.png", "/docProps/preview.png", "docProps/../docProps/preview.png"} {
			files := map[string]string{"_rels/.rels": strings.Replace(rels, `Target="docProps/preview.png"`, `Target="`+target+`"`, 1), "docProps/preview.png": thumbnail}
			assert.Contains(t, artwork(t, files), "artwork:thumbnail", target)
		}

		// An external thumbnail isn't read
		external := strings.Replace(rels, `Target="docProps/preview.png"`, `Target="https://example.com/preview.png" TargetMode="External"`, 1)
		assert.Empty(t, artwork(t, map[string]string{"_rels/.rels": external}))
	})

	t.Run("UsualPath", func(t *testing.T) {
		// Without relationships, the thumbnail at its usual path
		meta := artwork(t, map[string]string{"docProps/thumbnail.jpeg": thumbnail})
		assert.Contains(t, meta, "artwork:thumbnail")
	})

	t.Run("None", func(t *testing.T) {
		// A thumbnail which isn't a picture which can be decoded, such as a
		// Windows metafile, or none, isn't artwork
		assert.Empty(t, artwork(t, map[string]string{"docProps/thumbnail.emf": "\x01\x00\x00\x00 EMF", "_rels/.rels": strings.Replace(rels, "preview.png", "thumbnail.emf", 1)}))
		assert.Empty(t, artwork(t, map[string]string{"word/document.xml": "<w:document/>"}))
	})
}
