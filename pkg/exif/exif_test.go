//go:build cgo

package exif_test

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"testing"

	exif "github.com/mutablelogic/go-metadata/pkg/exif"
)

const (
	testJPEG          = "../../testfiles/sample.jpg"
	testJPEGMakerNote = "../../testfiles/canon_makernote_variant_1.jpg"
)

////////////////////////////////////////////////////////////////////////////////
// OPEN / PARSE / READ

func Test_exif_000(t *testing.T) {
	e, err := exif.Open(testJPEG)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	t.Log(e)
}

func Test_exif_001(t *testing.T) {
	// Non-existent file should return an error.
	_, err := exif.Open("/does/not/exist.jpg")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	t.Log(err)
}

func Test_exif_002(t *testing.T) {
	data, err := os.ReadFile(testJPEG)
	if err != nil {
		t.Fatal(err)
	}
	e, err := exif.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	t.Log(e)
}

func Test_exif_003(t *testing.T) {
	f, err := os.Open(testJPEG)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	e, err := exif.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	t.Log(e)
}

////////////////////////////////////////////////////////////////////////////////
// TAGS

func Test_exif_010(t *testing.T) {
	e, err := exif.Open(testJPEG)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	tags := e.Tags()
	if len(tags) == 0 {
		t.Fatal("expected at least one tag")
	}
	for _, tag := range tags {
		t.Logf("IFD=%d Tag=0x%04X Name=%q Format=%v Components=%d Value=%q Decoded=%v",
			tag.IFD(), tag.Tag(), tag.Name(), tag.Format(), tag.Components(), tag.Value(), tag.Any())
	}
}

func Test_exif_011(t *testing.T) {
	e, err := exif.Open(testJPEG)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	for _, tag := range e.Tags() {
		if tag.Name() == "" {
			t.Errorf("tag 0x%04X has empty name", tag.Tag())
		}
		if tag.String() == "" {
			t.Errorf("tag 0x%04X has empty string value", tag.Tag())
		}
	}
}

////////////////////////////////////////////////////////////////////////////////
// JSON

func Test_exif_015(t *testing.T) {
	e, err := exif.Open(testJPEG)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	for _, tag := range e.Tags() {
		data, err := json.Marshal(tag)
		if err != nil {
			t.Fatalf("tag 0x%04X: %v", tag.Tag(), err)
		}
		t.Log(string(data))
	}
}

////////////////////////////////////////////////////////////////////////////////
// MAKERNOTE

func Test_exif_020(t *testing.T) {
	e, err := exif.Open(testJPEG)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if mn := e.MakerNote(); mn != nil {
		t.Fatal("expected no makernote in sample.jpg")
	}
}

func Test_exif_021(t *testing.T) {
	e, err := exif.Open(testJPEGMakerNote)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	mn := e.MakerNote()
	if mn == nil {
		t.Fatal("expected makernote")
	}
	data, err := json.Marshal(mn)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(data))
}

func Test_exif_022(t *testing.T) {
	e, err := exif.Open(testJPEGMakerNote)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(data))
}

////////////////////////////////////////////////////////////////////////////////
// THUMBNAIL

func Test_exif_030(t *testing.T) {
	// The thumbnail embedded in the EXIF data of a photo is a JPEG image
	e, err := exif.Open("../../testfiles/PXL_20250102_172041079.MP.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	data := e.Thumbnail()
	if data == nil {
		t.Fatal("expected a thumbnail")
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	} else if format != "jpeg" || img.Bounds().Dx() != 283 || img.Bounds().Dy() != 504 {
		t.Errorf("expected a 283x504 jpeg, got a %dx%d %s", img.Bounds().Dx(), img.Bounds().Dy(), format)
	}
}

func Test_exif_031(t *testing.T) {
	// A photo without a thumbnail has none, and a thumbnail which is cut
	// short, as the image data has been removed, isn't checked
	e, err := exif.Open(testJPEG)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if data := e.Thumbnail(); data != nil {
		t.Errorf("expected no thumbnail, got %d bytes", len(data))
	}
	files, err := filepath.Glob("../../testfiles/*_makernote_*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		e, err := exif.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d bytes", filepath.Base(path), len(e.Thumbnail()))
		e.Close()
	}
}
