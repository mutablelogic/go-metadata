// Package email registers a handler of email messages, such as .eml files,
// which returns their kind, their subject, sender, date, identifier and the
// start of their text as Dublin Core, their addresses and attachments, and
// their headers and text as Markdown
package email

import (
	"context"
	"io"
	"mime"
	"net/mail"
	"regexp"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	eml "github.com/mutablelogic/go-metadata/pkg/eml"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the addresses and attachments of an
	// email message
	Namespace = "email"

	// maxSize is the most of a message which is read, which can be large
	// with its attachments
	maxSize = 64 << 20

	// maxDescription is the most of a message's text which is its
	// description, in bytes
	maxDescription = 1000
)

var (
	reContentType = regexp.MustCompile(`^message/rfc822$`)

	// reOriginal matches the line before the original message in a reply,
	// as Outlook writes it, such as -----Original Message-----
	reOriginal = regexp.MustCompile(`(?i)^-{2,}\s*original message\s*-{2,}$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The kind of an email message, its metadata, and its headers and text
	// as Markdown, only when the markdown namespace is requested
	metadata.AddHandler(reContentType, "email", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindEmail)
		m, err := eml.Parse(io.LimitReader(r, maxSize))
		if err != nil {
			return kind, err
		}
		result := append(messageMetadata(m), kind...)
		result = append(result, metadata.HintMetadata(Hints(m)...)...)
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return result, nil
		}
		w := imarkdown.NewWriter(o)
		markdown, err := w.Metadata(writeMarkdown(ctx, w, m))
		return append(result, markdown...), err
	}, Namespace, dc.Namespace, metadata.MetadataNamespace, metadata.MarkdownNamespace)
}

// Hints returns the hints of a message: that it can be read as Markdown,
// and that it has events, when a calendar is attached to it, such as an
// invitation
func Hints(m *eml.Message) []string {
	hints := []string{metadata.HintMarkdown}
	for _, a := range m.Attachments {
		if contentType, _, _ := mime.ParseMediaType(a.ContentType); contentType == "text/calendar" {
			return append(hints, metadata.HintEvent)
		}
	}
	return hints
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Markdown returns a message as Markdown: its subject as a heading, or the
// untitled heading, when it has no subject and it isn't empty, its sender,
// recipients, date and attachments as a list, and its text as paragraphs,
// with its line breaks, and its quoted lines as block quotes, or its HTML
// converted to Markdown, when it has no text. It's empty for a message
// without any of them.
func Markdown(ctx context.Context, m *eml.Message, untitled string) (string, error) {
	var blocks []string
	if subject := firstOf(m.Subject, untitled); subject != "" {
		blocks = append(blocks, "# "+itext.Escape(subject))
	}
	var items []string
	item := func(label string, values []string) {
		if len(values) > 0 {
			items = append(items, "- **"+label+":** "+itext.Escape(strings.Join(values, ", ")))
		}
	}
	item("From", addresses(m.From))
	item("To", addresses(m.To))
	item("Cc", addresses(m.Cc))
	if !m.Date.IsZero() {
		item("Date", []string{m.Date.Format("Mon, 2 Jan 2006 15:04 -0700")})
	}
	item("Attachments", attachments(m))
	if len(items) > 0 {
		blocks = append(blocks, strings.Join(items, "\n"))
	}

	// The text, as paragraphs, or else the HTML
	if strings.TrimSpace(m.Text) == "" {
		markdown, err := htmlMarkdown(ctx, m.HTML)
		if err != nil {
			return "", err
		} else if markdown != "" {
			blocks = append(blocks, markdown)
		}
	} else {
		blocks = append(blocks, paragraphs(m.Text)...)
	}
	return strings.Join(blocks, "\n\n"), nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// messageMetadata returns the metadata of a message: its subject as dc:title,
// its sender as dc:creator, by their name, or else their address, its date
// as dc:date, its identifier as dc:identifier, the start of its text as
// dc:description, and its senders', recipients' and copied recipients'
// addresses, and the names of its attachments, in the email namespace
func messageMetadata(m *eml.Message) []metadata.Metadata {
	var result []metadata.Metadata
	add := func(ns, key string, value any) {
		if entry := meta.New(ns, key, value); entry != nil {
			result = append(result, entry)
		}
	}
	list := func(key string, values []string) {
		if len(values) > 0 {
			result = append(result, meta.NewText(Namespace, key, values, strings.Join(values, ", ")))
		}
	}

	if m.Subject != "" {
		add(dc.Namespace, dc.Title, m.Subject)
	}
	if len(m.From) > 0 {
		add(dc.Namespace, dc.Creator, firstOf(m.From[0].Name, m.From[0].Address))
	}
	if !m.Date.IsZero() {
		add(dc.Namespace, dc.Date, m.Date.UTC())
	}
	if m.MessageID != "" {
		add(dc.Namespace, dc.Identifier, m.MessageID)
	}
	if text := description(text(m)); text != "" {
		add(dc.Namespace, dc.Description, text)
	}
	list("from", addresses(m.From))
	list("to", addresses(m.To))
	list("cc", addresses(m.Cc))
	list("attachments", attachments(m))
	return result
}

// writeMarkdown writes a message as Markdown (see Markdown). It stops with
// the writer's error, once it has its part.
func writeMarkdown(ctx context.Context, w io.StringWriter, m *eml.Message) error {
	markdown, err := Markdown(ctx, m, "")
	if err != nil || markdown == "" {
		return err
	}
	_, err = w.WriteString(markdown + "\n")
	return err
}

// text returns a message's text, or its HTML as plain text, when it has no
// text
func text(m *eml.Message) string {
	if strings.TrimSpace(m.Text) == "" {
		return itext.PlainText(m.HTML)
	}
	return m.Text
}

// description returns the start of a message's text, up to the original
// message which it replies to, when it's included, without its lines which
// are quoted, with its spaces collapsed, cut short at a word when it's longer
// than maxDescription
func description(text string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); reOriginal.MatchString(line) {
			break
		} else if line != "" && !strings.HasPrefix(line, ">") {
			lines = append(lines, strings.Join(strings.Fields(line), " "))
		}
	}
	result := strings.Join(lines, " ")
	if len(result) > maxDescription {
		result = strings.ToValidUTF8(result[:maxDescription], "")
		if i := strings.LastIndex(result, " "); i > 0 {
			result = result[:i]
		}
		result += "…"
	}
	return result
}

// paragraphs returns the paragraphs of a message's text as Markdown, which
// are its lines separated by blank lines, or by a change in how deeply
// they're quoted, as with a reply's > before them. A paragraph keeps its
// line breaks, and a quoted paragraph is a block quote, as deep as its
// quote.
func paragraphs(text string) []string {
	var result, lines []string
	depth := 0
	end := func() {
		if len(lines) > 0 {
			prefix := strings.Repeat("> ", depth)
			result = append(result, prefix+strings.Join(lines, "\\\n"+prefix))
		}
		lines = nil
	}
	for _, line := range strings.Split(text, "\n") {
		d, content := quote(line)
		if content = strings.TrimSpace(content); content == "" {
			end()
			continue
		} else if d != depth {
			end()
			depth = d
		}
		lines = append(lines, itext.Escape(content))
	}
	end()
	return result
}

// quote returns how deeply a line is quoted, which is how many > it starts
// with, which can have spaces between them, and the line without them
func quote(line string) (int, string) {
	depth := 0
	for {
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, ">") {
			return depth, line
		}
		depth++
		line = trimmed[1:]
	}
}

// addresses returns the addresses, as "Name <address>", or the address
// alone, when it has no name
func addresses(list []*mail.Address) []string {
	var result []string
	for _, address := range list {
		if address.Name != "" {
			result = append(result, address.Name+" <"+address.Address+">")
		} else {
			result = append(result, address.Address)
		}
	}
	return result
}

// attachments returns the names of a message's attachments which have one
func attachments(m *eml.Message) []string {
	var result []string
	for _, a := range m.Attachments {
		if a.Name != "" {
			result = append(result, a.Name)
		}
	}
	return result
}

// firstOf returns the first value which isn't empty
func firstOf(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
