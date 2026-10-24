// Package vcard registers handlers of vCards, which are contacts, such as a
// person in an address book, which return the details of the first contact,
// its photo as artwork, and the contacts as Markdown
package vcard

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	vcf "github.com/mutablelogic/go-metadata/pkg/vcard"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the details of a contact, such as its
	// email addresses
	Namespace = "vcard"

	// maxSize is the most of a vCard file which is read, which includes an
	// embedded photo of several megabytes, as base64
	maxSize = 16 << 20
)

var (
	reContentType = regexp.MustCompile(`^text/(?:vcard|x-vcard|directory)$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// A vCard is text, and refined by its extension
	for _, ext := range []string{".vcf", ".vcard"} {
		if err := metadata.AddTextExtensionType(ext, "text/vcard"); err != nil {
			panic(err)
		}
	}

	// The details of the first contact, and its kind
	metadata.AddHandler(reContentType, "vcard", func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindContact)
		cards, err := vcf.Decode(io.LimitReader(r, maxSize))
		if err != nil {
			return kind, err
		} else if len(cards) == 0 {
			return kind, nil
		}
		result := cardMetadata(cards[0])
		if len(cards) > 1 {
			result = append(result, meta.New(Namespace, "count", len(cards)))
		}
		return append(result, kind...), nil
	}, Namespace, dc.Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// cardMetadata returns the details of a contact: its formatted name, or the
// parts of its name, as dc:title, its
// first note as dc:description, or else its job title and organisation,
// such as "Translator at Analytical Engines", and its UID as dc:identifier;
// and its email addresses, phone numbers, URLs, nicknames and addresses as
// lists, and its first organisation, job title, and its birthday, as a date
// such as 1815-12-10, or --12-10 without a year, in the vcard namespace
func cardMetadata(c *vcf.Card) []metadata.Metadata {
	var result []metadata.Metadata
	add := func(ns, key string, value any) {
		if s, ok := value.(string); ok && strings.TrimSpace(s) == "" {
			return
		}
		if entry := meta.New(ns, key, value); entry != nil {
			result = append(result, entry)
		}
	}
	list := func(key string, values []string) {
		if values = nonEmpty(values); len(values) > 0 {
			result = append(result, meta.NewText(Namespace, key, values, strings.Join(values, "; ")))
		}
	}

	// The name, description and identifier. The title is the formatted
	// name, or the parts of the name, and not an email address, so a contact
	// without a name has the title of its file's name
	if name := strings.TrimSpace(c.FN); name != "" {
		add(dc.Namespace, dc.Title, name)
	} else {
		add(dc.Namespace, dc.Title, c.Name.String())
	}
	org, title := "", ""
	if len(c.Orgs) > 0 {
		org = c.Orgs[0].String()
	}
	if len(c.Titles) > 0 {
		title = c.Titles[0]
	}
	switch {
	case len(c.Notes) > 0:
		add(dc.Namespace, dc.Description, c.Notes[0])
	case title != "" && org != "":
		add(dc.Namespace, dc.Description, title+" at "+org)
	default:
		add(dc.Namespace, dc.Description, title+org)
	}
	add(dc.Namespace, dc.Identifier, c.UID)

	// The details of the contact
	list("email", values(c.Emails))
	list("tel", values(c.Phones))
	list("url", values(c.URLs))
	list("nickname", c.Nicknames)
	addresses := make([]string, len(c.Addresses))
	for i, address := range c.Addresses {
		addresses[i] = address.String()
	}
	list("adr", addresses)
	add(Namespace, "org", org)
	add(Namespace, "title", title)
	if c.Birthday != nil {
		add(Namespace, "bday", date(*c.Birthday))
	}
	return result
}

// values returns the values of email addresses, phone numbers or URLs
func values(values []vcf.Value) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = v.Value
	}
	return result
}

// date returns a date as ISO 8601, such as 1815-12-10, or --12-10 without a
// year
func date(d vcf.Date) string {
	if d.Year > 0 {
		return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
	}
	return fmt.Sprintf("--%02d-%02d", d.Month, d.Day)
}

// nonEmpty returns the values which aren't empty, without their spaces
func nonEmpty(values []string) []string {
	var result []string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
