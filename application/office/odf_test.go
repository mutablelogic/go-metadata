package office_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

const (
	odtType = "application/vnd.oasis.opendocument.text"

	// The metadata of an OpenDocument text document, as LibreOffice writes
	// it, whose creator is who last changed it
	odfMetaXML = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-meta xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/" office:version="1.3">
<office:meta>
<meta:initial-creator>Jane Doe</meta:initial-creator>
<meta:creation-date>2026-04-01T09:30:00.123456789</meta:creation-date>
<dc:title>Annual Report</dc:title>
<dc:subject>Finance</dc:subject>
<dc:description>The year in numbers</dc:description>
<dc:creator>John Doe</dc:creator>
<dc:date>2026-04-02T10:26:00</dc:date>
<dc:language>en-GB</dc:language>
<meta:generator>LibreOffice/24.2</meta:generator>
</office:meta>
</office:document-meta>`
)

// odfEntries returns the metadata of the OpenDocument document, by key
func odfEntries(t *testing.T, name string, data []byte, opts ...metadata.Option) (map[string]metadata.Metadata, error) {
	t.Helper()
	meta, err := getMetadata(t.Context(), named{bytes.NewReader(data), name}, odtType, opts...)
	result := make(map[string]metadata.Metadata)
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m
	}
	return result, err
}

func TestODFMetadata(t *testing.T) {
	files := map[string]string{"mimetype": odtType, "meta.xml": odfMetaXML, "content.xml": "<office:document-content/>", "Thumbnails/thumbnail.png": string(coverPNG(t))}

	t.Run("Metadata", func(t *testing.T) {
		// Its metadata, whose author is its initial creator, and its date
		// when it was created
		meta, err := odfEntries(t, "/docs/report.odt", document(t, files))
		require.NoError(t, err)
		assert.Equal(t, "Annual Report", meta["dc:title"].Value())
		assert.Equal(t, "Jane Doe", meta["dc:creator"].Value())
		assert.Equal(t, "Finance", meta["dc:subject"].Value())
		assert.Equal(t, "The year in numbers", meta["dc:description"].Value())
		assert.Equal(t, "en-GB", meta["dc:language"].Value())
		assert.Equal(t, time.Date(2026, 4, 1, 9, 30, 0, 123456789, time.UTC), meta["dc:date"].Any())
		assert.NotContains(t, meta, "artwork:thumbnail")
	})

	t.Run("Thumbnail", func(t *testing.T) {
		// Its thumbnail, when artwork is requested
		meta, err := odfEntries(t, "/docs/report.odt", document(t, files), metadata.WithNamespace("artwork"))
		require.NoError(t, err)
		require.Len(t, meta, 1)
		assert.Equal(t, 40, meta["artwork:thumbnail"].Image().Bounds().Dx())
	})

	t.Run("Untitled", func(t *testing.T) {
		// A document without metadata has the title from its name, and its
		// type and kind
		meta, err := odfEntries(t, "/docs/Minutes.odt", document(t, map[string]string{"mimetype": odtType, "content.xml": "<office:document-content/>"}))
		require.NoError(t, err)
		require.Len(t, meta, 3)
		assert.Equal(t, "document", meta["metadata:kind"].Value())
		assert.Equal(t, "Minutes", meta["dc:title"].Value())
	})
}

const (
	odfContentNS = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:presentation="urn:oasis:names:tc:opendocument:xmlns:presentation:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/"`

	// A text document's content, as LibreOffice writes it
	odtContentXML = `<office:document-content ` + odfContentNS + `><office:body><office:text>
<text:tracked-changes><text:changed-region><text:deletion><text:p>Deleted</text:p></text:deletion></text:changed-region></text:tracked-changes>
<text:h text:outline-level="1">Summary</text:h>
<text:p>The year was<text:s text:c="2"/>good<office:annotation><dc:creator>Jane</dc:creator><text:p>A comment</text:p></office:annotation>.<text:note><text:note-citation>1</text:note-citation><text:note-body><text:p>A footnote</text:p></text:note-body></text:note></text:p>
<text:h text:outline-level="2">Plans</text:h>
<text:list><text:list-item><text:p>Hire</text:p><text:list><text:list-item><text:p>Engineers</text:p></text:list-item></text:list></text:list-item><text:list-item><text:p>Grow</text:p></text:list-item></text:list>
<table:table table:name="Table1"><table:table-row><table:table-cell><text:p>Quarter</text:p></table:table-cell><table:table-cell><text:p>Sales</text:p></table:table-cell></table:table-row><table:table-row><table:table-cell><text:p>Q1</text:p></table:table-cell><table:table-cell><text:p>10</text:p></table:table-cell></table:table-row></table:table>
<text:p>1. Not a list</text:p>
<text:p>Boxed<draw:frame><draw:text-box><text:p>a_b</text:p></draw:text-box></draw:frame>text</text:p>
<text:p/>
</office:text></office:body></office:document-content>`

	odtMarkdown = "# Summary\n\nThe year was good.\n\n## Plans\n\n- Hire\n  - Engineers\n- Grow\n\n| Quarter | Sales |\n| --- | --- |\n| Q1 | 10 |\n\n1\\. Not a list\n\nBoxed a\\_b text"

	// A spreadsheet's content, with repeated cells, and an empty row
	// repeated for the rest of the sheet, as LibreOffice writes it
	odsContentXML = `<office:document-content ` + odfContentNS + `><office:body><office:spreadsheet>
<table:table table:name="Sales"><table:table-column table:number-columns-repeated="1024"/>
<table:table-row><table:table-cell><text:p>Quarter</text:p></table:table-cell><table:table-cell table:number-columns-repeated="2"/><table:table-cell><text:p>Sales</text:p></table:table-cell><table:table-cell table:number-columns-repeated="1020"/></table:table-row>
<table:table-row><table:table-cell><text:p>Q1</text:p></table:table-cell><table:table-cell table:number-columns-repeated="2"><text:p>-</text:p></table:table-cell><table:table-cell><text:p>10</text:p></table:table-cell></table:table-row>
<table:table-row table:number-rows-repeated="2"><table:table-cell><text:p>Q2</text:p></table:table-cell></table:table-row>
<table:table-row><table:table-cell office:value-type="string" office:string-value="Q3"/><table:table-cell office:value-type="float" office:value="42" table:number-columns-repeated="3"/></table:table-row>
<table:table-row table:number-rows-repeated="1048574"><table:table-cell table:number-columns-repeated="1024"/></table:table-row>
</table:table>
<table:table table:name="Empty"><table:table-row><table:table-cell table:number-columns-repeated="1024"/></table:table-row></table:table>
</office:spreadsheet></office:body></office:document-content>`

	odsMarkdown = "## Sheet: Sales\n\n| Quarter |  |  | Sales |\n| --- | --- | --- | --- |\n| Q1 | - | - | 10 |\n| Q2 |  |  |  |\n| Q2 |  |  |  |\n| Q3 | 42 | 42 | 42 |\n\n## Sheet: Empty"

	// A presentation's content, with a title, an outline, a text box, a page
	// number, and notes
	odpContentXML = `<office:document-content ` + odfContentNS + `><office:body><office:presentation>
<draw:page draw:name="page1">
<draw:frame presentation:class="title"><draw:text-box><text:p>Plans</text:p></draw:text-box></draw:frame>
<draw:frame presentation:class="outline"><draw:text-box><text:list><text:list-item><text:p>Hire</text:p><text:list><text:list-item><text:p>Engineers</text:p></text:list-item></text:list></text:list-item></text:list></draw:text-box></draw:frame>
<draw:frame><draw:text-box><text:p>Grow fast</text:p></draw:text-box></draw:frame>
<draw:frame presentation:class="page-number"><draw:text-box><text:p>1</text:p></draw:text-box></draw:frame>
<presentation:notes><draw:frame presentation:class="notes"><draw:text-box><text:p>Mention the budget</text:p></draw:text-box></draw:frame><draw:frame><table:table><table:table-row><table:table-cell><text:p>Cost</text:p></table:table-cell></table:table-row></table:table></draw:frame></presentation:notes>
</draw:page>
<draw:page draw:name="page2"><draw:frame><draw:text-box><text:p>The end</text:p></draw:text-box></draw:frame></draw:page>
</office:presentation></office:body></office:document-content>`

	odpMarkdown = "## Slide 1: Plans\n\n- Hire\n  - Engineers\n\nGrow fast\n\n### Notes\n\nMention the budget\n\n| Cost |\n| --- |\n\n## Slide 2\n\nThe end"
)

func TestODFExpansion(t *testing.T) {
	// A large cell repeated across columns and rows expands to at most the
	// most text of a table
	cell := strings.Repeat("x", 32<<10)
	content := `<office:document-content ` + odfContentNS + `><office:body><office:spreadsheet><table:table table:name="Big">
<table:table-row table:number-rows-repeated="1000"><table:table-cell table:number-columns-repeated="100"><text:p>` + cell + `</text:p></table:table-cell></table:table-row>
</table:table></office:spreadsheet></office:body></office:document-content>`
	data := document(t, map[string]string{"mimetype": "application/vnd.oasis.opendocument.spreadsheet", "content.xml": content})
	meta, err := getMetadata(t.Context(), bytes.NewReader(data), "application/vnd.oasis.opendocument.spreadsheet", metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(rangeOf(0, 1024)))
	require.NoError(t, err)
	values := map[string]string{}
	for _, m := range meta {
		values[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	assert.Len(t, values["markdown:body"], 1024)
	assert.NotEmpty(t, values["markdown:next"])
}

func TestODFMarkdown(t *testing.T) {
	for _, test := range []struct {
		name, contentType, content, want string
	}{
		{"Text", "application/vnd.oasis.opendocument.text", odtContentXML, odtMarkdown},
		{"Spreadsheet", "application/vnd.oasis.opendocument.spreadsheet", odsContentXML, odsMarkdown},
		{"Presentation", "application/vnd.oasis.opendocument.presentation", odpContentXML, odpMarkdown},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := document(t, map[string]string{"mimetype": test.contentType, "content.xml": test.content})
			meta, err := getMetadata(t.Context(), bytes.NewReader(data), test.contentType, metadata.WithNamespace(metadata.MarkdownNamespace))
			require.NoError(t, err)
			require.Len(t, meta, 1)
			assert.Equal(t, test.want, meta[0].Value())
		})
	}
}

func TestODFTemplates(t *testing.T) {
	// A template is detected by its extension, and read as a document is
	data, err := os.ReadFile(filepath.Join("..", "..", "testfiles", "sample.odt"))
	require.NoError(t, err)
	for _, test := range []struct{ name, contentType string }{
		{"letter.ott", "application/vnd.oasis.opendocument.text-template"},
		{"budget.ots", "application/vnd.oasis.opendocument.spreadsheet-template"},
		{"slides.otp", "application/vnd.oasis.opendocument.presentation-template"},
	} {
		contentType, meta, err := metadata.Extract(t.Context(), named{bytes.NewReader(data), test.name})
		require.NoError(t, err)
		assert.Equal(t, test.contentType, contentType)
		values := map[string]string{}
		for _, m := range meta {
			values[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
		}
		assert.Equal(t, "Sample OpenDocument", values["dc:title"], test.name)
	}
}
