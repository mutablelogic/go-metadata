package html

import (
	"errors"
	"io"
	"strings"

	// Packages
	itext "github.com/mutablelogic/go-metadata/internal/text"
	html "golang.org/x/net/html"
	charset "golang.org/x/net/html/charset"
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// parse returns the document, decoded from its own character set, from its
// byte order mark, or its meta element, or else UTF-8 when it's valid UTF-8,
// and Windows-1252 otherwise. The content type isn't used, as the charset
// sniffed with it is always UTF-8. A document larger than itext.MaxBytes is
// parsed up to that size, which is usually enough for its head.
func parse(r io.Reader) (*html.Node, error) {
	// An empty document is read as one, rather than an error
	cr, err := charset.NewReader(io.LimitReader(r, itext.MaxBytes), "")
	if errors.Is(err, io.EOF) {
		cr = strings.NewReader("")
	} else if err != nil {
		return nil, err
	}
	return html.Parse(cr)
}

// walkHTML calls the function for each element of the document, in order,
// except those within scripts, styles and noscript elements
func walkHTML(node *html.Node, fn func(*html.Node)) {
	if node.Type == html.ElementNode {
		switch strings.ToLower(node.Data) {
		case "script", "style", "noscript":
			return
		}
		fn(node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, fn)
	}
}

// attr returns the value of an attribute of a node, with its spaces
// trimmed, or an empty string when it has no such attribute
func attr(node *html.Node, key string) string {
	for _, a := range node.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

// textContent returns the text within a node, with its whitespace collapsed.
// The parser has already unescaped the text.
func textContent(node *html.Node) string {
	var builder strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			builder.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

// metaField returns the lowercase name and the content of a meta element,
// where its name is its name, property or http-equiv attribute, or an empty
// name when it has no name or content. An itemprop attribute isn't a name,
// as it only names a property in the context of its microdata item.
func metaField(node *html.Node) (string, string) {
	attrs := make(map[string]string, len(node.Attr))
	for _, attr := range node.Attr {
		if key := strings.ToLower(strings.TrimSpace(attr.Key)); key != "" {
			attrs[key] = strings.TrimSpace(attr.Val)
		}
	}

	var name string
	for _, attr := range []string{"name", "property", "http-equiv"} {
		if name = attrs[attr]; name != "" {
			break
		}
	}
	content := strings.Join(strings.Fields(attrs["content"]), " ")
	if name == "" || content == "" {
		return "", ""
	}
	return strings.ToLower(name), content
}
