package office_test

import (
	"bytes"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

const (
	pptxType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

	pptxNS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

	// The slides, in an order which isn't their files'
	pptxPresentationXML  = `<p:presentation ` + pptxNS + `><p:sldIdLst><p:sldId id="256" r:id="rId3"/><p:sldId id="257" r:id="rId2"/><p:sldId id="258" r:id="rId4"/></p:sldIdLst></p:presentation>`
	pptxPresentationRels = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="/ppt/slides/slide3.xml"/>
</Relationships>`

	// The first slide: a title, a list with levels, a text box, a hidden
	// add-in's text, and its number
	pptxSlide2XML = `<p:sld ` + pptxNS + `><p:cSld><p:spTree>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:r><a:t>Plans</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="Content"/><p:cNvSpPr/><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr><p:txBody>
<a:p><a:r><a:t>Hire</a:t></a:r></a:p><a:p><a:pPr lvl="1"/><a:r><a:rPr b="1"/><a:t>Engineers</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="4" name="TextBox"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:txBody><a:p><a:r><a:t>Grow </a:t></a:r><a:r><a:rPr i="1"/><a:t>fast</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="5" name="btfpLayoutConfig" hidden="1"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:txBody><a:p><a:r><a:t>overall_0_123</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="6" name="Number"/><p:cNvSpPr/><p:nvPr><p:ph type="sldNum"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:fld type="slidenum"><a:t>1</a:t></a:fld></a:p></p:txBody></p:sp>
</p:spTree></p:cSld></p:sld>`
	pptxSlide2Rels = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide1.xml"/>
</Relationships>`
	pptxNotesXML = `<p:notes ` + pptxNS + `><p:cSld><p:spTree>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Slide Image"/><p:cNvSpPr/><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="Notes"/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:r><a:t>Mention the budget</a:t></a:r></a:p></p:txBody></p:sp>
</p:spTree></p:cSld></p:notes>`

	// The second slide, which is hidden
	pptxSlide1XML = `<p:sld ` + pptxNS + ` show="0"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:r><a:t>Hidden</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`

	// The third slide, with a table, and without a title
	pptxSlide3XML = `<p:sld ` + pptxNS + `><p:cSld><p:spTree><p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="2" name="Table"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><a:graphic><a:graphicData><a:tbl>
<a:tr><a:tc><a:txBody><a:p><a:r><a:t>Quarter</a:t></a:r></a:p></a:txBody></a:tc><a:tc><a:txBody><a:p><a:r><a:t>Sales</a:t></a:r></a:p></a:txBody></a:tc></a:tr>
<a:tr><a:tc><a:txBody><a:p><a:r><a:t>Q1</a:t></a:r></a:p></a:txBody></a:tc><a:tc><a:txBody><a:p><a:r><a:t>10</a:t></a:r></a:p></a:txBody></a:tc></a:tr>
</a:tbl></a:graphicData></a:graphic></p:graphicFrame></p:spTree></p:cSld></p:sld>`

	// The presentation's Markdown
	pptxMarkdown = "## Slide 1: Plans\n\n- Hire\n  - **Engineers**\n\nGrow _fast_\n\n### Notes\n\nMention the budget\n\n" +
		"## Slide 3\n\n| Quarter | Sales |\n| --- | --- |\n| Q1 | 10 |"
)

func TestPptxMarkdown(t *testing.T) {
	presentation := document(t, map[string]string{
		"ppt/presentation.xml":             pptxPresentationXML,
		"ppt/_rels/presentation.xml.rels":  pptxPresentationRels,
		"ppt/slides/slide1.xml":            pptxSlide1XML,
		"ppt/slides/slide2.xml":            pptxSlide2XML,
		"ppt/slides/_rels/slide2.xml.rels": pptxSlide2Rels,
		"ppt/slides/slide3.xml":            pptxSlide3XML,
		"ppt/notesSlides/notesSlide1.xml":  pptxNotesXML,
	})

	// Each slide which isn't hidden, in order, numbered, with its title, its
	// placeholders' text as lists, its other text, its tables, and its
	// notes, without hidden shapes' text, or its number
	meta, err := getMetadata(t.Context(), bytes.NewReader(presentation), pptxType, metadata.WithNamespace(metadata.MarkdownNamespace))
	require.NoError(t, err)
	require.Len(t, meta, 1)
	assert.Equal(t, "markdown:body", meta[0].NS()+string(metadata.Separator)+meta[0].Key())
	assert.Equal(t, pptxMarkdown, meta[0].Value())

	// A part, from which the next is the next slide's
	meta, err = getMetadata(t.Context(), bytes.NewReader(presentation), pptxType, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(rangeOf(0, 20)))
	require.NoError(t, err)
	values := map[string]string{}
	for _, m := range meta {
		values[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	assert.Equal(t, pptxMarkdown[:20], values["markdown:body"])
	assert.Equal(t, "20", values["markdown:next"])

	// Slides are numbered from the presentation's first number, a slide
	// whose show is false is hidden, and a level is at most the deepest
	presentation = document(t, map[string]string{
		"ppt/presentation.xml":            strings.Replace(pptxPresentationXML, `<p:presentation `, `<p:presentation firstSlideNum="5" `, 1),
		"ppt/_rels/presentation.xml.rels": pptxPresentationRels,
		"ppt/slides/slide1.xml":           strings.Replace(pptxSlide1XML, `show="0"`, `show="false"`, 1),
		"ppt/slides/slide2.xml":           strings.Replace(pptxSlide2XML, `<a:pPr lvl="1"/>`, `<a:pPr lvl="2147483647"/>`, 1),
		"ppt/slides/slide3.xml":           pptxSlide3XML,
	})
	meta, err = getMetadata(t.Context(), bytes.NewReader(presentation), pptxType, metadata.WithNamespace(metadata.MarkdownNamespace))
	require.NoError(t, err)
	require.Len(t, meta, 1)
	assert.Equal(t, "## Slide 5: Plans\n\n- Hire\n"+strings.Repeat("  ", 8)+"- **Engineers**\n\nGrow _fast_\n\n## Slide 7\n\n| Quarter | Sales |\n| --- | --- |\n| Q1 | 10 |", meta[0].Value())
}

func TestPptxShapes(t *testing.T) {
	// slide returns the Markdown of a presentation with one slide, whose
	// shapes are the XML
	slide := func(t *testing.T, shapes string) string {
		t.Helper()
		presentation := document(t, map[string]string{
			"ppt/presentation.xml":            `<p:presentation ` + pptxNS + `><p:sldIdLst><p:sldId id="256" r:id="rId2"/></p:sldIdLst></p:presentation>`,
			"ppt/_rels/presentation.xml.rels": pptxPresentationRels,
			"ppt/slides/slide1.xml":           `<p:sld ` + pptxNS + ` xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><p:cSld><p:spTree>` + shapes + `</p:spTree></p:cSld></p:sld>`,
		})
		meta, err := getMetadata(t.Context(), bytes.NewReader(presentation), pptxType, metadata.WithNamespace(metadata.MarkdownNamespace))
		require.NoError(t, err)
		require.Len(t, meta, 1)
		return meta[0].Value()
	}
	text := func(text string, hidden bool) string {
		h := ""
		if hidden {
			h = ` hidden="true"`
		}
		return `<p:sp><p:nvSpPr><p:cNvPr id="9" name="Text"` + h + `/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:txBody><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp>`
	}
	pic := func(hidden bool) string {
		h := ""
		if hidden {
			h = ` hidden="1"`
		}
		return `<p:pic><p:nvPicPr><p:cNvPr id="8" name="Picture"` + h + `/><p:cNvPicPr/><p:nvPr/></p:nvPicPr></p:pic>`
	}
	group := func(hidden bool, shapes string) string {
		h := ""
		if hidden {
			h = ` hidden="1"`
		}
		return `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="7" name="Group"` + h + `/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` + shapes + `</p:grpSp>`
	}

	// A hidden picture in a group doesn't hide its siblings, and a visible
	// one doesn't show its hidden group's
	assert.Equal(t, "## Slide 1\n\nShown", slide(t, group(false, pic(true)+text("Shown", false))))
	assert.Equal(t, "## Slide 1", slide(t, group(true, pic(false)+text("Hidden", false))))

	// Alternate content is read once, from its choice
	assert.Equal(t, "## Slide 1\n\nOnce", slide(t, `<mc:AlternateContent><mc:Choice Requires="p14">`+text("Once", false)+`</mc:Choice><mc:Fallback>`+text("Once", false)+`</mc:Fallback></mc:AlternateContent>`))

	// Bold and italic are XML booleans
	assert.Equal(t, "## Slide 1\n\n**bold** _italic_", slide(t, `<p:sp><p:nvSpPr><p:cNvPr id="9" name="Text"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:txBody><a:p><a:r><a:rPr b="true"/><a:t>bold</a:t></a:r><a:r><a:t xml:space="preserve"> </a:t></a:r><a:r><a:rPr i="true"/><a:t>italic</a:t></a:r></a:p></p:txBody></p:sp>`))
}
