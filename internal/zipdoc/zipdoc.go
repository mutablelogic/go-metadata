// Package zipdoc reads documents which are zip archives, such as office
// documents and EPUB books: the archive, the XML files in it, and the
// images in it, such as a thumbnail or a cover
package zipdoc

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// MaxXMLSize is the largest XML file within a document which is
	// decompressed, so a small archive can't decompress to fill memory
	MaxXMLSize = 1 << 20

	// MaxImageSize is the largest image within a document which is read, in
	// bytes
	MaxImageSize = 16 << 20
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Open returns the document's zip archive, which is read in place when the
// reader can be read at any offset, and has a size, as Extract's readers
// do, so only the files in it which are read are, and otherwise from
// memory, unless it's larger than maxSize, when it's false
func Open(r io.Reader, maxSize int64) (*zip.Reader, bool, error) {
	if ra, ok := r.(interface {
		io.ReaderAt
		Size() int64
	}); ok {
		archive, err := zip.NewReader(ra, ra.Size())
		return archive, err == nil, err
	}
	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return nil, false, err
	} else if int64(len(data)) > maxSize {
		return nil, false, nil
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	return archive, err == nil, err
}

// ReadXML decodes the XML file in the archive with the name into v, which is
// decompressed up to MaxXMLSize, or returns ErrNotFound when there's no
// such file
func ReadXML(archive *zip.Reader, name string, v any) error {
	f, err := archive.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return metadata.ErrNotFound.Withf("%s not found", name)
	} else if err != nil {
		return err
	}
	defer f.Close()
	if err := xml.NewDecoder(io.LimitReader(f, MaxXMLSize)).Decode(v); err != nil {
		return metadata.ErrBadParameter.Withf("%s: %v", name, err)
	}
	return nil
}

// Image returns the image in the archive with the name as artwork, with the
// key, downscaled to fit the largest image dimension when it's larger, or
// nil when there's none, or it can't be decoded, such as a Windows
// metafile, which some documents' thumbnails are
func Image(archive *zip.Reader, name, key string, o *metadata.Opts) metadata.Metadata {
	f, err := archive.Open(name)
	if err != nil {
		return nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxImageSize+1))
	if err != nil || len(data) > MaxImageSize {
		return nil
	}

	return artwork.Image(data, key, o)
}
