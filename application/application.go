// Package application registers the handlers of application documents, such
// as PDF documents, office documents, EPUB and Kindle books, archives, and
// Google documents, so importing it anonymously registers them all
package application

import (
	// Handlers
	_ "github.com/mutablelogic/go-metadata/application/archive"
	_ "github.com/mutablelogic/go-metadata/application/epub"
	_ "github.com/mutablelogic/go-metadata/application/google"
	_ "github.com/mutablelogic/go-metadata/application/mobi"
	_ "github.com/mutablelogic/go-metadata/application/msoffice"
	_ "github.com/mutablelogic/go-metadata/application/office"
	_ "github.com/mutablelogic/go-metadata/application/pdf"
)
