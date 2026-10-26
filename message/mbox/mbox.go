// Package mbox registers a handler of mailboxes in the mbox format, such as
// Eudora's .mbx files, which returns their kind, how many messages they
// have, when the first and last were sent, their subjects and senders, and
// their messages as Markdown
package mbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	email "github.com/mutablelogic/go-metadata/message/email"
	eml "github.com/mutablelogic/go-metadata/pkg/eml"
	mbox "github.com/mutablelogic/go-metadata/pkg/mbox"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// summary is what's known of a mailbox's messages: how many there are, the
// dates of the first and last, their subjects, and their senders
type summary struct {
	messages   int
	start, end time.Time

	// The subjects, by their keys (see subjectKey)
	subjects map[string]*count

	// The senders, and the indexes of the senders by their addresses and
	// names, without case, so a sender with more than one address, or
	// whose address is sometimes without their name, is one sender
	senders   []*count
	byAddress map[string]int
	byName    map[string]int
}

// count is a subject or a sender of a mailbox's messages, as it's shown, how
// many messages it's of, and when it was first seen
type count struct {
	text     string
	messages int
	order    int
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of a mailbox
	Namespace = "mbox"

	// maxSubjects is the most subjects of a mailbox's messages which are
	// returned, the most common first, as a mailbox can have many
	maxSubjects = 1000

	// maxSenders is the most senders of a mailbox's messages which are
	// returned, those who sent the most first
	maxSenders = 100

	// untitled is the heading of a message without a subject, in the
	// Markdown of a mailbox, so its messages are separated
	untitled = "(no subject)"
)

var (
	reContentType = regexp.MustCompile(`^application/mbox$`)

	// reReply matches the prefix of the subject of a reply, or a forwarded
	// message, such as Re:, Fwd: or AW:, which can be numbered, as Re[2]:
	reReply = regexp.MustCompile(`(?i)^\s*(?:re|fw|fwd|aw|wg|sv|vs|antw|tr|rif|res|enc)(?:\s*\[\d+\])?\s*:\s*`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The kind of a mailbox, and what's known of its messages, and its
	// messages as Markdown, only when the markdown namespace is requested,
	// which are read a message at a time, as a mailbox can be large
	metadata.AddHandler(reContentType, Namespace, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindMailbox)
		var w *imarkdown.Writer
		var werr error
		if o.HasNamespace(metadata.MarkdownNamespace) {
			w = imarkdown.NewWriter(o)
		}

		s := summary{subjects: make(map[string]*count), byAddress: make(map[string]int), byName: make(map[string]int)}
		hints := make(map[string]bool)
		reader := mbox.NewReader(r)
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			msg, err := reader.Next()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return kind, err
			}
			m, err := eml.Parse(bytes.NewReader(msg.Data))
			if err != nil {
				continue
			}

			// A message without a date is from when it was received
			if m.Date.IsZero() {
				m.Date = msg.Date
			}
			s.add(m)
			for _, hint := range email.Hints(m) {
				hints[hint] = true
			}

			// The message as Markdown, until the writer has its part
			if w != nil && werr == nil {
				markdown, err := email.Markdown(ctx, m, untitled)
				if err != nil {
					return kind, err
				}
				if s.messages > 1 {
					markdown = "\n" + markdown
				}
				_, werr = w.WriteString(markdown + "\n")
			}
		}

		// The hints of its messages: that it can be read as Markdown, when it
		// has messages, and has events, when a message has a calendar
		result := append(s.metadata(), kind...)
		result = append(result, metadata.HintMetadata(slices.Sorted(maps.Keys(hints))...)...)
		if w == nil {
			return result, nil
		}
		markdown, err := w.Metadata(werr)
		return append(result, markdown...), err
	}, Namespace, metadata.MetadataNamespace, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// add adds a message to the summary
func (s *summary) add(m *eml.Message) {
	s.messages++
	if !m.Date.IsZero() {
		if s.start.IsZero() || m.Date.Before(s.start) {
			s.start = m.Date
		}
		if s.end.IsZero() || m.Date.After(s.end) {
			s.end = m.Date
		}
	}
	if subject := strings.Join(strings.Fields(m.Subject), " "); subject != "" {
		key := subjectKey(subject)
		if existing, exists := s.subjects[key]; exists {
			existing.messages++
		} else {
			s.subjects[key] = &count{text: stripReply(subject), messages: 1, order: len(s.subjects)}
		}
	}
	for _, from := range m.From {
		s.addSender(from.Name, from.Address)
	}
}

// addSender adds a message from the sender with the name and address, who
// is the sender with the address, or else with the name, when they've been
// seen, and is shown by their name, once they've been seen with one, or
// else their address
func (s *summary) addSender(name, address string) {
	address, name = strings.ToLower(address), strings.TrimSpace(name)
	if address == "" {
		return
	}
	index, exists := s.byAddress[address]
	if !exists && name != "" {
		index, exists = s.byName[strings.ToLower(name)]
	}
	if !exists {
		index = len(s.senders)
		s.senders = append(s.senders, &count{text: firstOf(name, address), order: index})
	}
	sender := s.senders[index]
	sender.messages++
	if name != "" && !strings.Contains(sender.text, " ") && strings.Contains(sender.text, "@") {
		sender.text = name
	}
	s.byAddress[address] = index
	if name != "" {
		s.byName[strings.ToLower(name)] = index
	}
}

// stripReply returns a subject without the prefixes of replies and
// forwarded messages, such as Re: Fwd:, or the subject when it's only them
func stripReply(subject string) string {
	stripped := subject
	for {
		next := reReply.ReplaceAllString(stripped, "")
		if next == stripped {
			break
		}
		stripped = next
	}
	return firstOf(strings.TrimSpace(stripped), subject)
}

// subjectKey returns the key of a subject, which is without the prefixes of
// replies and forwarded messages, and without case, so a reply has the
// subject of the message it replies to
func subjectKey(subject string) string {
	return strings.ToLower(stripReply(subject))
}

// mostCommon returns the texts of the counts, those of the most messages
// first, and then those first seen first, up to the most
func mostCommon(counts []*count, most int) []string {
	slices.SortFunc(counts, func(a, b *count) int {
		if a.messages != b.messages {
			return b.messages - a.messages
		}
		return a.order - b.order
	})
	result := make([]string, 0, min(len(counts), most))
	for _, c := range counts[:min(len(counts), most)] {
		result = append(result, c.text)
	}
	return result
}

// metadata returns what's known of the mailbox's messages: how many there
// are, when the first and last were sent, their subjects, without the
// prefixes of replies and forwarded messages, each once, the most common
// first, up to maxSubjects, with a note when there are more, and their
// senders, by their names, or else their addresses, those who sent the most
// first, up to maxSenders
func (s *summary) metadata() []metadata.Metadata {
	result := []metadata.Metadata{meta.New(Namespace, "messages", s.messages)}
	if !s.start.IsZero() {
		result = append(result,
			meta.New(Namespace, "date-start", s.start.UTC()),
			meta.New(Namespace, "date-end", s.end.UTC()),
		)
	}
	if len(s.subjects) > 0 {
		subjects := mostCommon(slices.Collect(maps.Values(s.subjects)), maxSubjects)
		result = append(result, meta.NewText(Namespace, "subjects", subjects, strings.Join(subjects, "; ")))
		if len(s.subjects) > len(subjects) {
			result = append(result, meta.New(Namespace, "note", fmt.Sprintf("Only the %d most common of the mailbox's %d subjects are in mbox:subjects.", len(subjects), len(s.subjects))))
		}
	}
	if len(s.senders) > 0 {
		senders := mostCommon(slices.Clone(s.senders), maxSenders)
		result = append(result, meta.NewText(Namespace, "senders", senders, strings.Join(senders, "; ")))
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
