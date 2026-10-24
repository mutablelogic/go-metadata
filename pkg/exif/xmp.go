package exif

import (
	// Packages
	xmp "github.com/mutablelogic/go-metadata/pkg/xmp"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// xmpValueTypes are the types of the properties of the exif and tiff schemas
// of XMP, which are the EXIF tags of an image, as XMP properties, in
// "prefix:name" form. This file doesn't need cgo, so the types are
// registered when the package is imported without it.
var xmpValueTypes = map[string]xmp.ValueType{
	// Dates and times
	"tiff:DateTime":          xmp.ValueTypeTime,
	"exif:DateTimeOriginal":  xmp.ValueTypeTime,
	"exif:DateTimeDigitized": xmp.ValueTypeTime,
	"exif:GPSTimeStamp":      xmp.ValueTypeTime,

	// Booleans, of which exif:Fired is a field of the exif:Flash struct
	"exif:Fired": xmp.ValueTypeBoolean,

	// Rationals
	"tiff:XResolution":            xmp.ValueTypeRational,
	"tiff:YResolution":            xmp.ValueTypeRational,
	"exif:ExposureTime":           xmp.ValueTypeRational,
	"exif:FNumber":                xmp.ValueTypeRational,
	"exif:CompressedBitsPerPixel": xmp.ValueTypeRational,
	"exif:ShutterSpeedValue":      xmp.ValueTypeRational,
	"exif:ApertureValue":          xmp.ValueTypeRational,
	"exif:BrightnessValue":        xmp.ValueTypeRational,
	"exif:ExposureBiasValue":      xmp.ValueTypeRational,
	"exif:MaxApertureValue":       xmp.ValueTypeRational,
	"exif:SubjectDistance":        xmp.ValueTypeRational,
	"exif:FocalLength":            xmp.ValueTypeRational,
	"exif:DigitalZoomRatio":       xmp.ValueTypeRational,
	"exif:GPSAltitude":            xmp.ValueTypeRational,

	// GPS coordinates
	"exif:GPSLatitude":      xmp.ValueTypeGPSCoord,
	"exif:GPSLongitude":     xmp.ValueTypeGPSCoord,
	"exif:GPSDestLatitude":  xmp.ValueTypeGPSCoord,
	"exif:GPSDestLongitude": xmp.ValueTypeGPSCoord,
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// Register the types of the properties of the exif and tiff schemas
	for key, typ := range xmpValueTypes {
		xmp.RegisterValueType(key, typ)
	}
}
