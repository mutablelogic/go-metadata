package vcard

import (
	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ContentType is the content type of a vCard
const ContentType = "text/vcard"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// A vCard is text, which is refined by its extension, so it's detected
	// the same on every platform, whatever its MIME types are
	for _, ext := range []string{".vcf", ".vcard"} {
		if err := metadata.AddTextExtensionType(ext, ContentType); err != nil {
			panic(err)
		}
	}
}
