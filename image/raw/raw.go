//go:build cgo

// Package raw registers handlers of RAW images from cameras, such as Canon
// CR2, Nikon NEF and Olympus ORF images, which return their EXIF metadata,
// as the JPEG handler does, the camera and shooting parameters which
// libraw reads, their format and size, and their embedded preview as
// artwork, which use libraw and libexif, so need cgo
package raw

import (
	"context"
	"io"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imageexif "github.com/mutablelogic/go-metadata/image/exif"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	tiff "github.com/mutablelogic/go-metadata/internal/tiff"
	exif "github.com/mutablelogic/go-metadata/pkg/exif"
	raw "github.com/mutablelogic/go-metadata/pkg/raw"
	xmp "github.com/mutablelogic/go-metadata/pkg/xmp"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// imageNamespace is the namespace of the format and size of an image
	imageNamespace = "image"

	// priority is higher than the default, so the format and size of a RAW
	// image are these, rather than those of the raster images' handler,
	// which can read a TIFF-based RAW image, such as a DNG image, as the
	// small image which is its first
	priority = metadata.DefaultPriority + 10

	// exifHeader is before the TIFF data of EXIF metadata in a JPEG image,
	// which libexif reads it after
	exifHeader = "Exif\x00\x00"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	metadata.AddHandlerWithPriority(raw.ContentTypes, "raw", priority, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		img, err := raw.Parse(data)
		if err != nil {
			// An image which can't be read has none, as another handler may
			// read it
			return nil, nil
		}
		defer img.Close()
		return rawMetadata(img, data, o.ContentType()), nil
	}, imageNamespace, exif.TIFFNamespace, exif.EXIFNamespace, dc.Namespace, xmp.Namespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// rawMetadata returns the metadata of a RAW image: its EXIF metadata, from
// the image, when it's TIFF data, or else from its preview, as the JPEG
// handler returns it, the camera and shooting parameters which libraw
// reads, which aren't in its EXIF metadata, its XMP metadata, and its
// format and size, which is as it's displayed, so the width and height are
// swapped when it's rotated by 90°
func rawMetadata(img *raw.RAW, data []byte, contentType string) []metadata.Metadata {
	// The EXIF metadata of the image, or of its preview
	tags, orientation := exifTags(tiff.Normalize(data))
	if tags == nil {
		if preview, err := img.ThumbnailBytes(); err == nil {
			tags, _ = exifTags(preview)
		}
	}
	result := imageexif.TagsMetadata(tags)

	// The metadata which libraw reads, which isn't in the EXIF metadata,
	// with the date the photo was taken as dc:date when there's no other
	has := make(map[string]bool, len(result))
	for _, m := range result {
		has[key(m)] = true
	}
	for _, m := range img.Metadata() {
		if has[key(m)] {
			continue
		}
		result = append(result, m)
		has[key(m)] = true
		if t, ok := m.Any().(time.Time); ok && m.Key() == "DateTimeOriginal" && !has[dc.Namespace+":"+dc.Date] {
			result = append(result, meta.New(dc.Namespace, dc.Date, t))
			has[dc.Namespace+":"+dc.Date] = true
		}
	}

	// The XMP metadata of the image
	if data := img.XMP(); len(data) > 0 {
		if doc, err := xmp.Parse(data); err == nil {
			result = append(result, doc.Metadata()...)
		}
	}

	// The format and size of the image, as it's displayed
	width, height := img.Width(), img.Height()
	if orientation >= 5 {
		width, height = height, width
	}
	if format := raw.Format(contentType); format != "" {
		result = append(result, meta.New(imageNamespace, "format", format))
	}
	return append(result,
		meta.New(imageNamespace, "width", width),
		meta.New(imageNamespace, "height", height),
	)
}

// exifTags returns the EXIF tags of TIFF data, or of a JPEG image, and the
// orientation of the image, or nil when there are none
func exifTags(data []byte) ([]*exif.Tag, int) {
	if len(data) == 0 {
		return nil, 1
	}
	parse := data
	if tiff.Normalize(data) != nil {
		parse = append([]byte(exifHeader), data...)
	}
	f, err := exif.Parse(parse)
	if err != nil {
		return nil, 1
	}
	defer f.Close()
	return f.Tags(), f.Orientation()
}

// key returns the namespace and key of metadata, such as tiff:Make
func key(m metadata.Metadata) string {
	return m.NS() + string(metadata.Separator) + m.Key()
}
