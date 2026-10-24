package office

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// odfConverter converts an OpenDocument document's content to Markdown
type odfConverter struct {
	w         io.StringWriter
	lastBlock string

	paragraphs  []*odfParagraph  // the paragraphs being read, the last within the others
	tables      []*markdownTable // the tables being read, the last within the others
	lists       int              // the depth of the lists being read
	item        bool             // whether the list item's first paragraph hasn't been read
	spreadsheet bool             // whether the document is a spreadsheet
	slide       *odfSlide        // the slide being read, in a presentation
	frames      []string         // the classes of the presentation's frames being read
	notes       bool             // whether the slide's notes are being read
	slides      int              // the number of slides read
}

// odfParagraph is a paragraph being read, with its level, when it's a
// heading, which is zero otherwise
type odfParagraph struct {
	strings.Builder
	level int
}

// odfSlide is the text of a slide: its title, its other blocks, and its
// notes, and whether its last block is a list
type odfSlide struct {
	title         string
	blocks, notes []string
	list          bool
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// odfContentPath is the path of an OpenDocument document's content
	odfContentPath = "content.xml"

	// The namespaces of an OpenDocument document's content
	odfOfficeNS       = "urn:oasis:names:tc:opendocument:xmlns:office:1.0"
	odfTextNS         = "urn:oasis:names:tc:opendocument:xmlns:text:1.0"
	odfTableNS        = "urn:oasis:names:tc:opendocument:xmlns:table:1.0"
	odfDrawNS         = "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"
	odfPresentationNS = "urn:oasis:names:tc:opendocument:xmlns:presentation:1.0"

	// maxContentSize is the largest content which is read
	maxContentSize = 256 << 20

	// maxRepeatedRows is the most times a row with text is repeated, and
	// maxTableSize the most text of a table, with its repetitions, so a
	// small document can't expand without end
	maxRepeatedRows = 1000
	maxTableSize    = 16 << 20
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// odfMarkdown writes the content of an OpenDocument document to the writer
// as Markdown: a text document's headings, paragraphs, lists and tables, a
// spreadsheet's sheets, each with a heading of its name and its rows as a
// table, and a presentation's slides, each with a heading of its number and
// title, its outlines as lists, its other text, and its notes. Comments,
// tracked changes and footnotes aren't written, nor is bold or italic text,
// which is in a document's styles. Returns the writer's error, once it has
// its part.
func odfMarkdown(archive *zip.Reader, w io.StringWriter) error {
	f, err := archive.Open(odfContentPath)
	if err != nil {
		return metadata.ErrBadParameter.Withf("%s: %v", odfContentPath, err)
	}
	defer f.Close()
	c := &odfConverter{w: w}
	return c.convert(xml.NewDecoder(io.LimitReader(f, maxContentSize)))
}

// convert reads the document's content, writing each block, sheet or slide
// when it ends
func (c *odfConverter) convert(d *xml.Decoder) error {
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return metadata.ErrBadParameter.Withf("%s: %v", odfContentPath, err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			if err := c.start(d, t); err != nil {
				return err
			}
		case xml.CharData:
			if p := last(c.paragraphs); p != nil {
				p.Write(t)
			}
		case xml.EndElement:
			if err := c.end(t); err != nil {
				return err
			}
		}
	}
}

// start reads the start of an element
func (c *odfConverter) start(d *xml.Decoder, t xml.StartElement) error {
	switch {
	// Comments, tracked changes and footnotes aren't read
	case t.Name.Space == odfOfficeNS && t.Name.Local == "annotation",
		t.Name.Space == odfTextNS && (t.Name.Local == "tracked-changes" || t.Name.Local == "note"):
		if err := d.Skip(); err != nil {
			return metadata.ErrBadParameter.Withf("%s: %v", odfContentPath, err)
		}

	// Paragraphs and headings, and their spaces, tabs and line breaks
	case t.Name.Space == odfTextNS && (t.Name.Local == "p" || t.Name.Local == "h"):
		paragraph := new(odfParagraph)
		if t.Name.Local == "h" {
			level, err := strconv.Atoi(odfAttr(t, "outline-level"))
			if err != nil || level < 1 {
				level = 1
			}
			paragraph.level = min(level, 6)
		}
		c.paragraphs = append(c.paragraphs, paragraph)
	case t.Name.Space == odfOfficeNS && t.Name.Local == "spreadsheet":
		c.spreadsheet = true
	case t.Name.Space == odfTextNS && t.Name.Local == "s":
		if p := last(c.paragraphs); p != nil {
			count, err := strconv.Atoi(odfAttr(t, "c"))
			if err != nil || count < 1 {
				count = 1
			}
			p.WriteString(strings.Repeat(" ", min(count, 100)))
		}
	case t.Name.Space == odfTextNS && (t.Name.Local == "tab" || t.Name.Local == "line-break"):
		if p := last(c.paragraphs); p != nil {
			p.WriteString(" ")
		}

	// Lists
	case t.Name.Space == odfTextNS && t.Name.Local == "list":
		c.lists++
	case t.Name.Space == odfTextNS && t.Name.Local == "list-item":
		c.item = true

	// Tables, and their rows and cells
	case t.Name.Space == odfTableNS && t.Name.Local == "table":
		c.tables = append(c.tables, &markdownTable{name: odfAttr(t, "name")})
	case t.Name.Space == odfTableNS && t.Name.Local == "table-row":
		if table := last(c.tables); table != nil {
			repeat, err := strconv.Atoi(odfAttr(t, "number-rows-repeated"))
			if err != nil || repeat < 1 {
				repeat = 1
			}
			table.rowRepeat = repeat
			table.rows = append(table.rows, nil)
		}
	case t.Name.Space == odfTableNS && (t.Name.Local == "table-cell" || t.Name.Local == "covered-table-cell"):
		if table := last(c.tables); table != nil && len(table.rows) > 0 {
			repeat, err := strconv.Atoi(odfAttr(t, "number-columns-repeated"))
			if err != nil || repeat < 1 {
				repeat = 1
			}
			table.repeat = repeat
			table.value = odfCellValue(t)
			table.rows[len(table.rows)-1] = append(table.rows[len(table.rows)-1], "")
		}

	// A presentation's slides, frames and notes
	case t.Name.Space == odfDrawNS && t.Name.Local == "page":
		c.slide = new(odfSlide)
		c.slides++
	case t.Name.Space == odfDrawNS && t.Name.Local == "frame":
		c.frames = append(c.frames, odfAttr(t, "class"))
	case t.Name.Space == odfPresentationNS && t.Name.Local == "notes":
		c.notes = true
	}
	return nil
}

// end reads the end of an element, writing a block, sheet or slide when it
// ends
func (c *odfConverter) end(t xml.EndElement) error {
	switch {
	case t.Name.Space == odfTextNS && (t.Name.Local == "p" || t.Name.Local == "h"):
		if len(c.paragraphs) == 0 {
			return nil
		}
		p := c.paragraphs[len(c.paragraphs)-1]
		c.paragraphs = c.paragraphs[:len(c.paragraphs)-1]
		raw := strings.Join(strings.Fields(p.String()), " ")
		text := escapeMarkdown(raw)
		switch {
		case text == "":
			return nil
		case len(c.paragraphs) > 0:
			// A paragraph within another, such as in a text box, is part of
			// it, which escapes it
			c.paragraphs[len(c.paragraphs)-1].WriteString(" " + raw + " ")
			return nil
		case len(c.tables) > 0:
			// A paragraph in a table is part of its cell
			c.tables[len(c.tables)-1].add(text)
			return nil
		case c.slide != nil:
			c.addToSlide(text)
			return nil
		case p.level > 0:
			return c.writeBlock("heading", strings.Repeat("#", p.level)+" "+text)
		case c.lists > 0:
			return c.writeListItem(text)
		default:
			return c.writeBlock("paragraph", escapeLineStart(text))
		}

	case t.Name.Space == odfTextNS && t.Name.Local == "list":
		c.lists = max(c.lists-1, 0)

	case t.Name.Space == odfTableNS && (t.Name.Local == "table-cell" || t.Name.Local == "covered-table-cell"):
		// A cell without text has its value, such as a number, and a repeated
		// cell with text is repeated, up to the most columns
		if table := last(c.tables); table != nil && len(table.rows) > 0 {
			row := table.rows[len(table.rows)-1]
			if len(row) > 0 && row[len(row)-1] == "" {
				row[len(row)-1] = escapeMarkdown(table.value)
			}
			for i := 1; i < table.repeat && len(row) < maxColumns; i++ {
				row = append(row, row[len(row)-1])
			}
			table.rows[len(table.rows)-1] = row
		}
	case t.Name.Space == odfTableNS && t.Name.Local == "table-row":
		// A row without text isn't the table's, nor are its empty cells
		if table := last(c.tables); table != nil && len(table.rows) > 0 {
			row := table.rows[len(table.rows)-1]
			for len(row) > 0 && row[len(row)-1] == "" {
				row = row[:len(row)-1]
			}
			if len(row) == 0 {
				table.rows = table.rows[:len(table.rows)-1]
			} else {
				// A repeated row with text is repeated, up to the most rows
				// which are repeated, and the table's most text, beyond which
				// its rows aren't read
				row = row[:min(len(row), maxColumns)]
				size := 0
				for _, cell := range row {
					size += len(cell) + 3
				}
				table.rows = table.rows[:len(table.rows)-1]
				for i := 0; i < min(table.rowRepeat, maxRepeatedRows) && table.size+size <= maxTableSize; i++ {
					table.rows = append(table.rows, row)
					table.size += size
				}
			}
		}
	case t.Name.Space == odfTableNS && t.Name.Local == "table":
		table := last(c.tables)
		if table == nil {
			return nil
		}
		c.tables = c.tables[:len(c.tables)-1]
		switch {
		case len(c.tables) > 0:
			c.tables[len(c.tables)-1].add(table.plain())
		case c.slide != nil && c.notes:
			if markdown := table.markdown(); markdown != "" {
				c.slide.notes = append(c.slide.notes, markdown)
			}
		case c.slide != nil:
			if markdown := table.markdown(); markdown != "" {
				c.slide.blocks = append(c.slide.blocks, markdown)
			}
		case c.spreadsheet:
			// A spreadsheet's sheet, with a heading of its name
			markdown := "## Sheet: " + escapeMarkdown(table.name)
			if body := table.markdown(); body != "" {
				markdown += "\n\n" + body
			}
			return c.writeBlock("sheet", markdown)
		default:
			if markdown := table.markdown(); markdown != "" {
				return c.writeBlock("table", markdown)
			}
		}

	case t.Name.Space == odfDrawNS && t.Name.Local == "frame":
		c.frames = c.frames[:max(len(c.frames)-1, 0)]
	case t.Name.Space == odfPresentationNS && t.Name.Local == "notes":
		c.notes = false
	case t.Name.Space == odfDrawNS && t.Name.Local == "page":
		slide := c.slide
		c.slide = nil
		if slide == nil {
			return nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("## Slide %d", c.slides))
		if slide.title != "" {
			b.WriteString(": " + slide.title)
		}
		for _, block := range slide.blocks {
			b.WriteString("\n\n" + block)
		}
		if len(slide.notes) > 0 {
			b.WriteString("\n\n### Notes")
			for _, note := range slide.notes {
				b.WriteString("\n\n" + note)
			}
		}
		return c.writeBlock("slide", b.String())
	}
	return nil
}

// addToSlide adds a paragraph to the slide being read: its title, which is
// its title frame's, a list item, when it's in a list, its notes, or
// another block
func (c *odfConverter) addToSlide(text string) {
	class := ""
	if len(c.frames) > 0 {
		class = c.frames[len(c.frames)-1]
	}
	switch {
	case c.notes:
		c.slide.notes = append(c.slide.notes, escapeLineStart(text))
	case class == "title":
		c.slide.title = strings.TrimSpace(c.slide.title + " " + text)
	case class == "page-number" || class == "date-time" || class == "footer" || class == "header":
		// A slide's number, date, footer or header isn't its text
	case c.lists > 0 || class == "outline":
		// A list item is part of the list before it
		item := strings.Repeat("  ", max(c.lists-1, 0)) + "- " + text
		if c.slide.list {
			c.slide.blocks[len(c.slide.blocks)-1] += "\n" + item
		} else {
			c.slide.blocks = append(c.slide.blocks, item)
		}
		c.slide.list = true
		return
	default:
		c.slide.blocks = append(c.slide.blocks, escapeLineStart(text))
	}
	c.slide.list = false
}

// writeListItem writes a paragraph of a list: the first of an item, with a
// bullet, indented by the list's depth, or another, indented beneath it
func (c *odfConverter) writeListItem(text string) error {
	indent := strings.Repeat("  ", max(c.lists-1, 0))
	if c.item {
		c.item = false
		return c.writeBlock("list", indent+"- "+text)
	}
	return c.writeBlock("list", indent+"  "+text)
}

// writeBlock writes a block of Markdown, after a blank line, or a line
// break between the items of a list
func (c *odfConverter) writeBlock(kind, markdown string) error {
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

// odfCellValue returns the value of a cell which it has as an attribute,
// such as a spreadsheet's number, date, time or boolean, which is its text
// when it has none
func odfCellValue(e xml.StartElement) string {
	for _, a := range e.Attr {
		if a.Name.Space != odfOfficeNS {
			continue
		}
		switch a.Name.Local {
		case "value", "date-value", "time-value", "boolean-value", "string-value":
			return a.Value
		}
	}
	return ""
}

// odfAttr returns the value of the element's attribute, whatever its
// namespace
func odfAttr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
