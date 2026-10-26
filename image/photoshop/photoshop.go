// Package photoshop registers handlers of Photoshop documents, PSD and PSB
// files, which return their channels, depth and color mode, their XMP and
// IPTC metadata, their EXIF metadata when built with cgo, and their
// embedded thumbnail as artwork. The kind, format and size of a document of
// an image/ content type, and the composite image as artwork when there's
// no thumbnail, are from the raster images' handler, as importing this
// package registers Photoshop documents with the image package, and they're
// from these handlers for an application/ content type, which that handler
// doesn't read.
package photoshop

import (
	"context"
	"io"
	"mime"
	"regexp"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	exif "github.com/mutablelogic/go-metadata/pkg/exif"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
	iptc "github.com/mutablelogic/go-metadata/pkg/iptc"
	xmp "github.com/mutablelogic/go-metadata/pkg/xmp"
	psd "github.com/oov/psd"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the properties of a Photoshop document,
	// such as its color mode
	Namespace = "psd"

	// XMPNamespace is the namespace of XMP metadata, whose properties are in
	// the namespaces of their prefixes, such as dc or photoshop
	XMPNamespace = "xmp"

	// contentType is the content type of a Photoshop document
	contentType = "image/vnd.adobe.photoshop"

	// The image resources of a Photoshop document: its IPTC, EXIF and XMP
	// metadata, and its thumbnail, which is a header and a JPEG image
	resourceIPTC      = 0x0404
	resourceEXIF      = 0x0422
	resourceXMP       = 0x0424
	resourceThumbnail = 0x040C

	// imageNamespace is the namespace of the format and size of an image,
	// which the raster images' handler returns for an image/ content type
	imageNamespace = "image"
)

var (
	reContentType = regexp.MustCompile(`^(?:image|application)/(?:vnd\.adobe\.photoshop|x-photoshop|photoshop)$`)

	// exifMetadata returns the EXIF metadata of a document, from its EXIF
	// resource, which is set when it's built with cgo, as it uses libexif
	exifMetadata func(data []byte) []metadata.Metadata

	// colorModes are the names of the color modes of a Photoshop document
	colorModes = map[psd.ColorMode]string{
		0: "Bitmap",
		1: "Grayscale",
		2: "Indexed",
		3: "RGB",
		4: "CMYK",
		7: "Multichannel",
		8: "Duotone",
		9: "Lab",
	}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	for _, ext := range []string{".psd", ".psb"} {
		if err := mime.AddExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	// A document starts with 8BPS, whatever its name, both PSD and PSB
	if err := metadata.AddSignature([]byte("8BPS"), contentType); err != nil {
		panic(err)
	}

	metadata.AddHandler(reContentType, "photoshop", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		// The kind of a document of an application/ content type, which the
		// raster images' handler doesn't read
		var result []metadata.Metadata
		if !isImage(o.ContentType()) {
			result = metadata.KindMetadata(dc.KindImage)
		}

		// A document which can't be read has no more metadata, which isn't
		// an error, as another handler may read it
		cfg, _, err := psd.DecodeConfig(r)
		if err != nil {
			return result, nil
		}
		if !isImage(o.ContentType()) {
			result = append(result,
				meta.New(imageNamespace, "format", format(cfg)),
				meta.New(imageNamespace, "width", cfg.Rect.Dx()),
				meta.New(imageNamespace, "height", cfg.Rect.Dy()),
			)
		}

		// Hints of its artwork, when it has a thumbnail, or it's a document
		// of an application/ content type, whose composite image is its
		// artwork, and of its place, when it has a GPS position
		result = append(result, documentMetadata(cfg)...)
		hints := positionHints(result)
		if res, exists := cfg.Res[resourceThumbnail]; (exists && len(res.Data) > thumbnailHeaderSize) || !isImage(o.ContentType()) {
			hints = append(hints, metadata.HintArtwork)
		}
		return append(result, metadata.HintMetadata(hints...)...), nil
	}, Namespace, XMPNamespace, iptc.Namespace, exif.TIFFNamespace, exif.EXIFNamespace, dc.Namespace, imageNamespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// positionHints returns the hint of a document's place, metadata.HintGeo,
// when its metadata has a position, such as the GPS position of its EXIF or
// XMP metadata, or none
func positionHints(meta []metadata.Metadata) []string {
	for _, m := range meta {
		if ll, ok := m.Any().(geo.LatLong); ok && ll.Validate() == nil {
			return []string{metadata.HintGeo}
		} else if m.Key() == "GPSLatitude" && strings.TrimSpace(m.Value()) != "" {
			return []string{metadata.HintGeo}
		}
	}
	return nil
}

// isImage returns true when the content type is an image/ content type,
// which the raster images' handler reads
func isImage(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

// format returns the format of a document, as the image package names it,
// which is psb for a large document, and otherwise psd
func format(cfg psd.Config) string {
	if cfg.PSB() {
		return "psb"
	}
	return "psd"
}

// documentMetadata returns the channels, depth and color mode of a
// Photoshop document, its IPTC metadata (see iptc.Metadata), its EXIF
// metadata, when it's built with cgo, and the properties of its XMP
// metadata (see xmp.XMP.Metadata)
func documentMetadata(cfg psd.Config) []metadata.Metadata {
	result := []metadata.Metadata{
		meta.New(Namespace, "channels", cfg.Channels),
		meta.New(Namespace, "depth", cfg.Depth),
	}
	if mode, exists := colorModes[cfg.ColorMode]; exists {
		result = append(result, meta.New(Namespace, "color_mode", mode))
	}
	if res, exists := cfg.Res[resourceIPTC]; exists && len(res.Data) > 0 {
		result = append(result, iptc.Metadata(iptc.Parse(res.Data))...)
	}
	if res, exists := cfg.Res[resourceEXIF]; exists && len(res.Data) > 0 && exifMetadata != nil {
		result = append(result, exifMetadata(res.Data)...)
	}
	if res, exists := cfg.Res[resourceXMP]; exists && len(res.Data) > 0 {
		if doc, err := xmp.Parse(res.Data); err == nil {
			result = append(result, doc.Metadata()...)
		}
	}
	return result
}
