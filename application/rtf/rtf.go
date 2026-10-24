// Package rtf registers a handler of RTF documents, which returns their
// kind, a document, the properties in their info group, such as their
// title, author and when they were created, and their text as Markdown
package rtf

import (
	"context"
	"io"
	"regexp"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	title "github.com/mutablelogic/go-metadata/internal/title"
	rtf "github.com/mutablelogic/go-metadata/pkg/rtf"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the properties of a document which
	// aren't Dublin Core
	Namespace = "rtf"

	// maxSize is the most of a document which is read, which can be large
	// with its pictures
	maxSize = 64 << 20
)

var reContentType = regexp.MustCompile(`^(?:application|text)/rtf$`)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// A document starts with {\rtf, whatever its name, such as one saved as
	// .doc
	if err := metadata.AddSignature([]byte(`{\rtf`), "application/rtf"); err != nil {
		panic(err)
	}

	// The kind of an RTF document, its properties, and its text as
	// Markdown, only when the markdown namespace is requested
	metadata.AddHandler(reContentType, "rtf", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindDocument)
		data, err := io.ReadAll(io.LimitReader(r, maxSize))
		if err != nil {
			return kind, err
		}
		doc := rtf.Parse(data)
		result := append(infoMetadata(doc.Info), kind...)
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return result, nil
		}
		w := imarkdown.NewWriter(o)
		markdown, err := w.Metadata(writeMarkdown(w, doc))
		return append(result, markdown...), err
	}, Namespace, dc.Namespace, metadata.MetadataNamespace, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// infoMetadata returns the properties of a document which are set, other
// than a title which is from the name of a file, such as "Microsoft Word -
// report.doc", which are named as the properties of a Word document are
// (see the msoffice package)
func infoMetadata(info rtf.Info) []metadata.Metadata {
	var result []metadata.Metadata
	add := func(ns, key string, value any) {
		switch v := value.(type) {
		case string:
			if v == "" {
				return
			}
		case time.Time:
			if v.IsZero() {
				return
			}
		case int:
			if v <= 0 {
				return
			}
		}
		if entry := meta.New(ns, key, value); entry != nil {
			result = append(result, entry)
		}
	}
	if !title.Untitled(info.Title) {
		add(dc.Namespace, dc.Title, info.Title)
	}
	add(dc.Namespace, dc.Subject, info.Subject)
	add(dc.Namespace, dc.Creator, info.Author)
	add(dc.Namespace, dc.Description, info.Comment)
	add(dc.Namespace, dc.Date, info.Created)
	add(Namespace, "keywords", info.Keywords)
	add(Namespace, "last-author", info.Operator)
	add(Namespace, "modified", info.Revised)
	add(Namespace, "application", info.Generator)
	add(Namespace, "pages", info.Pages)
	add(Namespace, "words", info.Words)
	add(Namespace, "characters", info.Characters)
	add(Namespace, "company", info.Company)
	add(Namespace, "manager", info.Manager)
	add(Namespace, "category", info.Category)
	return result
}

// writeMarkdown writes the paragraphs of the document's text as Markdown,
// with their characters which are Markdown escaped, and a cell of a table
// separated from the next by a space. It stops with the writer's error, once
// it has its part.
func writeMarkdown(w io.StringWriter, doc *rtf.Document) error {
	for i, paragraph := range doc.Paragraphs {
		text := strings.TrimSpace(itext.Escape(paragraph)) + "\n"
		if i > 0 {
			text = "\n" + text
		}
		if _, err := w.WriteString(text); err != nil {
			return err
		}
	}
	return nil
}
