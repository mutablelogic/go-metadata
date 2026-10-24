package photoshop_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/image/photoshop"
	_ "github.com/mutablelogic/go-metadata/image/raster"
)

// psdMeta returns the metadata of the document, by key
func psdMeta(t *testing.T, data []byte, opts ...metadata.Option) map[string]metadata.Metadata {
	t.Helper()
	return psdMetaOf(t, data, "image/vnd.adobe.photoshop", opts...)
}

// psdMetaOf returns the metadata of the document of the content type, by key
func psdMetaOf(t *testing.T, data []byte, contentType string, opts ...metadata.Option) map[string]metadata.Metadata {
	t.Helper()
	meta, err := getMetadata(context.Background(), bytes.NewReader(data), contentType, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]metadata.Metadata, len(meta))
	for _, m := range meta {
		result[key(m)] = m
	}
	return result
}

// sample returns the sample document, which has a thumbnail of 4x3 pixels,
// and is 8x6 pixels, with a red square in the top left
func sample(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(TEST_DIR, "sample.psd"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// withoutThumbnail returns the document without its thumbnail resource,
// which is the last of its image resources
func withoutThumbnail(t *testing.T, data []byte) []byte {
	t.Helper()
	const resourcesOffset = 26 + 4
	length := binary.BigEndian.Uint32(data[resourcesOffset:])
	resources := data[resourcesOffset+4 : resourcesOffset+4+int(length)]
	i := bytes.LastIndex(resources, []byte("8BIM\x04\x0c"))
	if i < 0 {
		t.Fatal("expected the thumbnail resource")
	}
	var out bytes.Buffer
	out.Write(data[:resourcesOffset])
	binary.Write(&out, binary.BigEndian, uint32(i))
	out.Write(resources[:i])
	out.Write(data[resourcesOffset+4+int(length):])
	return out.Bytes()
}

func Test_photoshop_000(t *testing.T) {
	// The channels, depth and color mode of a document, its XMP metadata,
	// and its format, size and kind, from the raster images' handler
	got := psdMeta(t, sample(t))
	for key, want := range map[string]string{
		"psd:channels":    "3",
		"psd:depth":       "8",
		"psd:color_mode":  "RGB",
		"dc:title":        "Sample Photoshop Document",
		"xmp:CreatorTool": "Adobe Photoshop 25.0",
		"image:format":    "psd",
		"image:width":     "8",
		"image:height":    "6",
		"metadata:kind":   "image",
		"dc:type":         "StillImage",
	} {
		if m, exists := got[key]; !exists {
			t.Errorf("expected %s", key)
		} else if m.Value() != want {
			t.Errorf("%s: expected %q, got %q", key, want, m.Value())
		}
	}
	if v, ok := got["xmp:CreateDate"].Any().(time.Time); !ok || !v.Equal(time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("xmp:CreateDate: expected a time, got %v", got["xmp:CreateDate"].Any())
	}
}

func Test_photoshop_001(t *testing.T) {
	// The artwork of a document is its thumbnail, or its composite image
	// when it has none
	for _, test := range []struct {
		name          string
		data          []byte
		width, height int
	}{
		{"thumbnail", sample(t), 4, 3},
		{"composite", withoutThumbnail(t, sample(t)), 8, 6},
	} {
		got := psdMeta(t, test.data, metadata.WithNamespace(metadata.ArtworkNamespace))
		if len(got) != 1 || got["artwork:thumbnail"] == nil {
			t.Fatalf("%s: expected artwork:thumbnail, got %v", test.name, got)
		}
		img, _, err := image.Decode(bytes.NewReader(got["artwork:thumbnail"].Bytes()))
		if err != nil {
			t.Fatal(err)
		} else if img.Bounds().Dx() != test.width || img.Bounds().Dy() != test.height {
			t.Errorf("%s: expected %dx%d, got %v", test.name, test.width, test.height, img.Bounds())
		}
	}
}

func Test_photoshop_002(t *testing.T) {
	// Data which isn't a document has no metadata from it, which isn't an
	// error, and a .psd file is a Photoshop document
	got := psdMeta(t, []byte("not a photoshop document"))
	for k := range got {
		if k != "metadata:kind" && k != "dc:type" {
			t.Errorf("expected no metadata, got %s", k)
		}
	}
	if contentType := contentTypeForFile(t, filepath.Join(TEST_DIR, "sample.psd")); contentType != "image/vnd.adobe.photoshop" {
		t.Errorf("expected image/vnd.adobe.photoshop, got %q", contentType)
	}
}

func Test_photoshop_003(t *testing.T) {
	// The IPTC metadata of a document
	got := psdMeta(t, sample(t))
	for key, want := range map[string]string{
		"iptc:ObjectName":      "Sample",
		"iptc:Keywords":        "sample; photoshop",
		"iptc:Byline":          "Ada Lovelace",
		"iptc:City":            "Zürich",
		"iptc:CountryName":     "Switzerland",
		"iptc:CopyrightNotice": "© 2024 Ada Lovelace",
		"iptc:DateCreated":     "2024-01-15T10:30:00+01:00",
	} {
		if m, exists := got[key]; !exists {
			t.Errorf("expected %s", key)
		} else if m.Value() != want {
			t.Errorf("%s: expected %q, got %q", key, want, m.Value())
		}
	}
}

func Test_photoshop_004(t *testing.T) {
	// A document of an application/ content type, which the raster images'
	// handler doesn't read, has its kind, format and size, and its composite
	// image as artwork when it has no thumbnail, from these handlers, once
	for _, contentType := range []string{"application/vnd.adobe.photoshop", "application/x-photoshop", "image/vnd.adobe.photoshop"} {
		meta, err := getMetadata(context.Background(), bytes.NewReader(sample(t)), contentType)
		if err != nil {
			t.Fatal(err)
		}
		count := map[string]int{}
		got := map[string]string{}
		for _, m := range meta {
			count[key(m)]++
			got[key(m)] = m.Value()
		}
		for key, want := range map[string]string{
			"metadata:kind": "image",
			"dc:type":       "StillImage",
			"image:format":  "psd",
			"image:width":   "8",
			"image:height":  "6",
			"psd:channels":  "3",
		} {
			if got[key] != want || count[key] != 1 {
				t.Errorf("%s: %s: expected %q once, got %q %d times", contentType, key, want, got[key], count[key])
			}
		}

		artwork := psdMetaOf(t, withoutThumbnail(t, sample(t)), contentType, metadata.WithNamespace(metadata.ArtworkNamespace))
		if m := artwork["artwork:thumbnail"]; m == nil {
			t.Errorf("%s: expected the composite image", contentType)
		} else if img, _, err := image.Decode(bytes.NewReader(m.Bytes())); err != nil || img.Bounds().Dx() != 8 {
			t.Errorf("%s: expected an image 8 pixels wide, got %v", contentType, err)
		}
	}
}

// named is a reader with a name, for its extension
type named struct {
	*bytes.Reader
	name string
}

func (r named) Name() string { return r.name }

func Test_photoshop_signature_000(t *testing.T) {
	// A document is a Photoshop document from its content, without a name,
	// without an extension, or with the wrong one
	data := sample(t)
	for _, r := range []interface {
		Read([]byte) (int, error)
	}{
		bytes.NewReader(data),
		named{bytes.NewReader(data), "Logo"},
		named{bytes.NewReader(data), "logo.bin"},
		named{bytes.NewReader(data), "logo.psb"},
	} {
		if contentType, _, err := metadata.ContentType(r); err != nil {
			t.Fatal(err)
		} else if contentType != "image/vnd.adobe.photoshop" {
			t.Errorf("%v: got %q", r, contentType)
		}
	}
}
