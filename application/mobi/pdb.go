package mobi

import (
	"bytes"
	"encoding/binary"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// book is a MOBI book, which is a Palm database of records: its first is
// its headers, and those after are its text, its images, and its indexes
type book struct {
	r       io.ReaderAt
	offsets []uint32 // the offset of each record, and the end of the last
	header  header
	exth    []exthRecord
}

// header is what's read from a book's first record: its PalmDOC header and
// its MOBI header
type header struct {
	compression    uint16 // 1 is none, 2 is PalmDOC, and 17480 is HUFF/CDIC
	textLength     uint32 // the length of the text, when decompressed
	textRecords    uint16 // the number of text records, from the second
	encryption     uint16 // 0 is none, and otherwise it's DRM
	headerLength   uint32 // the length of the MOBI header
	encoding       uint32 // 1252 or 65001, which is UTF-8
	version        uint32 // the format's version, which is 8 for KF8
	fullName       string // the book's title
	firstImage     uint32 // the record of the first image
	hasEXTH        bool   // whether there's an EXTH header
	extraDataFlags uint16 // the trailing entries of each text record
}

// exthRecord is a record of a book's EXTH header, which is its metadata
type exthRecord struct {
	kind uint32
	data []byte
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// pdbHeaderSize is the size of a Palm database's header, which is
	// followed by its list of records, of recordEntrySize bytes each
	pdbHeaderSize   = 78
	recordEntrySize = 8

	// palmDOCHeaderSize is the size of the PalmDOC header at the start of
	// the first record, which is followed by the MOBI header
	palmDOCHeaderSize = 16

	// The type and creator of a MOBI book, at pdbTypeOffset in its header
	pdbTypeOffset = 60
	pdbType       = "BOOKMOBI"

	// maxRecords is the most records a book can have, and maxRecordSize the
	// largest record which is read
	maxRecords    = 65535
	maxRecordSize = 16 << 20

	// noRecord is a record index which isn't set
	noRecord = 0xFFFFFFFF
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// open reads the headers of a MOBI book, from its first record, which has
// its PalmDOC header, its MOBI header, and its EXTH header. A file which
// isn't a MOBI book is a bad parameter.
func open(r io.ReaderAt, size int64) (*book, error) {
	// The Palm database header, which has the type, and the number of records
	pdb := make([]byte, pdbHeaderSize)
	if _, err := r.ReadAt(pdb, 0); err != nil {
		return nil, metadata.ErrBadParameter.Withf("not a mobi book: %v", err)
	} else if string(pdb[pdbTypeOffset:pdbTypeOffset+len(pdbType)]) != pdbType {
		return nil, metadata.ErrBadParameter.With("not a mobi book")
	}
	count := int(binary.BigEndian.Uint16(pdb[76:]))
	if count == 0 || count > maxRecords {
		return nil, metadata.ErrBadParameter.Withf("not a mobi book: %d records", count)
	}

	// The offsets of the records, which ascend, and end at the end of the file
	list := make([]byte, count*recordEntrySize)
	if _, err := r.ReadAt(list, pdbHeaderSize); err != nil {
		return nil, metadata.ErrBadParameter.Withf("not a mobi book: %v", err)
	}
	b := &book{r: r, offsets: make([]uint32, count+1)}
	for i := 0; i < count; i++ {
		b.offsets[i] = binary.BigEndian.Uint32(list[i*recordEntrySize:])
		if i > 0 && b.offsets[i] < b.offsets[i-1] {
			return nil, metadata.ErrBadParameter.With("not a mobi book: its records are out of order")
		}
	}
	b.offsets[count] = uint32(min(size, int64(^uint32(0))))

	// The first record, which has the headers
	record, err := b.record(0)
	if err != nil {
		return nil, err
	} else if err := b.readHeaders(record); err != nil {
		return nil, err
	}
	return b, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// record returns the record of the book with the index, or an error when
// there's no such record, or it's too large
func (b *book) record(i int) ([]byte, error) {
	if i < 0 || i+1 >= len(b.offsets) {
		return nil, metadata.ErrNotFound.Withf("no record %d", i)
	}
	start, end := b.offsets[i], b.offsets[i+1]
	if end < start || end-start > maxRecordSize {
		return nil, metadata.ErrBadParameter.Withf("record %d is too large", i)
	}
	data := make([]byte, end-start)
	if n, err := b.r.ReadAt(data, int64(start)); err != nil && !(err == io.EOF && n == len(data)) {
		return nil, metadata.ErrBadParameter.Withf("record %d: %v", i, err)
	}
	return data, nil
}

// readHeaders reads the PalmDOC, MOBI and EXTH headers from the first
// record of the book
func (b *book) readHeaders(record []byte) error {
	if len(record) < palmDOCHeaderSize+8 || string(record[palmDOCHeaderSize:palmDOCHeaderSize+4]) != "MOBI" {
		return metadata.ErrBadParameter.With("not a mobi book: no mobi header")
	}
	h := &b.header
	h.compression = binary.BigEndian.Uint16(record[0:])
	h.textLength = binary.BigEndian.Uint32(record[4:])
	h.textRecords = binary.BigEndian.Uint16(record[8:])
	h.encryption = binary.BigEndian.Uint16(record[12:])

	// The MOBI header, whose fields are read when it's long enough to have
	// them
	mobi := record[palmDOCHeaderSize:]
	h.headerLength = binary.BigEndian.Uint32(mobi[4:])
	u32 := func(offset int) uint32 {
		if offset+4 > len(mobi) || uint32(offset+4) > h.headerLength {
			return 0
		}
		return binary.BigEndian.Uint32(mobi[offset:])
	}
	h.encoding = u32(12)
	h.version = u32(20)
	h.firstImage = u32(92)
	h.hasEXTH = u32(112)&0x40 != 0
	if 228 <= h.headerLength && 228 <= len(mobi) {
		h.extraDataFlags = binary.BigEndian.Uint16(mobi[226:])
	}
	if offset, length := u32(68), u32(72); offset > 0 && length > 0 && int64(offset)+int64(length) <= int64(len(record)) {
		h.fullName = string(record[offset : offset+length])
	}

	// The EXTH header, which follows the MOBI header
	if h.hasEXTH {
		b.exth = readEXTH(record, int(palmDOCHeaderSize+h.headerLength))
	}
	return nil
}

// readEXTH returns the records of the EXTH header at the offset of the
// record, or none when there isn't one there, which stops at a record which
// is cut short
func readEXTH(record []byte, offset int) []exthRecord {
	if offset < 0 || offset+12 > len(record) || string(record[offset:offset+4]) != "EXTH" {
		return nil
	}
	count := int(binary.BigEndian.Uint32(record[offset+8:]))
	var result []exthRecord
	pos := offset + 12
	for i := 0; i < count && pos+8 <= len(record); i++ {
		kind := binary.BigEndian.Uint32(record[pos:])
		length := int(binary.BigEndian.Uint32(record[pos+4:]))
		if length < 8 || pos+length > len(record) {
			break
		}
		result = append(result, exthRecord{kind: kind, data: bytes.Clone(record[pos+8 : pos+length])})
		pos += length
	}
	return result
}
