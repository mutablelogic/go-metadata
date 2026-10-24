package office_test

import (
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

const (
	// A Word document's styles: a heading, a custom style based on another
	// heading, a title, and a style whose name is in German, which is a
	// heading by its outline level
	docxStylesXML = `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:style w:type="paragraph" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/></w:style>
<w:style w:type="paragraph" w:styleId="Section"><w:name w:val="Section"/><w:basedOn w:val="Heading2"/></w:style>
<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/></w:style>
<w:style w:type="paragraph" w:styleId="berschrift3"><w:name w:val="Überschrift 3"/><w:pPr><w:outlineLvl w:val="2"/></w:pPr></w:style>
</w:styles>`

	// A Word document's numbering: a bulleted list, and a numbered one
	docxNumberingXML = `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/></w:lvl><w:lvl w:ilvl="1"><w:numFmt w:val="bullet"/></w:lvl></w:abstractNum>
<w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl></w:abstractNum>
<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>
</w:numbering>`

	// A Word document's content
	docxDocumentXML = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><w:body>
<w:p><w:pPr><w:pStyle w:val="Title"/></w:pPr><w:r><w:t>Annual Report</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Summary</w:t></w:r></w:p>
<w:p><w:r><w:t xml:space="preserve">The year was </w:t></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>good</w:t></w:r><w:r><w:t xml:space="preserve">, and </w:t></w:r><w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve">very </w:t></w:r><w:r><w:rPr><w:b w:val="0"/></w:rPr><w:t>busy</w:t></w:r><w:del><w:r><w:delText>deleted</w:delText></w:r></w:del><w:ins><w:r><w:t>.</w:t></w:r></w:ins></w:p>
<w:p><w:pPr><w:pStyle w:val="Section"/></w:pPr><w:r><w:t>Plans</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Hire</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Engineers</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr><w:r><w:t>Grow</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="berschrift3"/></w:pPr><w:r><w:t>Zahlen</w:t></w:r></w:p>
<w:tbl><w:tr><w:tc><w:p><w:r><w:t>Quarter</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Sales</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:p><w:r><w:t>Q1</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>10 | 20</w:t></w:r></w:p><w:p><w:r><w:t>units</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
<w:p><w:r><mc:AlternateContent><mc:Choice><w:drawing><w:txbxContent><w:p><w:r><w:t>In a box</w:t></w:r></w:p></w:txbxContent></w:drawing></mc:Choice><mc:Fallback><w:pict><w:txbxContent><w:p><w:r><w:t>In a box</w:t></w:r></w:p></w:txbxContent></w:pict></mc:Fallback></mc:AlternateContent></w:r></w:p>
<w:p><w:r><w:t>1. Not a list, *not* bold</w:t></w:r></w:p>
<w:p></w:p>
</w:body></w:document>`

	// The document's Markdown
	docxMarkdown = "# Annual Report\n\n" +
		"# Summary\n\n" +
		"The year was **good**, and _very_ busy.\n\n" +
		"## Plans\n\n" +
		"- Hire\n  - Engineers\n1. Grow\n\n" +
		"### Zahlen\n\n" +
		"| Quarter | Sales |\n| --- | --- |\n| Q1 | 10 \\| 20 units |\n\n" +
		"In a box\n\n" +
		"1\\. Not a list, \\*not\\* bold"
)

// docxBody returns the Markdown of the Word document, from the offset, up to
// the limit, and the offset of the next part
func docxBody(t *testing.T, files map[string]string, offset, limit uint64) (string, string) {
	t.Helper()
	meta, err := entries(t, document(t, files), metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(rangeOf(offset, limit)))
	require.NoError(t, err)
	body, next := "", ""
	if m, exists := meta["markdown:body"]; exists {
		body = m.Value()
	}
	if m, exists := meta["markdown:next"]; exists {
		next = m.Value()
	}
	return body, next
}

func TestDocxMarkdown(t *testing.T) {
	files := map[string]string{"word/document.xml": docxDocumentXML, "word/styles.xml": docxStylesXML, "word/numbering.xml": docxNumberingXML, "docProps/core.xml": coreXML}

	t.Run("Markdown", func(t *testing.T) {
		// Headings by their styles, lists, a table, bold and italic, without
		// deleted text or fallbacks, and with Markdown's characters escaped
		body, next := docxBody(t, files, 0, 0)
		assert.Equal(t, docxMarkdown, body)
		assert.Empty(t, next)
	})

	t.Run("Parts", func(t *testing.T) {
		// The Markdown a part at a time is all of it
		var all strings.Builder
		offset := "0"
		for parts := 0; offset != ""; parts++ {
			require.Less(t, parts, 50)
			var n uint64
			for _, r := range offset {
				n = n*10 + uint64(r-'0')
			}
			body, next := docxBody(t, files, n, 16)
			all.WriteString(body)
			offset = next
		}
		assert.Equal(t, docxMarkdown, all.String())
	})

	t.Run("NotRequested", func(t *testing.T) {
		// Without the markdown namespace, there's none
		meta, err := entries(t, document(t, files))
		require.NoError(t, err)
		assert.NotContains(t, meta, "markdown:body")
	})

	t.Run("TextBoxInRun", func(t *testing.T) {
		// Text before, in, and after a text box in a run is in order, and
		// the box's text is escaped once
		doc := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Before</w:t><w:drawing><w:txbxContent><w:p><w:r><w:t>a_b</w:t></w:r></w:p></w:txbxContent></w:drawing><w:t>after</w:t></w:r><w:r><w:t xml:space="preserve"> end</w:t></w:r></w:p></w:body></w:document>`
		body, _ := docxBody(t, map[string]string{"word/document.xml": doc}, 0, 0)
		assert.Equal(t, "**Before** a\\_b **after** end", body)
	})

	t.Run("StyleList", func(t *testing.T) {
		// A paragraph whose style is in a list, such as List Bullet, is in
		// it, at its own level, when it has one, or its style's
		styles := strings.Replace(docxStylesXML, `</w:styles>`, `<w:style w:type="paragraph" w:styleId="ListBullet"><w:name w:val="List Bullet"/><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr></w:style>
<w:style w:type="paragraph" w:styleId="ListNumber2"><w:name w:val="List Number 2"/><w:basedOn w:val="ListNumber"/></w:style>
<w:style w:type="paragraph" w:styleId="ListNumber"><w:name w:val="List Number"/><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr></w:style></w:styles>`, 1)
		doc := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:pPr><w:pStyle w:val="ListBullet"/></w:pPr><w:r><w:t>One</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="ListBullet"/><w:numPr><w:ilvl w:val="1"/></w:numPr></w:pPr><w:r><w:t>Two</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="ListNumber2"/></w:pPr><w:r><w:t>Three</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="ListBullet"/><w:numPr><w:numId w:val="0"/></w:numPr></w:pPr><w:r><w:t>Not listed</w:t></w:r></w:p>
</w:body></w:document>`
		body, _ := docxBody(t, map[string]string{"word/document.xml": doc, "word/styles.xml": styles, "word/numbering.xml": docxNumberingXML}, 0, 0)
		assert.Equal(t, "- One\n  - Two\n1. Three\n\nNot listed", body)
	})

	t.Run("DeepLevel", func(t *testing.T) {
		// A list's level is at most the deepest Word has
		doc := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:pPr><w:numPr><w:ilvl w:val="9223372036854775807"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Deep</w:t></w:r></w:p></w:body></w:document>`
		body, _ := docxBody(t, map[string]string{"word/document.xml": doc, "word/numbering.xml": docxNumberingXML}, 0, 0)
		assert.Equal(t, strings.Repeat("  ", 8)+"- Deep", body)
	})

	t.Run("TrackedFormatting", func(t *testing.T) {
		// The formatting before a tracked change isn't the paragraph's or
		// the run's
		doc := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:pPr><w:pStyle w:val="Normal"/><w:pPrChange w:id="1"><w:pPr><w:pStyle w:val="Heading1"/></w:pPr></w:pPrChange></w:pPr><w:r><w:rPr><w:rPrChange w:id="2"><w:rPr><w:b/></w:rPr></w:rPrChange></w:rPr><w:t>Plain</w:t></w:r></w:p></w:body></w:document>`
		body, _ := docxBody(t, map[string]string{"word/document.xml": doc, "word/styles.xml": docxStylesXML}, 0, 0)
		assert.Equal(t, "Plain", body)
	})

	t.Run("GridSpan", func(t *testing.T) {
		// A cell which spans columns is followed by the columns it covers
		doc := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:tbl>
<w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>C</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:tcPr><w:gridSpan w:val="2"/></w:tcPr><w:p><w:r><w:t>AB</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>c</w:t></w:r></w:p></w:tc></w:tr>
</w:tbl></w:body></w:document>`
		body, _ := docxBody(t, map[string]string{"word/document.xml": doc}, 0, 0)
		assert.Equal(t, "| A | B | C |\n| --- | --- | --- |\n| AB |  | c |", body)
	})

	t.Run("NoStyles", func(t *testing.T) {
		// Without styles, a heading is known by its style's ID
		body, _ := docxBody(t, map[string]string{"word/document.xml": strings.Replace(docxDocumentXML, `w:val="Title"`, `w:val="Heading2"`, 1)}, 0, 0)
		assert.True(t, strings.HasPrefix(body, "## Annual Report\n\n# Summary"), body)
	})
}
