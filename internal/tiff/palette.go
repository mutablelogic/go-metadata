package tiff

import (
	"encoding/binary"
	"image"
	"image/color"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// field is an entry of an IFD, with its type, its count of values, and its
// value, or the offset of its values when they're more than four bytes
type field struct {
	typ   uint16
	count uint32
	value []byte
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// The tags of a palette image with an alpha channel
	tagWidth           = 0x0100
	tagHeight          = 0x0101
	tagBitsPerSample   = 0x0102
	tagCompression     = 0x0103
	tagPhotometric     = 0x0106
	tagStripOffsets    = 0x0111
	tagSamplesPerPixel = 0x0115
	tagStripByteCounts = 0x0117
	tagPlanarConfig    = 0x011C
	tagColorMap        = 0x0140

	// photometricPalette is the photometric interpretation of a palette
	// image, whose samples are indexes into its colour map
	photometricPalette = 3
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// PaletteAlpha returns the image of TIFF data which is an uncompressed
// palette image, with an alpha channel, with eight bits a sample, as
// Illustrator writes the preview of an EPS file, which golang.org/x/image/tiff
// reads as an image with one sample a pixel, or nil when it isn't one. Its
// tags don't need to be sorted.
func PaletteAlpha(data []byte) image.Image {
	order, ifd0, ok := header(data)
	if !ok {
		return nil
	}
	fields := readFields(data, order, ifd0)
	first := func(tag uint16, otherwise uint32) uint32 {
		if values := fieldValues(data, order, fields[tag]); len(values) > 0 {
			return values[0]
		}
		return otherwise
	}

	// Check it's an uncompressed palette image, with an alpha channel, whose
	// samples are interleaved, with eight bits a sample
	width, height := int(first(tagWidth, 0)), int(first(tagHeight, 0))
	switch {
	case width <= 0 || height <= 0 || width > 1<<14 || height > 1<<14:
		return nil
	case first(tagCompression, 1) != 1, first(tagPhotometric, 0) != photometricPalette:
		return nil
	case first(tagSamplesPerPixel, 1) != 2, first(tagPlanarConfig, 1) != 1, first(tagBitsPerSample, 1) != 8:
		return nil
	}
	colorMap := fieldValues(data, order, fields[tagColorMap])
	if len(colorMap) != 3*256 {
		return nil
	}

	// The pixels, from the strips, in order
	offsets, counts := fieldValues(data, order, fields[tagStripOffsets]), fieldValues(data, order, fields[tagStripByteCounts])
	if len(offsets) == 0 || len(offsets) != len(counts) {
		return nil
	}
	pixels := make([]byte, 0, width*height*2)
	for i, offset := range offsets {
		end := uint64(offset) + uint64(counts[i])
		if end > uint64(len(data)) {
			return nil
		}
		pixels = append(pixels, data[offset:end]...)
	}
	if len(pixels) < width*height*2 {
		return nil
	}

	// The colours of the palette are 16 bits a channel, reds, then greens,
	// then blues
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		index, alpha := pixels[i*2], pixels[i*2+1]
		img.Set(i%width, i/width, color.NRGBA{
			R: uint8(colorMap[index] >> 8),
			G: uint8(colorMap[256+int(index)] >> 8),
			B: uint8(colorMap[512+int(index)] >> 8),
			A: alpha,
		})
	}
	return img
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// readFields returns the fields of the IFD at the offset, by their tags, or
// none when it's outside the data
func readFields(tiff []byte, order binary.ByteOrder, offset uint32) map[uint16]field {
	result := make(map[uint16]field)
	start := uint64(offset)
	if start == 0 || start+2 > uint64(len(tiff)) {
		return result
	}
	count := uint64(order.Uint16(tiff[start:]))
	if start+2+count*ifdEntrySize > uint64(len(tiff)) {
		return result
	}
	for i := uint64(0); i < count; i++ {
		entry := tiff[start+2+i*ifdEntrySize:][:ifdEntrySize]
		result[order.Uint16(entry)] = field{typ: order.Uint16(entry[2:]), count: order.Uint32(entry[4:]), value: entry[8:12]}
	}
	return result
}

// fieldValues returns the values of a field which are SHORTs or LONGs, from
// the field itself, or the offset of its values when they're more than four
// bytes, or none when they're another type, or outside the data
func fieldValues(tiff []byte, order binary.ByteOrder, f field) []uint32 {
	var size uint64
	switch f.typ {
	case formatShort:
		size = 2
	case formatLong:
		size = 4
	default:
		return nil
	}
	data := f.value
	if n := size * uint64(f.count); n > 4 {
		offset := uint64(order.Uint32(f.value))
		if offset+n > uint64(len(tiff)) {
			return nil
		}
		data = tiff[offset : offset+n]
	} else if f.count == 0 {
		return nil
	}
	values := make([]uint32, f.count)
	for i := range values {
		if size == 2 {
			values[i] = uint32(order.Uint16(data[uint64(i)*2:]))
		} else {
			values[i] = order.Uint32(data[uint64(i)*4:])
		}
	}
	return values
}
