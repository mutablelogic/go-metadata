// Package xml registers a handler of XML documents, which returns their
// kind, which is data, their root element and its namespace, their DOCTYPE,
// and their title, when they have one, such as an RSS feed's, and their
// content as Markdown, as an XML code block. XML documents which other
// handlers read, such as XHTML, aren't read.
package xml

import (
	"context"
	stdxml "encoding/xml"
	"io"
	"regexp"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	itext "github.com/mutablelogic/go-metadata/internal/text"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// document is what's read of an XML document: its root element, and its
// namespace, its DOCTYPE, and its title, each of which is empty when it
// doesn't have one
type document struct {
	root, namespace, doctype, title string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of an XML document
	Namespace = "xml"

	// maxTitleDepth is the deepest a title is looked for, such as an RSS
	// feed's, which is in its rss, channel and title elements
	maxTitleDepth = 3

	// maxTokens is how many of a document's tokens are read for its root
	// element, DOCTYPE and title, so a large document isn't read to its end
	maxTokens = 1000

	// maxParseSize is the most of the start of a document which is read for
	// its root element, DOCTYPE and title
	maxParseSize = 1 << 20
)

var (
	// reContentType matches the content types of XML documents, such as
	// text/xml, and of more specific ones, such as application/rss+xml
	reContentType = regexp.MustCompile(`^(?:text|application)/(?:[a-z0-9.-]+\+)?xml$`)

	// otherTypes are the content types of XML documents which other
	// handlers read
	otherTypes = map[string]bool{"application/xhtml+xml": true}

	// doctypeRe matches a DOCTYPE, its name, and its public identifier,
	// when it has one
	doctypeRe = regexp.MustCompile(`(?s)^DOCTYPE\s+([^\s\[>]+)(?:\s+PUBLIC\s+(?:"([^"]*)"|'([^']*)'))?`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The kind of an XML document, its root element, DOCTYPE and title, and
	// its content, as an XML code block, only when the markdown namespace is
	// requested
	metadata.AddHandler(reContentType, "xml", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if otherTypes[o.ContentType()] {
			return nil, nil
		}
		kind := metadata.KindMetadata(dc.KindData)
		if itext.HasText(r) {
			kind = append(kind, metadata.HintMetadata(metadata.HintMarkdown)...)
		}
		seeker, ok := r.(io.Seeker)
		if !ok {
			// The document is read in memory, up to itext.MaxBytes
			text, err := itext.Read(r)
			if err != nil {
				return kind, err
			}
			result := append(parse(text).metadata(), kind...)
			if !o.HasNamespace(metadata.MarkdownNamespace) {
				return result, nil
			}
			w := imarkdown.NewWriter(o)
			_, err = w.WriteString(imarkdown.CodeBlock("xml", text))
			markdown, err := w.Metadata(err)
			return append(result, markdown...), err
		}
		start, err := seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			return kind, err
		}

		// The root element, DOCTYPE and title, which are at the start of the
		// document
		data, err := io.ReadAll(io.LimitReader(r, maxParseSize))
		if err != nil {
			return kind, err
		}
		text, err := itext.Decode(data, len(data) == maxParseSize)
		if err != nil {
			return kind, err
		}
		result := append(parse(text).metadata(), kind...)
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return result, nil
		}

		// The document, as it's read, from its start
		if _, err := seeker.Seek(start, io.SeekStart); err != nil {
			return result, err
		}
		w := imarkdown.NewWriter(o)
		markdown, err := w.Metadata(itext.WriteCodeBlock(w, "xml", r))
		return append(result, markdown...), err
	}, Namespace, dc.Namespace, metadata.MetadataNamespace, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// parse returns the root element of the XML document, and its namespace,
// its DOCTYPE, and its title, which is the text of the first title element,
// in any namespace, such as Dublin Core's, within maxTitleDepth of its root,
// from its first maxTokens. A document which isn't well-formed is read up to
// its first error. The text is UTF-8, so its declared encoding, which it
// was decoded from, isn't used.
func parse(text string) document {
	var doc document
	d := stdxml.NewDecoder(strings.NewReader(text))
	d.Strict = false
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) {
		return r, nil
	}

	var title strings.Builder
	depth, titleDepth := 0, 0
	for range maxTokens {
		token, err := d.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case stdxml.Directive:
			if doc.root == "" {
				doc.doctype = doctype(string(t))
			}
		case stdxml.StartElement:
			depth++
			switch {
			case depth == 1:
				doc.root, doc.namespace = t.Name.Local, t.Name.Space
			case titleDepth == 0 && depth <= maxTitleDepth && t.Name.Local == "title":
				titleDepth = depth
			}
		case stdxml.CharData:
			if titleDepth != 0 {
				title.Write(t)
			}
		case stdxml.EndElement:
			if titleDepth != 0 && depth == titleDepth {
				// An empty title isn't the title, and another is looked for
				if doc.title = strings.Join(strings.Fields(title.String()), " "); doc.title != "" {
					return doc
				}
				title.Reset()
				titleDepth = 0
			}
			depth--
		}
	}
	return doc
}

// doctype returns the public identifier of the DOCTYPE, such as
// "-//Apple//DTD PLIST 1.0//EN", or its name, such as html, when it doesn't
// have one, or an empty string when the directive isn't a DOCTYPE
func doctype(directive string) string {
	m := doctypeRe.FindStringSubmatch(strings.TrimSpace(directive))
	switch {
	case m == nil:
		return ""
	case m[2] != "" || m[3] != "":
		return m[2] + m[3]
	default:
		return m[1]
	}
}

// metadata returns the metadata of the document: its root element, and its
// namespace, its DOCTYPE, and its title, as dc:title, when it has them
func (doc document) metadata() []metadata.Metadata {
	var result []metadata.Metadata
	add := func(ns, key, value string) {
		if value != "" {
			result = append(result, meta.New(ns, key, value))
		}
	}
	add(Namespace, "root", doc.root)
	add(Namespace, "namespace", doc.namespace)
	add(Namespace, "doctype", doc.doctype)
	add(dc.Namespace, dc.Title, doc.title)
	return result
}
