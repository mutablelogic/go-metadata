// Package title knows the titles which applications give documents which
// aren't their titles, such as the name of the file a document was made
// from, so a document's title is from its own name instead
package title

import (
	"slices"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// untitledPrefixes are the starts of titles which applications set from
	// the name of the file the document was made from, rather than a title
	untitledPrefixes = []string{"Microsoft Word - ", "Microsoft PowerPoint - ", "Microsoft Excel - "}

	// untitledTitles are the titles which applications set for a document
	// without one
	untitledTitles = []string{"PowerPoint Presentation", "PowerPoint-Präsentation", "Présentation PowerPoint"}
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Untitled returns true when the title is empty, the name of the file the
// document was made from, such as "Microsoft Word - Document1.docx", or the
// title an application sets for a document without one, such as
// "PowerPoint Presentation"
func Untitled(title string) bool {
	title = strings.TrimSpace(title)
	if title == "" || slices.Contains(untitledTitles, title) {
		return true
	}
	for _, prefix := range untitledPrefixes {
		if strings.HasPrefix(title, prefix) {
			return true
		}
	}
	return false
}
