package exif

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The namespaces of the tags of EXIF metadata, which are those of their
// XMP schemas: tiff for the tags of the image, from IFD0 and IFD1, and exif
// for those of the photo, its GPS position, and interoperability. This file
// doesn't need cgo, so they're defined without it.
const (
	TIFFNamespace = "tiff"
	EXIFNamespace = "exif"
)
