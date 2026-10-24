package artwork

import (
	"image"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Orient returns the image transformed to be displayed, by its orientation,
// from 1 to 8, as EXIF defines it: 1 is as it is, 2 is flipped horizontally,
// 3 is rotated 180°, 4 is flipped vertically, 5 is transposed, 6 is rotated
// 90° clockwise, 7 is transverse, and 8 is rotated 90° anticlockwise. It
// returns the image itself for 1, or an orientation which isn't valid. A
// grey image stays grey, and any other is RGBA.
func Orient(img image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return img
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// The size of the result, which is swapped when it's rotated by 90°
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}

	// The pixel of the image for each pixel of the result
	source := func(x, y int) (int, int) {
		switch orientation {
		case 2:
			return w - 1 - x, y
		case 3:
			return w - 1 - x, h - 1 - y
		case 4:
			return x, h - 1 - y
		case 5:
			return y, x
		case 6:
			return y, h - 1 - x
		case 7:
			return w - 1 - y, h - 1 - x
		default:
			return w - 1 - y, x
		}
	}

	// Copy the pixels, keeping a grey image grey
	if grey, ok := img.(*image.Gray); ok {
		dst := image.NewGray(image.Rect(0, 0, dw, dh))
		for y := 0; y < dh; y++ {
			for x := 0; x < dw; x++ {
				sx, sy := source(x, y)
				dst.SetGray(x, y, grey.GrayAt(bounds.Min.X+sx, bounds.Min.Y+sy))
			}
		}
		return dst
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx, sy := source(x, y)
			dst.Set(x, y, img.At(bounds.Min.X+sx, bounds.Min.Y+sy))
		}
	}
	return dst
}
