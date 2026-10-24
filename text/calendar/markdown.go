package calendar

import (
	"cmp"
	"context"
	"io"
	"slices"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The calendar as Markdown, only when the markdown namespace is requested
	metadata.AddHandler(reContentType, "ical-markdown", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		c, err := ical.Decode(io.LimitReader(r, maxSize))
		if err != nil {
			return nil, err
		}
		w := imarkdown.NewWriter(o)
		return w.Metadata(writeMarkdown(ctx, w, c))
	}, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// writeMarkdown writes a calendar as Markdown: its name as a heading, and
// its description, and each of its events, in the order they start, as a
// heading of its title, and a list of when it is, in words, where, its
// organizer, attendees and categories, and then its description. It stops
// with the writer's error, once it has its part.
func writeMarkdown(ctx context.Context, w io.StringWriter, c *ical.Calendar) error {
	var b strings.Builder
	if name := strings.TrimSpace(c.Name); name != "" {
		b.WriteString("# " + itext.Escape(name) + "\n\n")
	}
	if description := strings.TrimSpace(c.Description); description != "" {
		b.WriteString(paragraph(description) + "\n")
	}
	if _, err := w.WriteString(b.String()); err != nil {
		return err
	}

	events := slices.Clone(c.Events)
	slices.SortStableFunc(events, func(a, b *ical.Event) int {
		return cmp.Compare(a.Start.Time.UnixNano(), b.Start.Time.UnixNano())
	})
	for _, e := range events {
		if err := ctx.Err(); err != nil {
			return err
		} else if _, err := w.WriteString(eventMarkdown(toEvent(e))); err != nil {
			return err
		}
	}
	return nil
}

// eventMarkdown returns an event as Markdown: its title as a heading, its
// details as a list, and its description
func eventMarkdown(e Event) string {
	var b strings.Builder
	title := strings.TrimSpace(e.Title)
	if title == "" {
		title = "Untitled event"
		if e.Kind == "task" {
			title = "Untitled task"
		}
	}
	b.WriteString("## " + itext.Escape(title) + "\n\n")

	var items []string
	item := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			items = append(items, "- **"+label+":** "+itext.Escape(value))
		}
	}
	item("When", eventWhen(e))
	item("Where", e.Location)
	item("Organizer", e.Organizer)
	item("Attendees", strings.Join(e.Attendees, ", "))
	item("Categories", strings.Join(e.Categories, ", "))
	if len(items) > 0 {
		b.WriteString(strings.Join(items, "\n") + "\n\n")
	}
	if description := strings.TrimSpace(e.Description); description != "" {
		b.WriteString(paragraph(description) + "\n")
	}
	return b.String()
}

// paragraph returns text as a paragraph of Markdown, escaped, with its line
// breaks kept
func paragraph(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = itext.Escape(strings.TrimSpace(line))
	}
	return strings.Join(lines, "  \n") + "\n"
}
