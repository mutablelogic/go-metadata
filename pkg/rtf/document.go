// Package rtf reads RTF documents: the text of their paragraphs, and the
// properties in their info group, such as their title, author, and when
// they were created
package rtf

import (
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Document is an RTF document: the text of its paragraphs, and the
// properties in its info group
type Document struct {
	// The paragraphs of the document's text, which aren't blank, in which a
	// cell of a table is followed by a tab, and a line break is a newline
	Paragraphs []string

	// The properties in the document's info group
	Info Info
}

// Info is the properties in an RTF document's info group, each of which is
// empty, or zero, when it isn't set. Its times have no time zone, and are in
// UTC.
type Info struct {
	Title    string // The document's title
	Subject  string // Its subject
	Author   string // Who wrote it
	Operator string // Who last changed it
	Keywords string // Its keywords
	Comment  string // A comment on it
	Company  string // The author's company
	Manager  string // The author's manager
	Category string // Its category

	// The application which made it, such as "Riched20 5.40.11.2210", or
	// Cocoa, for macOS's text system, such as TextEdit's
	Generator string

	Created time.Time // When it was created
	Revised time.Time // When it was last changed
	Printed time.Time // When it was last printed

	Pages      int // How many pages it had when it was last saved
	Words      int // How many words
	Characters int // How many characters
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ContentType is the content type of RTF documents
const ContentType = "application/rtf"
