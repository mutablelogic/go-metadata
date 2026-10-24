package pdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"

	// Packages
	textpdf "github.com/giraffesyo/pdf"
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	model "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// maxArtworkPages is the most pages which have artwork, as the image of
	// a scanned page is decoded and downscaled
	maxArtworkPages = 32

	// minScanSize is the smallest width and height of the image of a page,
	// in pixels, for the page to be a scan
	minScanSize = 500

	// minScanCoverage is the fraction of a page which its image covers, for
	// the page to be a scan
	minScanCoverage = 0.85
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// pdfArtwork returns images of the pages of a PDF document, as
// artwork:page0001, artwork:page0002 and so on, in the order of its pages:
// the thumbnail stored for a page, or else, when the page is a scan, which
// is one large image covering the page, that image downscaled to the
// resolution set via WithImageDPI, or else 150 dpi, and at most the
// dimension set via WithMaxImageDimension. Only the first maxArtworkPages
// pages have artwork. A page whose image can't be read
// has none.
func pdfArtwork(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	password := o.Password()
	rs, ok, err := readSeeker(r)
	if err != nil || !ok {
		return nil, err
	}
	doc, err := validate(ctx, rs, password)
	var encrypted encryptedError
	if errors.As(err, &encrypted) {
		return nil, encrypted.wrongPassword(password)
	} else if err != nil || doc == nil {
		return nil, err
	}
	pages := min(doc.PageCount, maxArtworkPages)

	// The thumbnails stored for the pages
	artwork := make(map[int][]byte, pages)
	for page := 1; page <= pages; page++ {
		if data := thumbnail(ctx, doc, page); data != nil {
			artwork[page] = data
		}
	}

	// The scans of the pages without thumbnails, whose images are read with
	// where they're placed on their pages, at the resolution, and at most
	// the largest dimension, unless the resolution is set
	if len(artwork) < pages {
		maxDimension := o.MaxImageDimension()
		if o.HasImageDPI() {
			maxDimension = 0
		}
		ra, size, ok, err := readerAt(rs)
		if err != nil || !ok {
			return nil, err
		}
		opts := textpdf.Options{
			Password:      password,
			IncludeImages: true,
			Pages:         []textpdf.PageRange{{First: 1, Last: pages}},
		}
		if _, err := textpdf.ExtractPages(ctx, ra, size, opts, func(page textpdf.Page) error {
			if _, exists := artwork[page.Number]; !exists {
				if data := scan(page, o.ImageDPI(), maxDimension); data != nil {
					artwork[page.Number] = data
				}
			}
			return ctx.Err()
		}); err != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// Return the artwork in the order of the pages
	var result []metadata.Metadata
	for _, page := range slices.Sorted(maps.Keys(artwork)) {
		if entry := meta.NewImage(metadata.ArtworkNamespace, fmt.Sprintf("page%04d", page), artwork[page]); entry != nil {
			result = append(result, entry)
		}
	}
	return result, nil
}

// thumbnail returns the encoded thumbnail stored for the page, or nil when
// there isn't one, or it can't be read
func thumbnail(ctx context.Context, doc *model.Context, page int) []byte {
	ref, exists := doc.PageThumbs[page]
	if !exists {
		return nil
	}
	sd, _, err := doc.DereferenceStreamDict(ref)
	if err != nil || sd == nil {
		return nil
	}
	img, err := pdfcpu.ExtractImage(ctx, doc, sd, true, "", ref.ObjectNumber.Value(), false)
	if err != nil || img == nil || img.Reader == nil {
		return nil
	}
	data, err := io.ReadAll(img.Reader)
	if err != nil {
		return nil
	}
	return data
}

// scan returns the image of the page, downscaled to the size of the page at
// the resolution, in dots per inch, and at most maxDimension, unless it's zero, and encoded as
// a PNG image when it's grey, or a JPEG image when it's in colour, when the
// page is a scan: one of its images, which is at least
// minScanSize pixels wide and high, covers at least minScanCoverage of it,
// which can be a mask, as a black and white scan often is. It returns nil
// when the page isn't a scan, or its image can't be decoded.
func scan(page textpdf.Page, dpi, maxDimension int) []byte {
	box := page.CropBox
	if area(box) <= 0 {
		box = page.MediaBox
	}
	var found *textpdf.Image
	for i, img := range page.Images {
		if min(img.Width, img.Height) < minScanSize || coverage(img.Bounds(), box) < minScanCoverage {
			continue
		} else if found != nil {
			return nil
		}
		found = &page.Images[i]
	}
	if found == nil {
		return nil
	}
	img, err := found.Decode()
	if err != nil {
		return nil
	}
	return artwork.Encode(artwork.Downscale(img, pageSize(box, dpi, maxDimension)))
}

// pageSize returns the largest width or height of an image of the page, in
// pixels, which is its longest side, in points, of 1/72 inch, at the
// resolution, in dots per inch, and at most maxDimension, unless it's zero
func pageSize(box textpdf.Rect, dpi, maxDimension int) int {
	longest := max(box.MaxX-box.MinX, box.MaxY-box.MinY)
	size := int(math.Round(longest / 72 * float64(dpi)))
	if maxDimension > 0 {
		size = min(size, maxDimension)
	}
	return max(1, size)
}

// coverage returns the fraction of the page which the bounds of an image
// cover
func coverage(bounds, page textpdf.Rect) float64 {
	pageArea := area(page)
	if pageArea <= 0 {
		return 0
	}
	overlap := textpdf.Rect{
		MinX: max(bounds.MinX, page.MinX),
		MinY: max(bounds.MinY, page.MinY),
		MaxX: min(bounds.MaxX, page.MaxX),
		MaxY: min(bounds.MaxY, page.MaxY),
	}
	return area(overlap) / pageArea
}

// area returns the area of the rectangle, or zero when it's empty
func area(r textpdf.Rect) float64 {
	return max(0, r.MaxX-r.MinX) * max(0, r.MaxY-r.MinY)
}
