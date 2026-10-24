package playlist

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	itext "github.com/mutablelogic/go-metadata/internal/text"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The entries of a playlist as Markdown, only when the markdown
	// namespace is requested
	metadata.AddHandler(ContentTypes, "playlist-markdown", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		data, err := io.ReadAll(io.LimitReader(r, maxPlaylistSize))
		if err != nil {
			return nil, err
		}
		w := imarkdown.NewWriter(o)
		return w.Metadata(parsePlaylist(data).writeMarkdown(ctx, w))
	}, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// writeMarkdown writes the playlist as Markdown: its title as a heading,
// and its entries as a numbered list, of their titles, with their
// durations, when they have them, which are links to their locations when
// they're URLs, and otherwise followed by their locations, or their
// locations when they have no title. It stops with the writer's error, once
// it has its part.
func (p *playlist) writeMarkdown(ctx context.Context, w io.StringWriter) error {
	if p.title != "" {
		if _, err := w.WriteString("# " + itext.Escape(p.title) + "\n\n"); err != nil {
			return err
		}
	}
	for i, entry := range p.entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := w.WriteString(entry.markdown(i + 1)); err != nil {
			return err
		}
	}
	return nil
}

// markdown returns the entry as an item of a numbered list
func (e playlistEntry) markdown(number int) string {
	prefix := strconv.Itoa(number) + ". "
	indent := strings.Repeat(" ", len(prefix))
	location := strings.TrimSpace(e.location)

	// The title, which is a link to a URL, or else the location
	var line string
	isURL := false
	if u, err := url.Parse(location); err == nil && u.Scheme != "" && u.Host != "" {
		isURL = true
	}
	switch {
	case e.title != "" && isURL:
		line = "[" + itext.Escape(e.title) + "](<" + location + ">)"
	case e.title != "":
		line = itext.Escape(e.title)
	case isURL:
		line = "<" + location + ">"
	default:
		line = itext.Escape(location)
	}
	if e.timed {
		line += " (" + duration(e.duration) + ")"
	}

	// The location of a titled entry which isn't a URL, on the next line
	if e.title != "" && !isURL && location != "" {
		line += "\n" + indent + itext.Escape(location)
	}
	return prefix + line + "\n"
}

// duration returns a duration as minutes and seconds, or hours, minutes and
// seconds, such as 3:20 or 1:02:03
func duration(d time.Duration) string {
	seconds := int64(d.Round(time.Second) / time.Second)
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
