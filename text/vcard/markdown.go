package vcard

import (
	"context"
	"io"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	vcf "github.com/mutablelogic/go-metadata/pkg/vcard"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The contacts as Markdown, only when the markdown namespace is
	// requested
	metadata.AddHandler(reContentType, "vcard-markdown", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		cards, err := vcf.Decode(io.LimitReader(r, maxSize))
		if err != nil {
			return nil, err
		}
		w := imarkdown.NewWriter(o)
		return w.Metadata(writeMarkdown(ctx, w, cards))
	}, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// writeMarkdown writes each contact as Markdown: its name as a heading, its
// notes as paragraphs, and its details as a list, such as "**Email:**
// ada@example.com (work)". It stops with the writer's error, once it has
// its part.
func writeMarkdown(ctx context.Context, w io.StringWriter, cards []*vcf.Card) error {
	for i, c := range cards {
		if err := ctx.Err(); err != nil {
			return err
		}
		var b strings.Builder
		if i > 0 {
			b.WriteString("\n")
		}
		name := c.DisplayName()
		if name == "" {
			name = "Contact"
		}
		b.WriteString("## " + itext.Escape(name) + "\n\n")

		var items []string
		item := func(label, value string, types []string) {
			if value = strings.TrimSpace(value); value == "" {
				return
			}
			line := "- **" + label + ":** " + itext.Escape(value)
			if len(types) > 0 {
				line += " (" + itext.Escape(strings.Join(types, ", ")) + ")"
			}
			items = append(items, line)
		}
		for _, nickname := range c.Nicknames {
			item("Nickname", nickname, nil)
		}
		for i, org := range c.Orgs {
			title := ""
			if i < len(c.Titles) {
				title = c.Titles[i]
			}
			if title != "" {
				item("Organisation", title+", "+org.String(), nil)
			} else {
				item("Organisation", org.String(), nil)
			}
		}
		for _, title := range c.Titles[min(len(c.Titles), len(c.Orgs)):] {
			item("Title", title, nil)
		}
		for _, email := range c.Emails {
			item("Email", email.Value, email.Types)
		}
		for _, phone := range c.Phones {
			item("Phone", phone.Value, phone.Types)
		}
		for _, url := range c.URLs {
			item("URL", url.Value, url.Types)
		}
		for _, address := range c.Addresses {
			item("Address", address.String(), address.Types)
		}
		if c.Birthday != nil {
			item("Birthday", date(*c.Birthday), nil)
		}
		if len(c.Categories) > 0 {
			item("Categories", strings.Join(c.Categories, ", "), nil)
		}
		if len(items) > 0 {
			b.WriteString(strings.Join(items, "\n") + "\n")
		}
		for _, note := range c.Notes {
			b.WriteString("\n")
			for _, line := range strings.Split(strings.TrimSpace(note), "\n") {
				b.WriteString(itext.Escape(line) + "  \n")
			}
		}
		if _, err := w.WriteString(b.String()); err != nil {
			return err
		}
	}
	return nil
}
