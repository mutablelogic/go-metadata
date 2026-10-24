package mobi

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	// Packages
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	converter "github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// The compressions of a book's text
	compressionNone    = 1
	compressionPalmDOC = 2
	compressionHuff    = 17480

	// maxTextLength is the most of a book's text which is decompressed
	maxTextLength = 64 << 20

	// The notes of a book whose text can't be read
	encryptedNote   = "The text of this book can't be read, as it's encrypted with DRM."
	compressionNote = "The text of this book can't be read yet, as it's compressed in a way which isn't supported, such as HUFF/CDIC."
)

// reBlankLines matches blank lines after a line, which are collapsed to one,
// as a book's HTML spaces its text with empty paragraphs and line breaks,
// and paragraphs of only non-breaking spaces
var reBlankLines = regexp.MustCompile(`\n(?:[ \t\x{00a0}]*\n){2,}`)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// markdown returns the book's text as Markdown, a part at a time (see
// metadata.WithRange), which is its text records, decompressed, which are
// HTML, converted to Markdown, with no more than one blank line between
// lines, or a markdown:note when its text can't be
// read, as it's encrypted, or compressed in a way which isn't supported. The
// text of a KF8 book, such as an .azw3 file, is in the order it's stored,
// which isn't always the order it's read.
func (b *book) markdown(ctx context.Context, o *metadata.Opts) ([]metadata.Metadata, error) {
	note := func(text string) []metadata.Metadata {
		return []metadata.Metadata{meta.New(metadata.MarkdownNamespace, "note", text)}
	}
	if b.header.encryption != 0 {
		return note(encryptedNote), nil
	} else if b.header.compression != compressionNone && b.header.compression != compressionPalmDOC {
		return note(compressionNote), nil
	}

	// Decompress the text records, without their trailing entries
	var text bytes.Buffer
	for i := 1; i <= int(b.header.textRecords) && text.Len() < maxTextLength; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, err := b.record(i)
		if err != nil {
			return nil, err
		}
		record = record[:len(record)-trailingSize(record, b.header.extraDataFlags)]
		if b.header.compression == compressionPalmDOC {
			record = decompress(record)
		}
		text.Write(record)
	}

	// Convert the HTML to Markdown, in the book's encoding
	html := b.decode(text.Bytes())
	data, err := htmltomarkdown.ConvertString(html, converter.WithContext(ctx))
	if err != nil {
		return nil, metadata.ErrBadParameter.Withf("the text of the book: %v", err)
	}
	w := imarkdown.NewWriter(o)
	_, err = w.WriteString(strings.TrimSpace(reBlankLines.ReplaceAllString(data, "\n\n")))
	return w.Metadata(err)
}

// trailingSize returns the size of the trailing entries at the end of a text
// record, which are its extra data: an entry for each of the flags but the
// lowest, whose size is a backward variable-width integer at its end, and,
// when the lowest flag is set, the bytes of a character which continues in
// the next record
func trailingSize(record []byte, flags uint16) int {
	size := 0
	for f := flags >> 1; f != 0; f >>= 1 {
		if f&1 != 0 {
			size += entrySize(record[:max(0, len(record)-size)])
		}
	}
	if flags&1 != 0 && len(record) > size {
		size += int(record[len(record)-size-1]&0x3) + 1
	}
	return min(size, len(record))
}

// entrySize returns the size of a trailing entry, which is a variable-width
// integer at the end of the data, read backwards, whose last byte has its
// high bit set
func entrySize(data []byte) int {
	size, shift := 0, 0
	for i := len(data) - 1; i >= 0 && shift < 28; i-- {
		size |= int(data[i]&0x7F) << shift
		shift += 7
		if data[i]&0x80 != 0 {
			break
		}
	}
	return size
}

// decompress returns the data decompressed with PalmDOC's compression, which
// is LZ77: a byte of 0x01 to 0x08 is followed by as many bytes as they
// are, 0x09 to 0x7F and 0x00 are themselves, 0x80 to 0xBF and the next byte
// are a distance back and a length of bytes to copy, and 0xC0 to 0xFF is a
// space followed by a character
func decompress(data []byte) []byte {
	out := make([]byte, 0, len(data)*2)
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case c >= 1 && c <= 8:
			end := min(i+1+int(c), len(data))
			out = append(out, data[i+1:end]...)
			i = end - 1
		case c < 0x80:
			out = append(out, c)
		case c >= 0xC0:
			out = append(out, ' ', c^0x80)
		default:
			if i+1 >= len(data) {
				return out
			}
			pair := int(c)<<8 | int(data[i+1])
			i++
			distance, length := (pair>>3)&0x07FF, (pair&0x07)+3
			if distance == 0 || distance > len(out) {
				continue
			}
			for j := 0; j < length; j++ {
				out = append(out, out[len(out)-distance])
			}
		}
	}
	return out
}
