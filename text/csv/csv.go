// Package csv registers a handler of comma-separated and tab-separated
// values, such as .csv and .tsv files, whose delimiter, which can be a comma,
// tab, semicolon or bar, is detected, which returns their kind, data, how
// many rows and columns they have, their headers, when their first row is
// one, and their rows as a Markdown table
package csv

import (
	"bufio"
	"bytes"
	"context"
	stdcsv "encoding/csv"
	"errors"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	charmap "golang.org/x/text/encoding/charmap"
	unicode "golang.org/x/text/encoding/unicode"
	transform "golang.org/x/text/transform"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// delimiter is a delimiter of values, and its name
type delimiter struct {
	r    rune
	name string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of comma-separated values
	Namespace = "csv"

	// The content types of comma-separated and tab-separated values
	ContentTypeCSV = "text/csv"
	ContentTypeTSV = "text/tab-separated-values"

	// sampleSize is how much of the start of the values is read, to detect
	// their character set and delimiter
	sampleSize = 64 << 10

	// sampleRows is how many rows are read, to detect the delimiter, and
	// whether the first row is a header
	sampleRows = 20

	// numericRows is the fraction of the sample's rows whose value in a
	// column is a number for the column to be numbers
	numericRows = 0.8
)

var (
	reContentType = regexp.MustCompile(`^(?:text/(?:csv|tab-separated-values)|application/csv)$`)

	// delimiters are the delimiters which are detected, in the order in
	// which they're preferred, when they're as consistent
	delimiters = []delimiter{{',', "comma"}, {'\t', "tab"}, {';', "semicolon"}, {'|', "bar"}}

	// bomUTF8 is the byte order mark of UTF-8, which a spreadsheet
	// application writes at the start of the values
	bomUTF8 = []byte("\xef\xbb\xbf")
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// Values are text, which are refined by their extension, so they're
	// detected the same on every platform, whatever its MIME types are
	for ext, contentType := range map[string]string{".csv": ContentTypeCSV, ".tsv": ContentTypeTSV} {
		if err := metadata.AddTextExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	// The kind of the values, how many rows and columns they have, their
	// headers, and their rows as a Markdown table, only when the markdown
	// namespace is requested, which are read a row at a time, as they can
	// be large
	metadata.AddHandler(reContentType, Namespace, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindData)
		br := bufio.NewReaderSize(r, sampleSize)
		sample, err := br.Peek(sampleSize)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
			return kind, err
		}
		text := decode(sample, br)
		delim := detectDelimiter(sampleText(sample), o.ContentType() == ContentTypeTSV)

		// Read the rows, the first of which can be a header, which is
		// detected from the first rows
		reader := newReader(text, delim.r)
		var rows [][]string
		var header []string
		var w *imarkdown.Writer
		var werr error
		if o.HasNamespace(metadata.MarkdownNamespace) {
			w = imarkdown.NewWriter(o)
		}
		count, columns := 0, 0
		write := func(row []string) {
			if w != nil && werr == nil {
				_, werr = w.WriteString(tableRow(row, columns))
			}
		}
		flush := func() {
			// The header is from the first rows, and the table starts with
			// it, or else with the columns' numbers
			if hasHeader(rows) {
				header, rows = rows[0], rows[1:]
			}
			for _, row := range rows {
				columns = max(columns, len(row))
			}
			columns = max(columns, len(header))
			if w != nil {
				names := header
				if names == nil {
					for i := range columns {
						names = append(names, strconv.Itoa(i+1))
					}
				}
				write(names)
				write(slices.Repeat([]string{"---"}, columns))
			}
			for _, row := range rows {
				write(row)
			}
			count += len(rows)
			rows = nil
		}
		sampled := false
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return kind, err
			}
			if !sampled {
				if rows = append(rows, row); len(rows) == sampleRows {
					flush()
					sampled = true
				}
				continue
			}
			count++
			columns = max(columns, len(row))
			write(row)
		}
		if !sampled {
			flush()
		}

		result := append(valuesMetadata(count, columns, delim, header), kind...)
		if count > 0 {
			result = append(result, metadata.HintMetadata(metadata.HintMarkdown)...)
		}
		if w == nil {
			return result, nil
		}
		markdown, err := w.Metadata(werr)
		return append(result, markdown...), err
	}, Namespace, dc.Namespace, metadata.MetadataNamespace, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// valuesMetadata returns the metadata of the values: how many rows of
// values they have, not counting the header, how many columns, the
// delimiter, and the headers, when the first row is a header
func valuesMetadata(rows, columns int, delim delimiter, header []string) []metadata.Metadata {
	result := []metadata.Metadata{
		meta.New(Namespace, "rows", rows),
		meta.New(Namespace, "columns", columns),
		meta.New(Namespace, "delimiter", delim.name),
	}
	if len(header) > 0 {
		result = append(result, meta.NewText(Namespace, "headers", header, strings.Join(header, ", ")))
	}
	return result
}

// newReader returns a reader of the values with the delimiter, whose rows
// can have different numbers of values, and whose quotes can be in values
// which aren't quoted, as a spreadsheet application can write them
func newReader(r io.Reader, delim rune) *stdcsv.Reader {
	reader := stdcsv.NewReader(r)
	reader.Comma = delim
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.ReuseRecord = false
	return reader
}

// decode returns a reader of the values as UTF-8, from the character set
// which the start of them is in: UTF-16 or UTF-8 with a byte order mark, or
// UTF-8 when it's valid UTF-8, or else Windows-1252, as a spreadsheet
// application on Windows writes them
func decode(sample []byte, r io.Reader) io.Reader {
	switch {
	case bytes.HasPrefix(sample, bomUTF8):
		return transform.NewReader(r, unicode.UTF8BOM.NewDecoder())
	case bytes.HasPrefix(sample, []byte{0xFF, 0xFE}), bytes.HasPrefix(sample, []byte{0xFE, 0xFF}):
		return transform.NewReader(r, unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder())
	case utf8.Valid(sample[:validEnd(sample)]):
		return r
	default:
		return transform.NewReader(r, charmap.Windows1252.NewDecoder())
	}
}

// sampleText returns the start of the values as text, to detect their
// delimiter
func sampleText(sample []byte) string {
	text, err := itext.Decode(sample, true)
	if err != nil {
		return string(sample)
	}
	return text
}

// validEnd returns the end of the data without a character which is cut off
// at its end
func validEnd(data []byte) int {
	for i := len(data) - 1; i >= 0 && i >= len(data)-utf8.UTFMax; i-- {
		if utf8.RuneStart(data[i]) {
			if utf8.FullRune(data[i:]) {
				return len(data)
			}
			return i
		}
	}
	return len(data)
}

// detectDelimiter returns the delimiter of the values which most of the
// first rows have the same number of values with, which is more than one,
// and of those, the delimiter which gives the most values, or else the
// first which is preferred, a tab for tab-separated values, and otherwise a
// comma
func detectDelimiter(sample string, tsv bool) delimiter {
	best, bestScore, bestColumns := delimiters[0], 0.0, 0
	if tsv {
		best = delimiters[1]
	}
	for _, d := range delimiters {
		reader := newReader(strings.NewReader(sample), d.r)
		widths := make(map[int]int)
		rows := 0
		for rows < sampleRows {
			row, err := reader.Read()
			if err != nil {
				break
			}
			widths[len(row)]++
			rows++
		}
		if rows == 0 {
			continue
		}
		width, most := 0, 0
		for w, n := range widths {
			if n > most || (n == most && w > width) {
				width, most = w, n
			}
		}
		if width < 2 {
			continue
		}
		score := float64(most) / float64(rows)
		if tsv && d.r == '\t' {
			score += 0.5
		}
		if score > bestScore || (score == bestScore && width > bestColumns) {
			best, bestScore, bestColumns = d, score, width
		}
	}
	return best
}

// hasHeader returns true when the first of the rows is a header: its values
// are all set, different, and aren't numbers, and at least one column of the
// other rows is numbers, or none of them are. A header of only one row
// isn't known to be one, and isn't.
func hasHeader(rows [][]string) bool {
	if len(rows) < 2 {
		return false
	}
	seen := make(map[string]bool)
	for _, value := range rows[0] {
		value = strings.TrimSpace(value)
		if value == "" || isNumber(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	numericColumns, votes := 0, 0
	for column, value := range rows[0] {
		numbers, values := 0, 0
		for _, row := range rows[1:] {
			if column < len(row) && strings.TrimSpace(row[column]) != "" {
				values++
				if isNumber(row[column]) {
					numbers++
				}
			}
		}
		if values > 0 && float64(numbers)/float64(values) >= numericRows {
			numericColumns++
			if !isNumber(value) {
				votes++
			}
		}
	}
	return votes > 0 || numericColumns == 0
}

// isNumber returns true when the value is a number, such as 42, -3.5, 1e6,
// 1,234 or 12%, or a currency amount, such as $10
func isNumber(value string) bool {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "$£€¥")
	value = strings.TrimSuffix(value, "%")
	value = strings.ReplaceAll(value, ",", "")
	if value == "" {
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

// tableRow returns a row of a Markdown table, with the values escaped, and
// as many cells as the columns
func tableRow(row []string, columns int) string {
	cells := make([]string, columns)
	for i := range cells {
		if i < len(row) {
			cells[i] = itext.Escape(strings.TrimSpace(row[i]))
		}
	}
	return "| " + strings.Join(cells, " | ") + " |\n"
}
