// Package mobi registers handlers of Mobipocket and Kindle books, such as
// .mobi, .prc, .azw and .azw3 files, which return their metadata, their
// cover, and their text as Markdown
package mobi

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"maps"
	"mime"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	isbns "github.com/mutablelogic/go-metadata/internal/isbn"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	title "github.com/mutablelogic/go-metadata/internal/title"
	charmap "golang.org/x/text/encoding/charmap"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of a book which isn't
	// Dublin Core
	Namespace = "mobi"

	// coverKey is the key of the cover of a book, as artwork
	coverKey = "cover"

	// encodingUTF8 is the text encoding of a book which is UTF-8, rather than
	// Windows-1252
	encodingUTF8 = 65001

	// maxBookSize is the largest book which is read into memory, when it
	// can't be read in place
	maxBookSize = 64 << 20
)

// The types of the records of an EXTH header
const (
	exthAuthor      = 100
	exthPublisher   = 101
	exthDescription = 103
	exthISBN        = 104
	exthSubject     = 105
	exthDate        = 106
	exthContributor = 108
	exthRights      = 109
	exthASIN        = 113
	exthCoverOffset = 201
	exthThumbOffset = 202
	exthDocType     = 501
	exthTitle       = 503
	exthLanguage    = 524
)

var (
	reContentType = regexp.MustCompile(`^application/(?:x-mobipocket-ebook|vnd\.amazon\.ebook|vnd\.amazon\.mobi8-ebook|x-mobi8-ebook)$`)

	// bookDates are the layouts of a book's publishing date
	bookDates = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02", "2006-01", "2006"}
)

// mobiTypes are the content types of books, by their extension, which are
// detected by their extension
var mobiTypes = map[string]string{
	".mobi": "application/x-mobipocket-ebook",
	".prc":  "application/x-mobipocket-ebook",
	".azw":  "application/vnd.amazon.ebook",
	".azw3": "application/vnd.amazon.mobi8-ebook",
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	for ext, contentType := range mobiTypes {
		if err := mime.AddExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	// Books, whose cover is returned only when artwork is requested, and text
	// only when Markdown is
	metadata.AddHandler(reContentType, Namespace, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		meta, err := mobiMetadata(ctx, r, o)
		return append(meta, metadata.KindMetadata(dc.KindBook)...), err
	}, dc.Namespace, Namespace, metadata.ArtworkNamespace, metadata.MarkdownNamespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// mobiMetadata returns the metadata of a book, from its EXTH header: its
// title, authors, publisher, description, ISBN, subjects, date, language and
// rights as Dublin Core, and its ASIN and type in the mobi namespace, its
// cover as artwork:cover, when artwork is requested, and its text as
// Markdown, when it's requested, a part at a time (see metadata.WithRange)
func mobiMetadata(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	ra, size, ok, err := readerAt(r)
	if err != nil || !ok {
		return nil, err
	}
	b, err := open(ra, size)
	if err != nil {
		return nil, err
	}

	// The metadata from the EXTH header, and the title, which is the updated
	// title, or else the full name
	entries := make(map[string]metadata.Metadata)
	add := func(ns, key string, value any) {
		if s, ok := value.(string); ok {
			if value = strings.TrimSpace(s); value == "" {
				return
			}
		}
		if entry := meta.New(ns, key, value); entry != nil {
			entries[ns+string(metadata.Separator)+key] = entry
		}
	}
	bookTitle := b.text(exthTitle)
	if bookTitle == "" {
		bookTitle = b.decode([]byte(b.header.fullName))
	}
	if !title.Untitled(bookTitle) {
		add(dc.Namespace, dc.Title, bookTitle)
	}
	authors := b.texts(exthAuthor)
	if len(authors) == 0 {
		authors = b.texts(exthContributor)
	}
	add(dc.Namespace, dc.Creator, strings.Join(authors, ", "))
	add(dc.Namespace, dc.Publisher, b.text(exthPublisher))
	add(dc.Namespace, dc.Description, itext.PlainText(b.text(exthDescription)))
	add(dc.Namespace, dc.Subject, strings.Join(b.texts(exthSubject), ", "))
	add(dc.Namespace, dc.Language, b.text(exthLanguage))
	add(dc.Namespace, dc.Rights, b.text(exthRights))
	if value, ok := isbns.Parse(b.text(exthISBN)); ok {
		add(dc.Namespace, dc.Identifier, value)
	}
	if date, ok := parseDate(b.text(exthDate)); ok {
		add(dc.Namespace, dc.Date, date)
	}
	add(Namespace, "asin", b.text(exthASIN))
	add(Namespace, "type", b.text(exthDocType))
	if b.header.encryption != 0 {
		add(Namespace, "encrypted", true)
	}

	// The cover, when artwork is requested
	if o.HasNamespace(metadata.ArtworkNamespace) {
		if cover := b.cover(o); cover != nil {
			entries[metadata.ArtworkNamespace+string(metadata.Separator)+coverKey] = cover
		}
	}

	// The text, when Markdown is requested
	if o.HasNamespace(metadata.MarkdownNamespace) {
		parts, err := b.markdown(ctx, o)
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			entries[part.NS()+string(metadata.Separator)+part.Key()] = part
		}
	}
	return slices.Collect(maps.Values(entries)), nil
}

// texts returns the values of the book's EXTH records of the kind, as text,
// without those which are empty, or repeated
func (b *book) texts(kind uint32) []string {
	var result []string
	for _, record := range b.exth {
		if record.kind != kind {
			continue
		}
		if value := strings.TrimSpace(b.decode(record.data)); value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

// text returns the value of the book's first EXTH record of the kind, as
// text, or an empty string when it has none
func (b *book) text(kind uint32) string {
	if values := b.texts(kind); len(values) > 0 {
		return values[0]
	}
	return ""
}

// decode returns the text in the book's encoding, which is UTF-8, or else
// Windows-1252, as UTF-8
func (b *book) decode(data []byte) string {
	data = bytes.TrimRight(data, "\x00")
	if b.header.encoding == encodingUTF8 || utf8.Valid(data) {
		return string(data)
	}
	text, err := charmap.Windows1252.NewDecoder().Bytes(data)
	if err != nil {
		return string(data)
	}
	return string(text)
}

// cover returns the book's cover as artwork:cover, downscaled to fit the
// largest image dimension, or nil when it has none, or it can't be decoded:
// the image whose offset from the first image is its cover offset, or else
// its thumbnail's
func (b *book) cover(o *metadata.Opts) metadata.Metadata {
	if b.header.firstImage == noRecord {
		return nil
	}
	for _, kind := range []uint32{exthCoverOffset, exthThumbOffset} {
		for _, record := range b.exth {
			if record.kind != kind || len(record.data) < 4 {
				continue
			}
			offset := binary.BigEndian.Uint32(record.data)
			if offset == noRecord {
				continue
			}
			data, err := b.record(int(b.header.firstImage) + int(offset))
			if err != nil {
				continue
			}
			if cover := artwork.Image(data, coverKey, o); cover != nil {
				return cover
			}
		}
	}
	return nil
}

// parseDate returns the book's publishing date, which is a W3C date, from a
// year to a time
func parseDate(value string) (time.Time, bool) {
	for _, layout := range bookDates {
		if t, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// readerAt returns the book to read at any offset, and its size, which is
// the reader itself when it can be, as Extract's readers can, and otherwise
// the book read into memory, or false when it's larger than maxBookSize
func readerAt(r io.Reader) (io.ReaderAt, int64, bool, error) {
	if ra, ok := r.(interface {
		io.ReaderAt
		Size() int64
	}); ok {
		return ra, ra.Size(), true, nil
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBookSize+1))
	if err != nil {
		return nil, 0, false, err
	} else if len(data) > maxBookSize {
		return nil, 0, false, nil
	}
	return bytes.NewReader(data), int64(len(data)), true, nil
}
