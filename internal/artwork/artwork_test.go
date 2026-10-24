package artwork

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"testing"
)

func Test_artwork_001(t *testing.T) {
	// An image is downscaled to fit the size, keeping its aspect ratio, and
	// a smaller image isn't changed
	for _, test := range []struct {
		w, h, size   int
		wantW, wantH int
	}{
		{1240, 1754, 640, 452, 640},
		{1754, 1240, 640, 640, 452},
		{400, 300, 640, 400, 300},
		{5000, 2, 640, 640, 1},
	} {
		img := Downscale(image.NewRGBA(image.Rect(0, 0, test.w, test.h)), test.size)
		if b := img.Bounds(); b.Dx() != test.wantW || b.Dy() != test.wantH {
			t.Errorf("%dx%d: expected %dx%d, got %dx%d", test.w, test.h, test.wantW, test.wantH, b.Dx(), b.Dy())
		}
	}
}

func Test_artwork_002(t *testing.T) {
	// A grey image, such as a scan of a black and white document on tinted
	// paper, with a few pixels of colour, is a PNG image, and an image in
	// colour is a JPEG image
	paper := color.RGBA{250, 247, 238, 255}
	for _, test := range []struct {
		name   string
		img    image.Image
		colour int
		want   string
	}{
		{"grey model", image.NewGray(image.Rect(0, 0, 100, 100)), 0, "image/png"},
		{"tinted paper", image.NewRGBA(image.Rect(0, 0, 100, 100)), 0, "image/png"},
		{"a few colour pixels", image.NewRGBA(image.Rect(0, 0, 100, 100)), 50, "image/png"},
		{"colour", image.NewRGBA(image.Rect(0, 0, 100, 100)), 2500, "image/jpeg"},
	} {
		if rgba, ok := test.img.(*image.RGBA); ok {
			for i := 0; i < 100*100; i++ {
				c := paper
				if i < test.colour {
					c = color.RGBA{30, 90, 200, 255}
				}
				rgba.Set(i%100, i/100, c)
			}
		}
		data := Encode(test.img)
		if got := http.DetectContentType(data); got != test.want {
			t.Errorf("%s: expected %s, got %s", test.name, test.want, got)
		}
		if test.want == "image/png" {
			if img, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Errorf("%s: %v", test.name, err)
			} else if _, ok := img.(*image.Gray); !ok {
				t.Errorf("%s: expected a greyscale image, got %T", test.name, img)
			}
		}
	}
}
