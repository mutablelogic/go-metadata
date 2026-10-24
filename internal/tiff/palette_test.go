package tiff_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	// Packages
	tiff "github.com/mutablelogic/go-metadata/internal/tiff"
	xtiff "golang.org/x/image/tiff"
)

// paletteAlphaTIFF returns a big-endian TIFF image, two pixels wide and one
// high, whose palette has red at index 1 and blue at index 2, with an alpha
// channel, as Illustrator writes the preview of an EPS file, with its tags
// in the order Illustrator writes them, which isn't sorted
func paletteAlphaTIFF() []byte {
	order := binary.BigEndian
	const ifd, colorMap, pixels = 8, 200, 1800
	data := make([]byte, pixels+4)
	copy(data, "MM\x00*")
	order.PutUint32(data[4:], ifd)

	// The entries: tag, type, count, and value, in Illustrator's order, in
	// which PlanarConfig is before StripOffsets
	entries := [][4]uint32{
		{256, 3, 1, 2}, {257, 3, 1, 1}, {258, 3, 1, 8}, {259, 3, 1, 1},
		{262, 3, 1, 3}, {277, 3, 1, 2}, {284, 3, 1, 1},
		{273, 4, 1, pixels}, {279, 4, 1, 4}, {320, 3, 768, colorMap},
	}
	order.PutUint16(data[ifd:], uint16(len(entries)))
	for i, e := range entries {
		entry := data[ifd+2+i*12:]
		order.PutUint16(entry, uint16(e[0]))
		order.PutUint16(entry[2:], uint16(e[1]))
		order.PutUint32(entry[4:], e[2])
		if e[1] == 3 && e[2] == 1 {
			order.PutUint16(entry[8:], uint16(e[3]))
		} else {
			order.PutUint32(entry[8:], e[3])
		}
	}

	// The colour map: reds, then greens, then blues, 16 bits each
	order.PutUint16(data[colorMap+1*2:], 0xFFFF)
	order.PutUint16(data[colorMap+(512+2)*2:], 0xFFFF)

	// The pixels: red, opaque, and blue, half transparent
	copy(data[pixels:], []byte{1, 255, 2, 128})
	return data
}

func Test_palette_000(t *testing.T) {
	// A palette image with an alpha channel, whose tags aren't sorted
	img := tiff.PaletteAlpha(paletteAlphaTIFF())
	if img == nil {
		t.Fatal("expected an image")
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 2, 1) {
		t.Errorf("bounds %v", got)
	}
	if got := color.NRGBAModel.Convert(img.At(0, 0)); got != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("first pixel %v", got)
	}
	if got := color.NRGBAModel.Convert(img.At(1, 0)); got != (color.NRGBA{0, 0, 255, 128}) {
		t.Errorf("second pixel %v", got)
	}
}

func Test_palette_001(t *testing.T) {
	// Another image, or data which isn't TIFF, isn't one
	var buf bytes.Buffer
	if err := xtiff.Encode(&buf, image.NewGray(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{buf.Bytes(), []byte("not a TIFF image"), nil} {
		if img := tiff.PaletteAlpha(data); img != nil {
			t.Errorf("expected no image, got %v", img.Bounds())
		}
	}
}

func Test_sort_000(t *testing.T) {
	// Tags which aren't sorted can't be decoded, until they're sorted
	data := paletteAlphaTIFF()
	if _, err := xtiff.DecodeConfig(bytes.NewReader(data)); err == nil {
		t.Fatal("expected an error for tags which aren't sorted")
	}
	sorted := tiff.SortTags(data)
	if _, err := xtiff.DecodeConfig(bytes.NewReader(sorted)); err != nil {
		t.Errorf("sorted: %v", err)
	}
	if bytes.Equal(data, sorted) {
		t.Error("expected a copy with the tags sorted")
	} else if tiff.PaletteAlpha(sorted) == nil {
		t.Error("expected the sorted image to be the same image")
	}
	if got := tiff.SortTags([]byte("not TIFF")); string(got) != "not TIFF" {
		t.Errorf("got %q", got)
	}
}
