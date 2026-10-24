// Package iwork reads documents of Apple's iWork applications, Pages,
// Keynote and Numbers, which are zip archives: the properties of a document
// from iWork '09, such as its title and authors, and the template and
// version of a document from iWork 2013 and later, and their previews
package iwork

import (
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Document is what's known of an iWork document, each of which is empty,
// or zero, when it isn't known
type Document struct {
	// The document's properties, which a document from iWork '09 has
	Title, Comment, Copyright   string
	Authors, Keywords, Projects []string

	// The template the document was made from, and when, and the version
	// of the application which last saved it, such as 7.1, which a
	// document from iWork 2013 or later has
	Template string
	Created  time.Time
	Version  string

	// The document's preview, which is a JPEG image, or nil
	Preview []byte
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The content types of documents of Pages, Keynote and Numbers
const (
	ContentTypePages   = "application/vnd.apple.pages"
	ContentTypeKeynote = "application/vnd.apple.keynote"
	ContentTypeNumbers = "application/vnd.apple.numbers"
)
