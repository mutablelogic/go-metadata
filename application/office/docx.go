package office

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	zipdoc "github.com/mutablelogic/go-metadata/internal/zipdoc"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// docxStyles are a Word document's styles, in word/styles.xml, by which its
// headings are known
type docxStyles struct {
	Styles []struct {
		Type    string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr"`
		ID      string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main styleId,attr"`
		Name    docxVal `xml:"name"`
		BasedOn docxVal `xml:"basedOn"`
		PPr     struct {
			OutlineLvl *docxVal `xml:"outlineLvl"`
			NumPr      *struct {
				Level *docxVal `xml:"ilvl"`
				NumID *docxVal `xml:"numId"`
			} `xml:"numPr"`
		} `xml:"pPr"`
	} `xml:"style"`
}

// docxNumbering is a Word document's numbering, in word/numbering.xml, by
// which its lists are bulleted or numbered
type docxNumbering struct {
	Abstract []struct {
		ID     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main abstractNumId,attr"`
		Levels []struct {
			Level  string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ilvl,attr"`
			NumFmt docxVal `xml:"numFmt"`
		} `xml:"lvl"`
	} `xml:"abstractNum"`
	Nums []struct {
		ID       string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numId,attr"`
		Abstract docxVal `xml:"abstractNumId"`
	} `xml:"num"`
}

// docxVal is an element whose value is its w:val attribute
type docxVal struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// docxRun is text of a paragraph, with its formatting
type docxRun struct {
	text         string
	bold, italic bool
}

// docxParagraph is a paragraph of a Word document, with its style, list,
// and its level in it, when they're its own rather than its style's, its
// outline level, which is -1 when it has none, its runs, and whether its
// last run is being read
type docxParagraph struct {
	style      string
	numID      string
	level      int
	hasLevel   bool
	outlineLvl int
	runs       []docxRun
	inRun      bool
}

// docxConverter converts a Word document to Markdown, with its styles and
// numbering
type docxConverter struct {
	styles    map[string]docxStyle
	bulleted  map[string]map[int]bool
	w         io.StringWriter
	lastBlock string
}

// docxStyle is a paragraph style, with the style it's based on, its outline
// level, and its list and level in it, when it has them
type docxStyle struct {
	name, basedOn    string
	outlineLvl       int
	numID            string
	level            int
	hasNum, hasLevel bool
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// docxDocumentPath is the path of a Word document's content, and its
	// styles and numbering
	docxDocumentPath  = "word/document.xml"
	docxStylesPath    = "word/styles.xml"
	docxNumberingPath = "word/numbering.xml"

	// wordNS is the namespace of a Word document's elements, and mcNS of
	// its markup compatibility, whose fallbacks repeat their choices
	wordNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	mcNS   = "http://schemas.openxmlformats.org/markup-compatibility/2006"

	// maxDocxSize is the largest content of a Word document which is read
	maxDocxSize = 256 << 20

	// maxListLevel is the deepest level of a list, from zero, which Word
	// and DrawingML have
	maxListLevel = 8
)

// headingStyleRe matches the name of a built-in heading style, which is in
// English, whatever the document's language
var headingStyleRe = regexp.MustCompile(`(?i)^heading\s*([1-9])$`)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// docxMarkdown writes the content of a Word document to the writer as
// Markdown: its paragraphs, with headings by their styles, lists as bulleted
// or numbered, tables, and bold and italic text. Deleted text, and the
// fallbacks of other content, aren't written. Returns the writer's error,
// once it has its part.
func docxMarkdown(archive *zip.Reader, w io.StringWriter) error {
	c := &docxConverter{styles: make(map[string]docxStyle), bulleted: make(map[string]map[int]bool), w: w}
	c.readStyles(archive)
	c.readNumbering(archive)

	f, err := archive.Open(docxDocumentPath)
	if err != nil {
		return metadata.ErrBadParameter.Withf("%s: %v", docxDocumentPath, err)
	}
	defer f.Close()
	return c.convert(xml.NewDecoder(io.LimitReader(f, maxDocxSize)))
}

// readStyles reads the document's paragraph styles, when it has them
func (c *docxConverter) readStyles(archive *zip.Reader) {
	var styles docxStyles
	if err := zipdoc.ReadXML(archive, docxStylesPath, &styles); err != nil {
		return
	}
	for _, s := range styles.Styles {
		if s.Type != "" && s.Type != "paragraph" {
			continue
		}
		style := docxStyle{name: s.Name.Val, basedOn: s.BasedOn.Val, outlineLvl: -1}
		if s.PPr.OutlineLvl != nil {
			if level, err := strconv.Atoi(s.PPr.OutlineLvl.Val); err == nil {
				style.outlineLvl = level
			}
		}
		if numPr := s.PPr.NumPr; numPr != nil {
			if numPr.NumID != nil {
				style.numID, style.hasNum = numPr.NumID.Val, true
			}
			if numPr.Level != nil {
				style.level, style.hasLevel = listLevel(numPr.Level.Val), true
			}
		}
		c.styles[s.ID] = style
	}
}

// readNumbering reads which of the document's lists, and their levels, are
// bulleted, when it has them
func (c *docxConverter) readNumbering(archive *zip.Reader) {
	var numbering docxNumbering
	if err := zipdoc.ReadXML(archive, docxNumberingPath, &numbering); err != nil {
		return
	}
	abstract := make(map[string]map[int]bool)
	for _, a := range numbering.Abstract {
		levels := make(map[int]bool)
		for _, l := range a.Levels {
			if level, err := strconv.Atoi(l.Level); err == nil {
				levels[level] = l.NumFmt.Val == "bullet" || l.NumFmt.Val == "none" || l.NumFmt.Val == ""
			}
		}
		abstract[a.ID] = levels
	}
	for _, n := range numbering.Nums {
		c.bulleted[n.ID] = abstract[n.Abstract.Val]
	}
}

// headingLevel returns the level of the heading which the paragraph is, by
// its own outline level, or its style's, or the style it's based on, or
// zero when it isn't one. A title is a first level heading.
func (c *docxConverter) headingLevel(p *docxParagraph) int {
	if p.outlineLvl >= 0 && p.outlineLvl < 9 {
		return min(p.outlineLvl+1, 6)
	}
	seen := make(map[string]bool)
	for id := p.style; id != "" && !seen[id]; id = c.styles[id].basedOn {
		seen[id] = true
		style, exists := c.styles[id]
		if !exists {
			// A document without styles names its headings by their IDs
			style = docxStyle{name: id, outlineLvl: -1}
		}
		if match := headingStyleRe.FindStringSubmatch(style.name); match != nil {
			level, _ := strconv.Atoi(match[1])
			return min(level, 6)
		} else if strings.EqualFold(style.name, "title") {
			return 1
		} else if style.outlineLvl >= 0 && style.outlineLvl < 9 {
			return min(style.outlineLvl+1, 6)
		}
	}
	return 0
}

// convert reads the document's content, writing each paragraph and table
// when it ends
func (c *docxConverter) convert(d *xml.Decoder) error {
	var paragraphs []*docxParagraph // the paragraphs being read, the last within the others, such as in a text box
	var tables []*markdownTable     // the tables being read, the last within the others
	inText := false
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return metadata.ErrBadParameter.Withf("%s: %v", docxDocumentPath, err)
		}
		paragraph := func() *docxParagraph {
			if len(paragraphs) == 0 {
				return nil
			}
			return paragraphs[len(paragraphs)-1]
		}()

		switch t := token.(type) {
		case xml.StartElement:
			// Deleted text, the formatting before tracked changes, and
			// fallbacks, which repeat other content, aren't read
			if (t.Name.Space == wordNS && (t.Name.Local == "del" || t.Name.Local == "moveFrom" || strings.HasSuffix(t.Name.Local, "PrChange"))) || (t.Name.Space == mcNS && t.Name.Local == "Fallback") {
				if err := d.Skip(); err != nil {
					return metadata.ErrBadParameter.Withf("%s: %v", docxDocumentPath, err)
				}
				continue
			}
			if t.Name.Space != wordNS {
				continue
			}
			switch t.Name.Local {
			case "p":
				paragraphs = append(paragraphs, &docxParagraph{outlineLvl: -1})
			case "pStyle":
				if paragraph != nil {
					paragraph.style = attr(t, "val")
				}
			case "outlineLvl":
				if paragraph != nil {
					if level, err := strconv.Atoi(attr(t, "val")); err == nil {
						paragraph.outlineLvl = level
					}
				}
			case "numId":
				if paragraph != nil {
					paragraph.numID = attr(t, "val")
				}
			case "ilvl":
				if paragraph != nil {
					paragraph.level, paragraph.hasLevel = listLevel(attr(t, "val")), true
				}
			case "r":
				if paragraph != nil {
					paragraph.runs = append(paragraph.runs, docxRun{})
					paragraph.inRun = true
				}
			case "b":
				if run := paragraph.run(); run != nil {
					run.bold = on(t)
				}
			case "i":
				if run := paragraph.run(); run != nil {
					run.italic = on(t)
				}
			case "t":
				inText = true
			case "tab", "br", "cr":
				if run := paragraph.run(); run != nil {
					run.text += " "
				}
			case "tbl":
				tables = append(tables, &markdownTable{})
			case "tr":
				if len(tables) > 0 {
					tables[len(tables)-1].rows = append(tables[len(tables)-1].rows, nil)
				}
			case "tc":
				if table := last(tables); table != nil && len(table.rows) > 0 {
					table.rows[len(table.rows)-1] = append(table.rows[len(table.rows)-1], "")
					table.repeat = 1
				}
			case "gridSpan":
				if table := last(tables); table != nil {
					span, err := strconv.Atoi(attr(t, "val"))
					if err != nil || span < 1 {
						span = 1
					}
					table.repeat = span
				}
			}

		case xml.CharData:
			if run := paragraph.run(); inText && run != nil {
				run.text += string(t)
			}

		case xml.EndElement:
			if t.Name.Space != wordNS {
				continue
			}
			switch t.Name.Local {
			case "t":
				inText = false
			case "r":
				if paragraph != nil {
					paragraph.inRun = false
				}
			case "p":
				if paragraph == nil {
					continue
				}
				paragraphs = paragraphs[:len(paragraphs)-1]
				text := paragraph.text()
				switch {
				case len(paragraphs) > 0:
					// A paragraph within another, such as in a text box in a
					// run, is part of it, with its runs, and the run it's in
					// continues after it, with the run's formatting
					outer := paragraphs[len(paragraphs)-1]
					var continued docxRun
					if run := outer.run(); run != nil {
						continued = docxRun{bold: run.bold, italic: run.italic}
					}
					outer.runs = append(append(append(outer.runs, docxRun{text: " "}), paragraph.runs...), docxRun{text: " "})
					if outer.inRun {
						outer.runs = append(outer.runs, continued)
					}
				case len(tables) > 0:
					// A paragraph in a table is part of its cell
					tables[len(tables)-1].add(plainParagraph(paragraph.runs))
				case text != "":
					if err := c.writeParagraph(paragraph, text); err != nil {
						return err
					}
				}
			case "tc":
				// A cell which spans columns is followed by the columns it
				// covers, up to the most columns
				if table := last(tables); table != nil && len(table.rows) > 0 {
					row := table.rows[len(table.rows)-1]
					for i := 1; i < table.repeat && len(row) < maxColumns; i++ {
						row = append(row, "")
					}
					table.rows[len(table.rows)-1] = row
				}
			case "tbl":
				table := tables[len(tables)-1]
				tables = tables[:len(tables)-1]
				if len(tables) > 0 {
					// A table within a table's cell is part of the cell
					tables[len(tables)-1].add(table.plain())
				} else if err := c.writeBlock("table", table.markdown()); err != nil {
					return err
				}
			}
		}
	}
}

// writeParagraph writes the paragraph, as a heading, a list item, or text
func (c *docxConverter) writeParagraph(p *docxParagraph, text string) error {
	if level := c.headingLevel(p); level > 0 {
		return c.writeBlock("heading", strings.Repeat("#", level)+" "+plainParagraph(p.runs))
	}
	if numID, level := c.numbering(p); numID != "" && numID != "0" {
		// A list, or its level, which isn't defined is bulleted
		marker := "-"
		if bulleted, defined := c.bulleted[numID][level]; defined && !bulleted {
			marker = "1."
		}
		return c.writeBlock("list", strings.Repeat("  ", level)+marker+" "+text)
	}
	return c.writeBlock("paragraph", escapeLineStart(text))
}

// numbering returns the list the paragraph is in, and its level in it,
// which are its own, or else its style's, or the style it's based on, such
// as List Bullet's, or an empty list when it isn't in one
func (c *docxConverter) numbering(p *docxParagraph) (string, int) {
	numID, level := p.numID, p.level
	hasNum, hasLevel := p.numID != "", p.hasLevel
	seen := make(map[string]bool)
	for id := p.style; id != "" && !seen[id] && (!hasNum || !hasLevel); id = c.styles[id].basedOn {
		seen[id] = true
		style := c.styles[id]
		if !hasNum && style.hasNum {
			numID, hasNum = style.numID, true
		}
		if !hasLevel && style.hasLevel {
			level, hasLevel = style.level, true
		}
	}
	return numID, level
}

// writeBlock writes a block of Markdown, after a blank line, or a line
// break between the items of a list
func (c *docxConverter) writeBlock(kind, markdown string) error {
	separator := "\n\n"
	switch {
	case c.lastBlock == "":
		separator = ""
	case kind == "list" && c.lastBlock == "list":
		separator = "\n"
	}
	c.lastBlock = kind
	_, err := c.w.WriteString(separator + markdown)
	return err
}

// text returns the paragraph's text as Markdown, with its bold and italic
// runs, and its whitespace collapsed
func (p *docxParagraph) text() string {
	var b strings.Builder
	for i := 0; i < len(p.runs); {
		run := p.runs[i]
		text := run.text
		j := i + 1
		for ; j < len(p.runs) && p.runs[j].bold == run.bold && p.runs[j].italic == run.italic; j++ {
			text += p.runs[j].text
		}
		b.WriteString(emphasise(escapeMarkdown(text), run.bold, run.italic))
		i = j
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// emphasise returns the text in bold or italic, keeping its spaces outside
// the emphasis, as Markdown requires
func emphasise(text string, bold, italic bool) string {
	marker := ""
	if bold {
		marker += "**"
	}
	if italic {
		marker += "_"
	}
	trimmed := strings.TrimSpace(text)
	if marker == "" || trimmed == "" {
		return text
	}
	start := text[:strings.Index(text, trimmed)]
	end := text[len(start)+len(trimmed):]
	closing := marker
	if bold && italic {
		closing = "_**"
	}
	return start + marker + trimmed + closing + end
}

// plainParagraph returns the runs' text, escaped as Markdown, without their
// formatting, and with their whitespace collapsed, as in a heading or table
func plainParagraph(runs []docxRun) string {
	var b strings.Builder
	for _, run := range runs {
		b.WriteString(run.text)
	}
	return escapeMarkdown(strings.Join(strings.Fields(b.String()), " "))
}

// run returns the paragraph's run which is being read, or nil when there's
// none, or no paragraph
func (p *docxParagraph) run() *docxRun {
	if p == nil || !p.inRun || len(p.runs) == 0 {
		return nil
	}
	return &p.runs[len(p.runs)-1]
}

// listLevel returns a level of a list, from zero, which is at most the
// deepest, so a malformed document can't indent a list without end
func listLevel(value string) int {
	level, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return min(max(level, 0), maxListLevel)
}

// attr returns the value of the element's attribute in Word's namespace
func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name && (a.Name.Space == wordNS || a.Name.Space == "") {
			return a.Value
		}
	}
	return ""
}

// on returns true when a property, such as bold, is on, which it is unless
// its value is false
func on(e xml.StartElement) bool {
	switch attr(e, "val") {
	case "0", "false", "off", "none":
		return false
	default:
		return true
	}
}
