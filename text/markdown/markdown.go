// Package markdown registers handlers of Markdown documents, which return
// their title, and the metadata in their front matter, and their content
package markdown

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"regexp"
	"slices"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	tokenizer "github.com/mutablelogic/go-tokenizer"
	ast "github.com/mutablelogic/go-tokenizer/pkg/ast"
	md "github.com/mutablelogic/go-tokenizer/pkg/markdown"
	yaml "go.yaml.in/yaml/v3"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata in the front matter of a
	// document which isn't Dublin Core, which is the namespace of its
	// content too
	Namespace = metadata.MarkdownNamespace
)

var (
	reContentType = regexp.MustCompile(`^text/(?:markdown|x-markdown)$`)
)

// dcFrontMatter maps the names of fields of the front matter to Dublin Core
// element names. A name listed earlier for a key is preferred.
var dcFrontMatter = []struct {
	name, key string
}{
	{"title", dc.Title},
	{"description", dc.Description},
	{"summary", dc.Description},
	{"author", dc.Creator},
	{"authors", dc.Creator},
	{"date", dc.Date},
	{"tags", dc.Subject},
	{"keywords", dc.Subject},
	{"categories", dc.Subject},
	{"lang", dc.Language},
	{"language", dc.Language},
}

// reservedKeys are the keys of the markdown namespace which aren't fields of
// the front matter, but the content of the document, and how to read it a
// part at a time, so a field with one of these names has reservedPrefix
var reservedKeys = map[string]bool{
	"body":   true,
	"offset": true,
	"next":   true,
	"note":   true,
}

const reservedPrefix = "frontmatter-"

// errFound stops walking the document when the title is found
var errFound = errors.New("found")

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// Markdown is detected as plain text, and refined by its extension
	for _, ext := range []string{".md", ".markdown"} {
		if err := mime.AddExtensionType(ext, "text/markdown"); err != nil {
			panic(err)
		}
	}

	// The title, and the metadata in the front matter
	metadata.AddHandler(reContentType, "markdown", func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindDocument)
		text, err := itext.Read(r)
		if err != nil {
			return kind, err
		}
		fields, body, err := frontMatter(text)
		entries := markdownMetadata(fields, body)
		return append(slices.Collect(maps.Values(entries)), kind...), err
	}, dc.Namespace, Namespace, metadata.MetadataNamespace)

	// The content of the document, without its front matter, only when the
	// markdown namespace is requested
	metadata.AddHandler(reContentType, "markdown-body", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		text, err := itext.Read(r)
		if err != nil {
			return nil, err
		}
		_, body, _ := frontMatter(text)
		w := imarkdown.NewWriter(o)
		_, err = w.WriteString(body)
		return w.Metadata(err)
	}, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// markdownMetadata returns the metadata of the document, by key: the fields
// of its front matter, as Dublin Core where there is an element for them,
// and otherwise in the markdown namespace, with a field whose name is a key of
// the content prefixed with frontmatter-, and its title, which is the
// title in its front matter, or else its first level one heading, or else
// its first heading
func markdownMetadata(fields map[string]any, body string) map[string]metadata.Metadata {
	entries := make(map[string]metadata.Metadata)
	add := func(ns, key string, value any) {
		k := ns + string(metadata.Separator) + key
		if _, exists := entries[k]; !exists {
			if entry := meta.New(ns, key, value); entry != nil {
				entries[k] = entry
			}
		}
	}

	// The fields of the front matter, as Dublin Core, and the rest, in order
	// of name, so the first of the names with the same key is kept
	mapped := make(map[string]bool)
	for _, m := range dcFrontMatter {
		if value, exists := fields[m.name]; exists {
			mapped[m.name] = true
			if value = dcValue(value); value != "" {
				add(dc.Namespace, m.key, value)
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		if !mapped[name] {
			key := fieldKey(name)
			if reservedKeys[key] {
				key = reservedPrefix + key
			}
			add(Namespace, key, fields[name])
		}
	}

	// The title from the headings, when there's none in the front matter
	if title := headingTitle(body); title != "" {
		add(dc.Namespace, dc.Title, title)
	}
	return entries
}

// frontMatter returns the fields of the YAML front matter of the document,
// which is between lines of --- at its start, by their lowercase names, and
// the rest of the document. A document without front matter has no fields.
func frontMatter(text string) (map[string]any, string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, text, nil
	}

	// The front matter ends with a line of --- or ...
	rest := text[len("---\n"):]
	var yamlText, body string
	found := false
	for offset := 0; offset <= len(rest); {
		end := strings.IndexByte(rest[offset:], '\n')
		line := rest[offset:]
		if end >= 0 {
			line = rest[offset : offset+end]
		}
		if line == "---" || line == "..." {
			yamlText, found = rest[:offset], true
			if end >= 0 {
				body = rest[offset+end+1:]
			}
			break
		}
		if end < 0 {
			break
		}
		offset += end + 1
	}
	if !found {
		return nil, text, nil
	}

	// Parse the fields, by their lowercase names
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlText), &doc); err != nil {
		return nil, body, fmt.Errorf("front matter: %w", err)
	}
	value, err := nodeValue(&doc)
	if err != nil {
		return nil, body, fmt.Errorf("front matter: %w", err)
	}
	fields, _ := value.(map[string]any)
	result := make(map[string]any, len(fields))
	for name, value := range fields {
		result[strings.ToLower(strings.TrimSpace(name))] = value
	}
	return result, body, nil
}

// nodeValue returns the value of a YAML node: a mapping as a map, by the
// text of its keys, a sequence as a list, a timestamp, such as a date, as
// the text it's written as, rather than a time, and anything else as YAML
// decodes it
func nodeValue(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil, nil
		}
		return nodeValue(node.Content[0])
	case yaml.AliasNode:
		return nodeValue(node.Alias)
	case yaml.MappingNode:
		result := make(map[string]any, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			value, err := nodeValue(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			result[node.Content[i].Value] = value
		}
		return result, nil
	case yaml.SequenceNode:
		result := make([]any, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := nodeValue(item)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case yaml.ScalarNode:
		if node.ShortTag() == "!!timestamp" {
			return node.Value, nil
		}
	}
	var value any
	err := node.Decode(&value)
	return value, err
}

// dcValue returns the value of a field as the value of a Dublin Core
// element: a list as its items, separated by commas, and anything else as a
// string
func dcValue(value any) any {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(fmt.Sprint(dcValue(item))); s != "" {
				items = append(items, s)
			}
		}
		return strings.Join(items, ", ")
	default:
		return fmt.Sprint(v)
	}
}

// fieldKey returns the name of a field as a key, replacing each character
// which can't be in a key with a hyphen
func fieldKey(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, name)
}

// headingTitle returns the text of the first level one heading of the
// document, or else its first heading, without its markup, or an empty
// string when it has no heading
func headingTitle(body string) string {
	doc := md.Parse(strings.NewReader(body), tokenizer.Pos{})

	// Find the first level one heading, and the first heading
	var first, title *md.Heading
	ast.Walk(doc, func(node ast.Node, _ int) error {
		if heading, ok := node.(*md.Heading); ok {
			if first == nil {
				first = heading
			}
			if heading.Level() == 1 {
				title = heading
				return errFound
			}
		}
		return nil
	})
	if title == nil {
		title = first
	}
	if title == nil {
		return ""
	}
	return plainText(title)
}

// plainText returns the text and code within a node, without its markup,
// with its whitespace collapsed
func plainText(node ast.Node) string {
	var parts []string
	ast.Walk(node, func(node ast.Node, _ int) error {
		switch n := node.(type) {
		case *md.Text:
			parts = append(parts, n.Value())
		case *md.Code:
			parts = append(parts, n.Value())
		}
		return nil
	})
	return strings.Join(strings.Fields(strings.Join(parts, "")), " ")
}
