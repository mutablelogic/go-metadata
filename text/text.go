// Package text registers the handlers of text documents, such as HTML,
// Markdown, plain text and source code, so importing it anonymously
// registers them all
package text

import (
	// Handlers
	_ "github.com/mutablelogic/go-metadata/text/html"
	_ "github.com/mutablelogic/go-metadata/text/markdown"
	_ "github.com/mutablelogic/go-metadata/text/plain"
	_ "github.com/mutablelogic/go-metadata/text/sourcecode"
)
