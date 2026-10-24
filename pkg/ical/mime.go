package ical

import (
	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ContentType is the content type of an iCalendar
const ContentType = "text/calendar"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// An iCalendar is text, which is refined by its extension, so it's
	// detected the same on every platform, whatever its MIME types are
	for _, ext := range []string{".ics", ".ical", ".ifb"} {
		if err := metadata.AddTextExtensionType(ext, ContentType); err != nil {
			panic(err)
		}
	}
}
