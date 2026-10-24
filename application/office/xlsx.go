package office

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	zipdoc "github.com/mutablelogic/go-metadata/internal/zipdoc"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// xlsxWorkbook is an Excel workbook's sheets, in order, by the identifiers
// of their relationships
type xlsxWorkbook struct {
	Sheets []struct {
		Name  string `xml:"name,attr"`
		State string `xml:"state,attr"`
		RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	} `xml:"sheets>sheet"`
}

// xlsxCell is a cell of a row, by its column, from zero
type xlsxCell struct {
	column int
	text   string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// xlsxWorkbookPath is the path of a workbook's sheets, and
	// xlsxSharedStringsPath of the text of its cells
	xlsxWorkbookPath      = "xl/workbook.xml"
	xlsxSharedStringsPath = "xl/sharedStrings.xml"

	// spreadsheetNS is the namespace of a workbook's elements
	spreadsheetNS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"

	// maxSheetSize is the largest sheet, or shared strings, which is read
	maxSheetSize = 256 << 20

	// maxColumns is the most columns of a sheet which are written, and
	// maxSheetColumns the most a sheet has, whose last is XFD
	maxColumns      = 100
	maxSheetColumns = 16384
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// xlsxMarkdown writes the content of an Excel workbook to the writer as
// Markdown: each sheet which isn't hidden, in order, with a heading of its
// name, and its rows which have text as a table, whose first row is its
// header. A cell's value is as it's stored, so a date is a number. Returns
// the writer's error, once it has its part.
func xlsxMarkdown(archive *zip.Reader, w io.StringWriter) error {
	var workbook xlsxWorkbook
	if err := zipdoc.ReadXML(archive, xlsxWorkbookPath, &workbook); err != nil {
		return err
	}
	rels := readRelationships(archive, xlsxWorkbookPath)
	shared, err := readSharedStrings(archive)
	if err != nil {
		return err
	}

	first := true
	for _, sheet := range workbook.Sheets {
		target, exists := rels.target(sheet.RelID)
		if !exists || sheet.State == "hidden" || sheet.State == "veryHidden" {
			continue
		}
		name := resolvePart(xlsxWorkbookPath, target)

		// The sheet's width, which is its widest row's, and its heading
		width, err := readSheet(archive, name, shared, nil)
		if err != nil {
			return err
		}
		heading := "## Sheet: " + escapeMarkdown(strings.TrimSpace(sheet.Name))
		if !first {
			heading = "\n\n" + heading
		}
		first = false
		if _, err := w.WriteString(heading); err != nil {
			return err
		} else if width == 0 {
			continue
		}

		// Its rows, as a table, whose first row is its header
		header := true
		if _, err := readSheet(archive, name, shared, func(row []xlsxCell) error {
			cells := make([]string, min(width, maxColumns))
			for _, cell := range row {
				if cell.column >= 0 && cell.column < len(cells) {
					cells[cell.column] = strings.ReplaceAll(escapeMarkdown(cell.text), "|", `\|`)
				}
			}
			line := "\n| " + strings.Join(cells, " | ") + " |"
			if header {
				line = "\n" + line + "\n|" + strings.Repeat(" --- |", len(cells))
				header = false
			}
			_, err := w.WriteString(line)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// readSharedStrings returns the text of a workbook's cells, which they
// refer to by its index, or none when it has none
func readSharedStrings(archive *zip.Reader) ([]string, error) {
	f, err := archive.Open(xlsxSharedStringsPath)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	d := xml.NewDecoder(io.LimitReader(f, maxSheetSize))

	var shared []string
	var text strings.Builder
	inItem, inText, inPhonetic := false, false, false
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return shared, nil
		} else if err != nil {
			return nil, metadata.ErrBadParameter.Withf("%s: %v", xlsxSharedStringsPath, err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Local == "si":
				inItem = true
				text.Reset()
			case t.Name.Local == "rPh":
				inPhonetic = true
			case t.Name.Local == "t" && inItem && !inPhonetic:
				inText = true
			}
		case xml.CharData:
			if inText {
				text.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "rPh":
				inPhonetic = false
			case "si":
				shared = append(shared, text.String())
				inItem = false
			}
		}
	}
}

// readSheet reads the rows of a sheet which have text, calling the function
// with each, when it's set, and returns the sheet's width, which is the
// number of columns of its widest row which has text
func readSheet(archive *zip.Reader, name string, shared []string, fn func([]xlsxCell) error) (int, error) {
	f, err := archive.Open(name)
	if err != nil {
		return 0, metadata.ErrBadParameter.Withf("%s: %v", name, err)
	}
	defer f.Close()
	d := xml.NewDecoder(io.LimitReader(f, maxSheetSize))

	width, next := 0, 0
	var row []xlsxCell
	var cell *xlsxCell
	var cellType string
	var value strings.Builder
	inValue, inInline, inPhonetic := false, false, false
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return width, nil
		} else if err != nil {
			return 0, metadata.ErrBadParameter.Withf("%s: %v", name, err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Space != spreadsheetNS {
				continue
			}
			switch t.Name.Local {
			case "row":
				row, next = nil, 0
			case "c":
				cell = &xlsxCell{column: columnIndex(attrNamed(t, "r"), next)}
				next = cell.column + 1
				cellType = attrNamed(t, "t")
				value.Reset()
			case "v":
				inValue = true
			case "is":
				inInline = true
			case "rPh":
				inPhonetic = true
			case "t":
				inValue = inInline && !inPhonetic
			}
		case xml.CharData:
			if inValue {
				value.Write(t)
			}
		case xml.EndElement:
			if t.Name.Space != spreadsheetNS {
				continue
			}
			switch t.Name.Local {
			case "v", "t":
				inValue = false
			case "is":
				inInline = false
			case "rPh":
				inPhonetic = false
			case "c":
				if cell != nil {
					cell.text = strings.Join(strings.Fields(cellText(cellType, value.String(), shared)), " ")
					if cell.text != "" && cell.column >= 0 {
						row = append(row, *cell)
					}
				}
				cell = nil
			case "row":
				if len(row) == 0 {
					continue
				}
				width = max(width, row[len(row)-1].column+1)
				if fn != nil {
					if err := fn(row); err != nil {
						return 0, err
					}
				}
			}
		}
	}
}

// cellText returns the text of a cell of the type, by its value: a shared
// string, a boolean, or a number, which is rounded to the fifteen digits
// which Excel shows, or else its value
func cellText(cellType, value string, shared []string) string {
	switch cellType {
	case "s":
		if i, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && i >= 0 && i < len(shared) {
			return shared[i]
		}
		return ""
	case "b":
		if strings.TrimSpace(value) == "1" {
			return "TRUE"
		}
		return "FALSE"
	case "", "n":
		if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			if rounded, err := strconv.ParseFloat(strconv.FormatFloat(f, 'g', 15, 64), 64); err == nil {
				return strconv.FormatFloat(rounded, 'f', -1, 64)
			}
		}
	}
	return value
}

// columnIndex returns the column of a cell's reference, such as 2 for C3,
// from zero, or the next column when it has no reference, or -1 when it's
// beyond a sheet's last column
func columnIndex(ref string, next int) int {
	column := 0
	for _, r := range strings.ToUpper(ref) {
		if r < 'A' || r > 'Z' {
			break
		}
		column = column*26 + int(r-'A'+1)
		if column > maxSheetColumns {
			return -1
		}
	}
	if column == 0 {
		return next
	}
	return column - 1
}

// attrNamed returns the value of the element's attribute without a
// namespace
func attrNamed(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name && a.Name.Space == "" {
			return a.Value
		}
	}
	return ""
}
