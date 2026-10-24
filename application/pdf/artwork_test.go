package pdf

import (
	"math"
	"testing"

	// Packages
	textpdf "github.com/giraffesyo/pdf"
)

func Test_artwork_000(t *testing.T) {
	// The coverage of the page by an image is the fraction of it which the
	// image's bounds overlap
	page := textpdf.Rect{MinX: 0, MinY: 0, MaxX: 595, MaxY: 842}
	for _, test := range []struct {
		bounds textpdf.Rect
		want   float64
	}{
		{textpdf.Rect{MinX: 0, MinY: 0, MaxX: 595, MaxY: 842}, 1},
		{textpdf.Rect{MinX: -10, MinY: -10, MaxX: 605, MaxY: 852}, 1},
		{textpdf.Rect{MinX: 0, MinY: 0, MaxX: 595, MaxY: 421}, 0.5},
		{textpdf.Rect{MinX: 600, MinY: 0, MaxX: 700, MaxY: 842}, 0},
		{textpdf.Rect{MinX: 10, MinY: 10, MaxX: 10, MaxY: 10}, 0},
	} {
		if got := coverage(test.bounds, page); math.Abs(got-test.want) > 1e-9 {
			t.Errorf("%v: expected %v, got %v", test.bounds, test.want, got)
		}
	}
	if got := coverage(page, textpdf.Rect{}); got != 0 {
		t.Errorf("an empty page: expected 0, got %v", got)
	}
}

func Test_artwork_003(t *testing.T) {
	// The size of an image of a page is its longest side at the dpi, and at
	// most the largest dimension, when there is one
	a4 := textpdf.Rect{MaxX: 595, MaxY: 842}
	letter := textpdf.Rect{MaxX: 612, MaxY: 792}
	a0 := textpdf.Rect{MaxX: 2384, MaxY: 3370}
	for _, test := range []struct {
		box               textpdf.Rect
		dpi, maxDimension int
		want              int
	}{
		{a4, 150, 4096, 1754},
		{a4, 300, 4096, 3508},
		{a4, 72, 4096, 842},
		{letter, 150, 4096, 1650},
		{a0, 150, 4096, 4096},
		{a4, 150, 1280, 1280},

		// No largest dimension, when the resolution is set
		{a4, 300, 0, 3508},
		{a0, 150, 0, 7021},
	} {
		if got := pageSize(test.box, test.dpi, test.maxDimension); got != test.want {
			t.Errorf("%v at %d dpi, at most %d: expected %d, got %d", test.box, test.dpi, test.maxDimension, test.want, got)
		}
	}
}
