package eml

import (
	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// An email message is text, which is refined by its extension, so it's
	// detected the same on every platform, whatever its MIME types are
	if err := metadata.AddTextExtensionType(".eml", ContentType); err != nil {
		panic(err)
	}
}
