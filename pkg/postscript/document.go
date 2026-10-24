// Package postscript reads the comments of PostScript documents, and of
// Encapsulated PostScript (EPS) files, which follow Adobe's Document
// Structuring Conventions (DSC), such as their title, the application which
// made them, who they were made for, when, and their pages and bounding box,
// and the preview of an EPS file with a DOS EPS header
package postscript

import (
	"math"
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Document is what a PostScript document's comments say about it, each of
// which is empty, or zero, when it doesn't have it
type Document struct {
	// The version of the conventions it follows, such as 3.0, from its first
	// line, %!PS-Adobe-3.0, and the version of EPS, such as 3.0, when it's an
	// EPS file, from %!PS-Adobe-3.0 EPSF-3.0
	Version, EPS string

	// Its title, the application which made it, and who it was made for,
	// such as a person and their company
	Title, Creator, For string

	// When it was made, as it's written, such as "Fri Oct 24 07:32:48 1997",
	// and as a time, in UTC, when it can be read
	CreationDate string
	Created      time.Time

	// How many pages it has, from its %%Pages comment, or else its %%Page
	// comments, and its PostScript language level, such as 2
	Pages, LanguageLevel int

	// The size of paper it's for, as it says, such as A4, or empty (see
	// Paper)
	PaperSize string

	// The names of the fonts it uses, such as Times-Roman
	Fonts []string

	// The box which its marks are within, in points, a 72nd of an inch,
	// which is its high resolution bounding box, when it has one, or nil
	BoundingBox *BoundingBox

	// The preview of an EPS file with a DOS EPS header, which is a TIFF
	// image, or nil
	Preview []byte
}

// BoundingBox is a box on a page, by its lower left and upper right
// corners, in points
type BoundingBox struct {
	LLX, LLY, URX, URY float64
}

// paper is a size of paper, by its name, and its width and height, in
// points
type paper struct {
	name          string
	width, height float64
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ContentType is the content type of PostScript documents and EPS files
const ContentType = "application/postscript"

// paperTolerance is how far, in points, a page's size can be from a size of
// paper, for it to be that size
const paperTolerance = 2

// papers are the sizes of paper which documents are for
var papers = []paper{
	{"A3", 842, 1191}, {"A4", 595, 842}, {"A5", 420, 595}, {"B5", 499, 709},
	{"Letter", 612, 792}, {"Legal", 612, 1008}, {"Tabloid", 792, 1224},
	{"Executive", 522, 756},
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Paper returns the size of paper the document is for: the size it says,
// with the case of a known size, such as A4 for a4, or else the size its
// bounding box is, in either orientation, when it's a page, which starts at
// the corner of the page, such as dvips gives a document, or an empty string
// when it isn't known, as an EPS file's bounding box is its drawing's
func (doc *Document) Paper() string {
	if doc.PaperSize != "" {
		for _, p := range papers {
			if strings.EqualFold(p.name, doc.PaperSize) {
				return p.name
			}
		}
		return doc.PaperSize
	}
	box := doc.BoundingBox
	if box == nil || doc.EPS != "" || math.Abs(box.LLX) > paperTolerance || math.Abs(box.LLY) > paperTolerance {
		return ""
	}
	return paperOfSize(box.Width(), box.Height())
}

// Width returns the width of the box, in points
func (b BoundingBox) Width() float64 {
	return b.URX - b.LLX
}

// Height returns the height of the box, in points
func (b BoundingBox) Height() float64 {
	return b.URY - b.LLY
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// paperOfSize returns the name of the size of paper which is the width and
// height, in points, in either orientation, or an empty string when none is
func paperOfSize(width, height float64) string {
	near := func(a, b float64) bool { return math.Abs(a-b) <= paperTolerance }
	for _, p := range papers {
		if (near(width, p.width) && near(height, p.height)) || (near(width, p.height) && near(height, p.width)) {
			return p.name
		}
	}
	return ""
}
