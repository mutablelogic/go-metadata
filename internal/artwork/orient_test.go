package artwork_test

import (
	"image"
	"image/color"
	"testing"

	// Packages
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
)

func Test_orient_000(t *testing.T) {
	// An image of 3x2 pixels, whose pixels are numbered from the top left,
	// is transformed by each of the orientations of EXIF
	//
	//   0 1 2
	//   3 4 5
	src := image.NewGray(image.Rect(0, 0, 3, 2))
	for i := range 6 {
		src.SetGray(i%3, i/3, color.Gray{Y: uint8(i)})
	}
	for _, test := range []struct {
		orientation int
		want        [][]uint8
	}{
		{1, [][]uint8{{0, 1, 2}, {3, 4, 5}}},
		{2, [][]uint8{{2, 1, 0}, {5, 4, 3}}},
		{3, [][]uint8{{5, 4, 3}, {2, 1, 0}}},
		{4, [][]uint8{{3, 4, 5}, {0, 1, 2}}},
		{5, [][]uint8{{0, 3}, {1, 4}, {2, 5}}},
		{6, [][]uint8{{3, 0}, {4, 1}, {5, 2}}},
		{7, [][]uint8{{5, 2}, {4, 1}, {3, 0}}},
		{8, [][]uint8{{2, 5}, {1, 4}, {0, 3}}},
		{0, [][]uint8{{0, 1, 2}, {3, 4, 5}}},
		{9, [][]uint8{{0, 1, 2}, {3, 4, 5}}},
	} {
		got, ok := artwork.Orient(src, test.orientation).(*image.Gray)
		if !ok {
			t.Fatalf("%d: expected a grey image", test.orientation)
		} else if got.Bounds().Dx() != len(test.want[0]) || got.Bounds().Dy() != len(test.want) {
			t.Fatalf("%d: expected %dx%d, got %v", test.orientation, len(test.want[0]), len(test.want), got.Bounds())
		}
		for y, row := range test.want {
			for x, want := range row {
				if v := got.GrayAt(got.Bounds().Min.X+x, got.Bounds().Min.Y+y).Y; v != want {
					t.Errorf("%d: at %d,%d expected %d, got %d", test.orientation, x, y, want, v)
				}
			}
		}
	}

	// A colour image is RGBA, and an image which isn't at the origin is
	// transformed from its bounds
	rgba := image.NewRGBA(image.Rect(10, 10, 13, 12))
	rgba.Set(10, 10, color.RGBA{R: 255, A: 255})
	got := artwork.Orient(rgba, 6)
	if got.Bounds() != image.Rect(0, 0, 2, 3) {
		t.Fatalf("expected 2x3, got %v", got.Bounds())
	} else if r, _, _, _ := got.At(1, 0).RGBA(); r != 0xffff {
		t.Errorf("expected the red pixel at the top right, got %v", got.At(1, 0))
	}
}
