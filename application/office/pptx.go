package office

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	zipdoc "github.com/mutablelogic/go-metadata/internal/zipdoc"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// pptxPresentation is a PowerPoint presentation's slides, in order, by the
// identifiers of their relationships, and the number of its first slide
type pptxPresentation struct {
	FirstSlideNum string `xml:"firstSlideNum,attr"`
	Slides        []struct {
		RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	} `xml:"sldIdLst>sldId"`
}

// partRelationships are the relationships of a part of an Office Open XML
// document, such as a presentation's to its slides, or a slide's to its
// notes
type partRelationships struct {
	Relationships []struct {
		ID     string `xml:"Id,attr"`
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

// pptxSlide is the text of a slide, or its notes: its title, and its other
// blocks of Markdown
type pptxSlide struct {
	hidden bool
	title  string
	blocks []string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// pptxPresentationPath is the path of a presentation's slides
	pptxPresentationPath = "ppt/presentation.xml"

	// drawingNS is the namespace of the text of slides, and presentationNS
	// of their shapes
	drawingNS      = "http://schemas.openxmlformats.org/drawingml/2006/main"
	presentationNS = "http://schemas.openxmlformats.org/presentationml/2006/main"

	// notesRelationship is the type of the relationship of a slide to its
	// notes
	notesRelationship = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"

	// maxSlideSize is the largest slide, or notes, which is read
	maxSlideSize = 16 << 20
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// pptxMarkdown writes the content of a PowerPoint presentation to the writer
// as Markdown: each slide which isn't hidden, in order, with a heading of
// its number and title, its text, as lists when it's in placeholders, its
// tables, and its notes. Returns the writer's error, once it has its part.
func pptxMarkdown(archive *zip.Reader, w io.StringWriter) error {
	var presentation pptxPresentation
	if err := zipdoc.ReadXML(archive, pptxPresentationPath, &presentation); err != nil {
		return err
	}
	rels := readRelationships(archive, pptxPresentationPath)

	// Slides are numbered from the presentation's first slide's number,
	// which is one unless it's set
	first, number := true, 0
	if n, err := strconv.Atoi(presentation.FirstSlideNum); err == nil && n > 0 {
		number = n - 1
	}
	for _, slide := range presentation.Slides {
		target, exists := rels.target(slide.RelID)
		if !exists {
			continue
		}
		number++
		slidePath := resolvePart(pptxPresentationPath, target)
		content, err := readSlide(archive, slidePath, false)
		if err != nil {
			return err
		} else if content.hidden {
			continue
		}

		// The slide's heading, text and notes
		var b strings.Builder
		if !first {
			b.WriteString("\n\n")
		}
		first = false
		b.WriteString(fmt.Sprintf("## Slide %d", number))
		if content.title != "" {
			b.WriteString(": " + content.title)
		}
		for _, block := range content.blocks {
			b.WriteString("\n\n" + block)
		}
		if notesPath, exists := readRelationships(archive, slidePath).targetOf(notesRelationship); exists {
			if notes, err := readSlide(archive, resolvePart(slidePath, notesPath), true); err == nil && len(notes.blocks) > 0 {
				b.WriteString("\n\n### Notes")
				for _, block := range notes.blocks {
					b.WriteString("\n\n" + block)
				}
			}
		}
		if _, err := w.WriteString(b.String()); err != nil {
			return err
		}
	}
	return nil
}

// readSlide returns the text of a slide, or its notes: its title, which is
// its title placeholder's, its placeholders' paragraphs as lists, or as
// paragraphs in its notes, its other shapes' paragraphs, and its tables.
// The notes' slide image, and a slide's number, date, header and footer,
// aren't its text.
func readSlide(archive *zip.Reader, name string, notes bool) (*pptxSlide, error) {
	f, err := archive.Open(name)
	if err != nil {
		return nil, metadata.ErrBadParameter.Withf("%s: %v", name, err)
	}
	defer f.Close()
	d := xml.NewDecoder(io.LimitReader(f, maxSlideSize))

	slide := new(pptxSlide)
	var placeholder string // the type of the shape's placeholder, or "text" when it isn't one
	var paragraph *docxParagraph
	var run *docxRun
	var table *markdownTable
	var list []string // the shape's paragraphs, as list items
	var shapes []bool // whether each shape being read, the last within the others, is hidden
	inText := false
	hidden := func() bool {
		for _, h := range shapes {
			if h {
				return true
			}
		}
		return false
	}
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return slide, nil
		} else if err != nil {
			return nil, metadata.ErrBadParameter.Withf("%s: %v", name, err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			// The fallbacks of alternate content repeat its choices
			if t.Name.Space == mcNS && t.Name.Local == "Fallback" {
				if err := d.Skip(); err != nil {
					return nil, metadata.ErrBadParameter.Withf("%s: %v", name, err)
				}
				continue
			}
			switch {
			case t.Name.Space == presentationNS && (t.Name.Local == "sld" || t.Name.Local == "notes"):
				slide.hidden = isFalse(attr(t, "show"))
			case t.Name.Space == presentationNS && t.Name.Local == "sp":
				placeholder, list = "text", nil
				shapes = append(shapes, false)
			case t.Name.Space == presentationNS && isShape(t.Name.Local):
				shapes = append(shapes, false)
			case t.Name.Space == presentationNS && t.Name.Local == "cNvPr" && len(shapes) > 0:
				shapes[len(shapes)-1] = isTrue(attr(t, "hidden"))
			case t.Name.Space == presentationNS && t.Name.Local == "ph":
				placeholder = attr(t, "type")
				if placeholder == "" {
					placeholder = "body"
				}
			case t.Name.Space == drawingNS && t.Name.Local == "tbl":
				table = &markdownTable{}
			case t.Name.Space == drawingNS && t.Name.Local == "tr" && table != nil:
				table.rows = append(table.rows, nil)
			case t.Name.Space == drawingNS && t.Name.Local == "tc" && table != nil && len(table.rows) > 0:
				table.rows[len(table.rows)-1] = append(table.rows[len(table.rows)-1], "")
			case t.Name.Space == drawingNS && t.Name.Local == "p":
				paragraph = &docxParagraph{outlineLvl: -1}
			case t.Name.Space == drawingNS && t.Name.Local == "pPr" && paragraph != nil:
				paragraph.level = listLevel(attr(t, "lvl"))
			case t.Name.Space == drawingNS && (t.Name.Local == "r" || t.Name.Local == "fld") && paragraph != nil:
				paragraph.runs = append(paragraph.runs, docxRun{})
				run = &paragraph.runs[len(paragraph.runs)-1]
			case t.Name.Space == drawingNS && t.Name.Local == "rPr" && run != nil:
				run.bold, run.italic = isTrue(attr(t, "b")), isTrue(attr(t, "i"))
			case t.Name.Space == drawingNS && t.Name.Local == "br" && paragraph != nil:
				paragraph.runs = append(paragraph.runs, docxRun{text: " "})
			case t.Name.Space == drawingNS && t.Name.Local == "t":
				inText = true
			}

		case xml.CharData:
			if inText && run != nil {
				run.text += string(t)
			}

		case xml.EndElement:
			switch {
			case t.Name.Space == drawingNS && t.Name.Local == "t":
				inText = false
			case t.Name.Space == drawingNS && (t.Name.Local == "r" || t.Name.Local == "fld"):
				run = nil
			case t.Name.Space == drawingNS && t.Name.Local == "p" && paragraph != nil:
				p := paragraph
				paragraph, run = nil, nil
				text := p.text()
				switch {
				case text == "" || hidden():
					// A hidden shape's text, such as an add-in's settings,
					// isn't the slide's
				case table != nil:
					table.add(plainParagraph(p.runs))
				case placeholder == "title" || placeholder == "ctrTitle":
					slide.title = strings.TrimSpace(slide.title + " " + plainParagraph(p.runs))
				case placeholder == "sldImg" || placeholder == "sldNum" || placeholder == "dt" || placeholder == "ftr" || placeholder == "hdr":
					// A notes' slide image, or a slide's number, date, footer
					// or header, isn't its text
				case placeholder == "text" || placeholder == "subTitle" || placeholder == "" || notes:
					slide.blocks = append(slide.blocks, escapeLineStart(text))
				default:
					list = append(list, strings.Repeat("  ", max(p.level, 0))+"- "+text)
				}
			case t.Name.Space == drawingNS && t.Name.Local == "tbl" && table != nil:
				if markdown := table.markdown(); markdown != "" {
					slide.blocks = append(slide.blocks, markdown)
				}
				table = nil
			case t.Name.Space == presentationNS && t.Name.Local == "sp":
				if len(list) > 0 {
					slide.blocks = append(slide.blocks, strings.Join(list, "\n"))
				}
				placeholder, list = "", nil
				shapes = shapes[:max(len(shapes)-1, 0)]
			case t.Name.Space == presentationNS && isShape(t.Name.Local):
				shapes = shapes[:max(len(shapes)-1, 0)]
			}
		}
	}
}

// isShape returns true when the element is a shape other than a text shape,
// which has its own visibility: a group, a graphic frame, a picture or a
// connector
func isShape(name string) bool {
	switch name {
	case "grpSp", "graphicFrame", "pic", "cxnSp":
		return true
	default:
		return false
	}
}

// isTrue and isFalse return true when the value is an XML boolean which is
// true or false
func isTrue(value string) bool  { return value == "1" || value == "true" }
func isFalse(value string) bool { return value == "0" || value == "false" }

// readRelationships returns the relationships of the part, which are in
// the _rels directory beside it, or none when it has none
func readRelationships(archive *zip.Reader, part string) partRelationships {
	var rels partRelationships
	_ = zipdoc.ReadXML(archive, path.Join(path.Dir(part), "_rels", path.Base(part)+".rels"), &rels)
	return rels
}

// target returns the target of the relationship with the identifier
func (r partRelationships) target(id string) (string, bool) {
	for _, rel := range r.Relationships {
		if rel.ID == id {
			return rel.Target, true
		}
	}
	return "", false
}

// targetOf returns the target of the first relationship of the type
func (r partRelationships) targetOf(relType string) (string, bool) {
	for _, rel := range r.Relationships {
		if rel.Type == relType {
			return rel.Target, true
		}
	}
	return "", false
}

// resolvePart returns the path of a relationship's target, which is
// relative to the part's directory, or absolute within the package
func resolvePart(part, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/")
	}
	return path.Join(path.Dir(part), target)
}
