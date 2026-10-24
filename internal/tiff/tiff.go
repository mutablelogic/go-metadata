// Package tiff reads the tags of the TIFF structure of EXIF data, in a JPEG
// image or a TIFF image, which libexif doesn't expose, or which are needed
// without cgo, such as the orientation of an image and its thumbnail
package tiff

import (
	"bytes"
	"encoding/binary"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// entry is an entry of an IFD, which is a tag, and its value when it's a
// SHORT or LONG
type entry struct {
	tag   uint16
	value uint32
	ok    bool
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// tagOrientation is the tag of IFD0 which is the orientation of the image
	tagOrientation = 0x0112

	// The tags of IFD1 which are the offset, from the TIFF header, and the
	// length of the thumbnail
	tagJPEGInterchangeFormat       = 0x0201
	tagJPEGInterchangeFormatLength = 0x0202

	// The formats of the values of the tags, which are SHORT and LONG
	formatShort = 3
	formatLong  = 4

	// ifdEntrySize is the size of an entry of an IFD, which is its tag,
	// format, number of components, and value or the offset of its value
	ifdEntrySize = 12

	// maxEXIFOffset is the furthest into a JPEG image its EXIF header is
	// looked for, which is in its APP1 segment, near its start, so the
	// image data isn't searched
	maxEXIFOffset = 128 << 10
)

var (
	// exifHeader is before the TIFF header in the APP1 segment of a JPEG
	// image, from which the offsets of the EXIF data are
	exifHeader = []byte("Exif\x00\x00")
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Data returns the data from the TIFF header, which is after the EXIF
// header in a JPEG image, or at the start of a TIFF image or EXIF data, or
// after the four bytes before the EXIF data of a HEIF image, or nil when
// there isn't one
func Data(data []byte) []byte {
	if i := bytes.Index(data[:min(len(data), maxEXIFOffset)], exifHeader); i >= 0 {
		return data[i+len(exifHeader):]
	}
	for _, start := range []int{0, 4} {
		if len(data) >= start+4 {
			if header := string(data[start : start+4]); header == "II*\x00" || header == "MM\x00*" {
				return data[start:]
			}
		}
	}
	return nil
}

// Normalize returns the data of a RAW image which is TIFF data with a
// header of its own, such as an Olympus ORF image, which is IIRO, or a
// Panasonic RW2 image, which is IIU, with a TIFF header, so it can be read
// as TIFF data, or the data itself when it has a TIFF header, or nil when
// it isn't TIFF data
func Normalize(data []byte) []byte {
	if len(data) < 8 {
		return nil
	}
	switch string(data[:4]) {
	case "II*\x00", "MM\x00*":
		return data
	case "IIRO", "IIRS", "IIU\x00":
		return append([]byte("II*\x00"), data[4:]...)
	case "MMOR":
		return append([]byte("MM\x00*"), data[4:]...)
	}
	return nil
}

// Orientation returns the orientation of the image, from 1 to 8, as EXIF
// defines it, which is the transformation to display it (see
// artwork.Orient), or 1 when it doesn't have one, or it isn't valid
func Orientation(data []byte) int {
	tiff := Data(data)
	if tiff == nil {
		return 1
	}
	order, ifd0, ok := header(tiff)
	if !ok {
		return 1
	}
	entries, _ := readIFD(tiff, order, ifd0)
	for _, e := range entries {
		if e.tag == tagOrientation && e.ok && e.value >= 1 && e.value <= 8 {
			return int(e.value)
		}
	}
	return 1
}

// Thumbnail returns the thumbnail which is embedded in the EXIF data, which
// is usually a small JPEG image, such as 160x120, or nil when there isn't
// one, or it's outside the data. It's the data at the offset and with the
// length of the JPEGInterchangeFormat and JPEGInterchangeFormatLength tags
// of IFD1, from the TIFF header. It isn't checked that it's an image, as it
// can be cut short, such as when the image data has been removed.
func Thumbnail(data []byte) []byte {
	tiff := Data(data)
	if tiff == nil {
		return nil
	}
	order, ifd0, ok := header(tiff)
	if !ok {
		return nil
	}
	_, ifd1 := readIFD(tiff, order, ifd0)
	entries, _ := readIFD(tiff, order, ifd1)
	var offset, length uint32
	for _, e := range entries {
		switch {
		case !e.ok:
			continue
		case e.tag == tagJPEGInterchangeFormat:
			offset = e.value
		case e.tag == tagJPEGInterchangeFormatLength:
			length = e.value
		}
	}
	if offset == 0 || length == 0 || uint64(offset)+uint64(length) > uint64(len(tiff)) {
		return nil
	}
	return bytes.Clone(tiff[offset : offset+length])
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// header returns the byte order of the TIFF data, and the offset of IFD0,
// or false when it doesn't start with a TIFF header
func header(tiff []byte) (binary.ByteOrder, uint32, bool) {
	if len(tiff) < 8 {
		return nil, 0, false
	}
	switch string(tiff[:2]) {
	case "II":
		return binary.LittleEndian, binary.LittleEndian.Uint32(tiff[4:]), true
	case "MM":
		return binary.BigEndian, binary.BigEndian.Uint32(tiff[4:]), true
	}
	return nil, 0, false
}

// readIFD returns the entries of the IFD at the offset of the TIFF data, and
// the offset of the next IFD, or nothing when it's outside the data
func readIFD(tiff []byte, order binary.ByteOrder, offset uint32) ([]entry, uint32) {
	start := uint64(offset)
	if start == 0 || start+2 > uint64(len(tiff)) {
		return nil, 0
	}
	count := uint64(order.Uint16(tiff[start:]))
	end := start + 2 + count*ifdEntrySize
	if end+4 > uint64(len(tiff)) {
		return nil, 0
	}
	entries := make([]entry, count)
	for i := range entries {
		data := tiff[start+2+uint64(i)*ifdEntrySize:]
		entries[i].tag = order.Uint16(data)
		switch order.Uint16(data[2:]) {
		case formatShort:
			entries[i].value, entries[i].ok = uint32(order.Uint16(data[8:])), true
		case formatLong:
			entries[i].value, entries[i].ok = order.Uint32(data[8:]), true
		}
	}
	return entries, order.Uint32(tiff[end:])
}
