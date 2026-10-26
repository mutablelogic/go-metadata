//go:build cgo

// Package heif registers handlers of HEIF images, such as HEIC photos from
// an iPhone, and AVIF images, which return their EXIF metadata, as the JPEG
// handler does, their XMP metadata, and their embedded thumbnail as
// artwork, which uses libheif, so needs cgo. Their format and size are from
// the raster images' handler, as importing this package registers HEIF
// images with the image package.
package heif

import (
	"context"
	"io"
	"regexp"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imageexif "github.com/mutablelogic/go-metadata/image/exif"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	exif "github.com/mutablelogic/go-metadata/pkg/exif"
	heif "github.com/mutablelogic/go-metadata/pkg/heif"
	xmp "github.com/mutablelogic/go-metadata/pkg/xmp"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata blocks of a HEIF image which
	// aren't EXIF or XMP data
	Namespace = heif.Namespace

	// XMPNamespace is the namespace of XMP metadata, whose properties are in
	// the namespaces of their prefixes, such as dc or photoshop
	XMPNamespace = "xmp"
)

var (
	reContentType = regexp.MustCompile(`^image/(?:heic|heif|avif)(?:-sequence)?$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The metadata of a HEIF image, and hints of its artwork, when it has a
	// thumbnail, and its place, when it has a GPS position
	metadata.AddHandler(reContentType, "heif", func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		h, err := heif.Read(r)
		if err != nil {
			// An image which can't be read has none, as another handler may
			// read it
			return nil, nil
		}
		defer h.Close()
		result := heifMetadata(h.Metadata())
		hints := imageexif.PositionHints(result)
		if len(h.Thumbnails()) > 0 {
			hints = append(hints, metadata.HintArtwork)
		}
		return append(result, metadata.HintMetadata(hints...)...), nil
	}, imageexif.TIFFNamespace, imageexif.EXIFNamespace, dc.Namespace, Namespace, XMPNamespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// heifMetadata returns the metadata of the blocks of a HEIF image: its EXIF
// tags as the JPEG handler returns them (see exif.TagsMetadata), its XMP
// properties, as one document (see xmp.XMP.Metadata), and
// its other blocks as their values, without those whose namespace or key
// isn't an identifier
func heifMetadata(blocks []metadata.Metadata) []metadata.Metadata {
	var tags []*exif.Tag
	var result []metadata.Metadata
	doc := xmp.New()
	for _, m := range blocks {
		if tag, ok := m.(*exif.Tag); ok {
			tags = append(tags, tag)
		} else if item, ok := m.(*xmp.Item); ok {
			doc.Add(item)
		} else if entry := meta.New(m.NS(), m.Key(), m.Any()); entry != nil {
			result = append(result, entry)
		}
	}
	return append(append(imageexif.TagsMetadata(tags), doc.Metadata()...), result...)
}
