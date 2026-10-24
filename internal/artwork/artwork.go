// Package artwork downscales and encodes images which represent documents,
// such as the scans of their pages, their thumbnails, and their covers
package artwork

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	tiff "github.com/mutablelogic/go-metadata/internal/tiff"
	draw "golang.org/x/image/draw"

	// Image decoders
	_ "image/gif"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// artworkQuality is the quality of an image, as a JPEG image, when it's
	// in colour
	artworkQuality = 85

	// maxGreyChroma is the most a pixel's red, green and blue can differ,
	// out of 65535, for it to be grey, which allows for the tint of paper
	maxGreyChroma = 24 << 8

	// maxColourPixels is the fraction of the pixels of a grey image which
	// can have a colour
	maxColourPixels = 0.01
)

// webFormats are the formats of images which are displayed on the web, so
// artwork in them is kept as it is, when it's small enough. A GIF image is
// re-encoded, so an animated one is its first frame.
var webFormats = map[string]bool{"jpeg": true, "png": true, "webp": true}

// exifFormats are the formats of images whose orientation is read from
// their EXIF data, as it isn't applied when they're decoded
var exifFormats = map[string]bool{"jpeg": true, "tiff": true}

// losslessFormats are the formats of images which are re-encoded as PNG
// images, which are lossless too, and keep their transparency
var losslessFormats = map[string]bool{"png": true, "gif": true, "bmp": true, "tiff": true}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Image returns the encoded image as artwork, with the key, downscaled to
// fit the largest image dimension when it's larger, or re-encoded when it's
// in a format which isn't displayed on the web, such as BMP or TIFF, or it
// has an orientation in its EXIF data which isn't as it is, as its EXIF data
// isn't kept, and otherwise as it is, or nil when it can't be decoded (see
// OrientedImage).
func Image(data []byte, key string, o *metadata.Opts) metadata.Metadata {
	return orientedImage(data, key, 0, o)
}

// OrientedImage returns the encoded image as artwork, with the key, as Image
// does, transformed by the orientation, from 1 to 8, as EXIF defines it (see
// Orient), such as the orientation of a photo for its thumbnail, which has
// no EXIF data of its own. An image in a lossless format, such as PNG, is
// re-encoded as a PNG image, a JPEG image as a JPEG image, and any other as
// a greyscale PNG image when it's grey, or else a JPEG image.
func OrientedImage(data []byte, key string, orientation int, o *metadata.Opts) metadata.Metadata {
	return orientedImage(data, key, max(1, orientation), o)
}

// orientedImage returns the encoded image as artwork, transformed by the
// orientation, or by the orientation in its EXIF data when it's zero, which
// is only read from a JPEG or TIFF image, as the decoder of another format,
// such as HEIF, transforms an image itself
func orientedImage(data []byte, key string, orientation int, o *metadata.Opts) metadata.Metadata {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil
	} else if orientation == 0 {
		orientation = 1
		if exifFormats[format] {
			orientation = tiff.Orientation(data)
		}
	}
	if max(config.Width, config.Height) > o.MaxImageDimension() || !webFormats[format] || (orientation >= 2 && orientation <= 8) {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil
		}

		// Downscale before the image is transformed, which is quicker, as
		// its largest dimension is the same either way
		img = Orient(Downscale(img, o.MaxImageDimension()), orientation)
		switch {
		case losslessFormats[format]:
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				return nil
			}
			data = buf.Bytes()
		case format == "jpeg":
			// A JPEG image stays a JPEG image, as it's lossy already
			data = encodeJPEG(img)
		default:
			data = Encode(img)
		}
		if data == nil {
			return nil
		}
	}
	return meta.NewImage(metadata.ArtworkNamespace, key, data)
}

// Downscale returns the image scaled down so its width and height are at
// most size, keeping its aspect ratio, or the image itself when it's
// smaller
func Downscale(img image.Image, size int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= size && h <= size {
		return img
	}
	if w >= h {
		w, h = size, max(1, h*size/w)
	} else {
		w, h = max(1, w*size/h), size
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
	return dst
}

// Encode returns the image encoded as a greyscale PNG image when it's grey,
// such as a scan of a black and white document, which is smaller and sharper
// than a JPEG image, or else as a JPEG image
func Encode(img image.Image) []byte {
	var buf bytes.Buffer
	if isGrey(img) {
		bounds := img.Bounds()
		grey := image.NewGray(bounds)
		draw.Draw(grey, bounds, img, bounds.Min, draw.Src)
		if err := png.Encode(&buf, grey); err != nil {
			return nil
		}
	} else {
		return encodeJPEG(img)
	}
	return buf.Bytes()
}

// encodeJPEG returns the image encoded as a JPEG image, or nil when it can't
// be encoded
func encodeJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: artworkQuality}); err != nil {
		return nil
	}
	return buf.Bytes()
}

// isGrey returns true when the image is grey: its colour model is grey, or
// at most maxColourPixels of its pixels have a colour, rather than a grey,
// as a scan of a black and white document is often a colour image, with the
// paper's tint, and a few pixels of colour at the edges of the text
func isGrey(img image.Image) bool {
	switch img.ColorModel() {
	case color.GrayModel, color.Gray16Model:
		return true
	}
	bounds := img.Bounds()
	colour, limit := 0, int(float64(bounds.Dx()*bounds.Dy())*maxColourPixels)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if max(r, g, b)-min(r, g, b) > maxGreyChroma {
				if colour++; colour > limit {
					return false
				}
			}
		}
	}
	return true
}
