package mbox

import (
	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// A mailbox is text, which is refined by its extension, such as
	// Eudora's .mbx, so it's detected the same on every platform, whatever
	// its MIME types are
	for _, ext := range []string{".mbox", ".mbx"} {
		if err := metadata.AddTextExtensionType(ext, ContentType); err != nil {
			panic(err)
		}
	}
}
