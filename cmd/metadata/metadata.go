package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"mime"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	// Packages
	kong "github.com/alecthomas/kong"
	doublestar "github.com/bmatcuk/doublestar/v4"
	glamour "github.com/charmbracelet/glamour"
	lipgloss "github.com/charmbracelet/lipgloss"
	table "github.com/charmbracelet/lipgloss/table"
	ansi "github.com/charmbracelet/x/ansi"
	term "github.com/charmbracelet/x/term"
	metadata "github.com/mutablelogic/go-metadata"
	byterange "github.com/mutablelogic/go-metadata/internal/byterange"
	xmp "github.com/mutablelogic/go-metadata/pkg/xmp"

	// Handlers
	_ "github.com/mutablelogic/go-metadata/application"
	_ "github.com/mutablelogic/go-metadata/audio"
	_ "github.com/mutablelogic/go-metadata/image"
	_ "github.com/mutablelogic/go-metadata/text"
	_ "github.com/mutablelogic/go-metadata/video"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	Type      bool     `name:"type" xor:"type,type-xmp" help:"Print the content type only, rather than the metadata."`
	JSON      bool     `name:"json" xor:"artwork-json,json-xmp" help:"Print as JSON, rather than as a table."`
	XMP       bool     `name:"xmp" xor:"type-xmp,json-xmp,markdown-xmp,artwork-xmp" help:"Print the metadata of each file as an XMP document, as a sidecar is, which describes the file by its path, rather than as a table. Artwork and Markdown aren't included."`
	Markdown  bool     `name:"markdown" xor:"type,ns,artwork-markdown,markdown-xmp" help:"Print the content of each file as Markdown, rather than its metadata."`
	Artwork   bool     `name:"artwork" xor:"type,ns,artwork-markdown,artwork-json,artwork-xmp" help:"Save the artwork of the files, such as the images of the pages of a PDF document, as images in the current directory, such as page0001.png, rather than printing their metadata. Saving stops at an image which already exists, such as the first page of the next file."`
	Force     bool     `name:"force" help:"With --artwork, overwrite images which already exist, and carry on."`
	DPI       int      `name:"dpi" help:"The resolution of the images of pages, such as the artwork of scanned pages, which are downscaled to it. By default, it's 150 dpi, and at most 1024 pixels wide or high; when it's set, it's the resolution however large, such as 300 dpi, the usual resolution for OCR."`
	Namespace []string `name:"ns" xor:"ns" help:"Print only the metadata in these namespaces." sep:","`
	Password  string   `name:"password" env:"METADATA_PASSWORD" help:"The password of encrypted documents, such as PDF documents which need a password to be opened. Set it in the environment, rather than as a flag, to keep it out of the shell's history."`
	Range     string   `name:"range" placeholder:"FROM:TO" help:"With --markdown, print only the bytes of the Markdown from:to, such as 0:4096, and then 4096:8192, or from:, :to, or : for all of it. By default, it's all of it."`
	Path      string   `arg:"" name:"path" help:"File, directory, or glob pattern such as 'photos/**/*.jpg' (quoted, so the shell doesn't expand it), of the files to read."`

	// renderer renders Markdown for the terminal, once there is some
	renderer *glamour.TermRenderer

	// offset and limit are the range of the Markdown, from --range, and
	// paged is true when it was set, so the pages are read in turn
	offset, limit uint64
	paged         bool
}

// result is the content type, or metadata, of a file
type result struct {
	Path        string            `json:"path"`
	ContentType string            `json:"content_type,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	PageRange   string            `json:"page_range,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`

	// keys of the metadata, in order
	keys []string

	// meta is the metadata, as it was extracted, without notes
	meta []metadata.Metadata

	// rel is the path relative to the directory of the files which match
	rel string
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// exitInterrupted is the exit status when interrupted, which is 128 and
	// the number of the signal, SIGINT, as a shell expects
	exitInterrupted = 130

	// noteKey is the key of the metadata which is a warning, from an error
	// returned along with metadata
	noteKey = "metadata:note"

	// The keys of the content of a file as Markdown, the offset it starts
	// at, the offset of the next part, when there's more, and the note of
	// how to read the next page, with --range
	markdownKey       = metadata.MarkdownNamespace + ":body"
	markdownOffsetKey = metadata.MarkdownNamespace + ":offset"
	markdownNextKey   = metadata.MarkdownNamespace + ":next"
	markdownNoteKey   = metadata.MarkdownNamespace + ":note"

	// defaultMarkdownWidth is the width Markdown is wrapped to, when the
	// width of the terminal isn't known
	defaultMarkdownWidth = 80

	// maxValueWidth is the widest a value is printed in a table, in
	// characters, before it's truncated
	maxValueWidth = 80
)

// existsError is an image which already exists, which isn't overwritten,
// so saving the artwork stops
type existsError string

func (e existsError) Error() string {
	return string(e) + " already exists, use --force to overwrite it"
}

func (e existsError) Unwrap() error {
	return fs.ErrExist
}

///////////////////////////////////////////////////////////////////////////////
// MAIN

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("metadata"),
		kong.Description("Print the content type and metadata of files."),
		kong.UsageOnError(),
	)

	// Cancel on an interrupt, or when terminated, and then restore the
	// default behaviour, so a second interrupt exits at once
	parent, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-parent.Done()
		stop()
	}()

	// Exit quietly when interrupted, as a shell expects
	if err := cli.Run(parent); parent.Err() != nil {
		os.Exit(exitInterrupted)
	} else {
		ctx.FatalIfErrorf(err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Validate parses the range, which can only be used with --markdown, and is
// the whole document by default
func (cli *CLI) Validate() error {
	if cli.Range != "" && !cli.Markdown {
		return errors.New("--range can only be used with --markdown")
	} else if cli.Force && !cli.Artwork {
		return errors.New("--force can only be used with --artwork")
	}
	cli.paged = cli.Range != ""
	if !cli.paged {
		cli.Range = ":"
	}
	offset, limit, err := byterange.Parse(cli.Range)
	if err != nil {
		return fmt.Errorf("--range: %w", err)
	}
	cli.offset, cli.limit = offset, limit
	return nil
}

// Run prints the content type and metadata of the files which match the
// path, which is a file, a directory, which matches every file within it and
// its subdirectories, or a glob pattern, which can use ** to match any
// number of directories. Hidden files and directories only match when the
// pattern names them, and symlinks aren't followed. It continues after a
// file which fails, and returns the errors of all the files which failed. A
// file with no handler for its content type, or whose handler failed but
// returned metadata, has a warning as a note, rather than failing. As JSON,
// or Markdown, each file is written once it's read, and as a table, every
// file is written once they've all been read.
func (cli *CLI) Run(ctx context.Context) error {
	var results []result
	var errs error
	var matched bool
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	base, pattern := cli.glob()
	walkErr := doublestar.GlobWalk(os.DirFS(base), pattern, func(rel string, d fs.DirEntry) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Skip anything which isn't a regular file, such as a symlink
		if !d.Type().IsRegular() {
			return nil
		}
		matched = true

		rel = filepath.FromSlash(rel)
		r := result{Path: filepath.Join(base, rel), rel: rel}
		if err := cli.describe(ctx, &r); err != nil {
			r.Error = err.Error()
			errs = errors.Join(errs, fmt.Errorf("%s: %w", r.Path, err))
		}

		// Write the file as indented JSON, or as Markdown, now, stopping
		// when it can't be written, or keep it for the table
		switch {
		case cli.JSON:
			return enc.Encode(r)
		case cli.XMP:
			return cli.writeXMP(os.Stdout, r)
		case cli.Markdown:
			return cli.writeMarkdown(os.Stdout, r)
		case cli.Artwork:
			// Save the artwork, stopping at an image which already exists
			if err := cli.writeArtwork(os.Stdout, r); errors.Is(err, fs.ErrExist) {
				return err
			} else if err != nil {
				errs = errors.Join(errs, fmt.Errorf("%s: %w", r.Path, err))
			}
			return nil
		}
		results = append(results, r)
		return nil
	}, doublestar.WithFilesOnly(), doublestar.WithNoFollow(), doublestar.WithNoHidden())
	if errors.Is(walkErr, fs.ErrExist) {
		errs = errors.Join(errs, walkErr)
	} else if walkErr != nil {
		errs = errors.Join(errs, fmt.Errorf("%s: %w", cli.Path, walkErr))
	} else if !matched {
		// Report the directory of the pattern not existing, rather than no
		// files matching
		if _, err := os.Stat(base); err != nil {
			errs = errors.Join(errs, err)
		} else {
			errs = errors.Join(errs, fmt.Errorf("%s: no files match", cli.Path))
		}
	}

	// Write the table, and then return the errors
	if !cli.JSON && !cli.XMP && !cli.Markdown && !cli.Artwork {
		errs = errors.Join(cli.writeTable(os.Stdout, results), errs)
	}
	return errs
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// describe sets the content type of the file, with its parameters, and its
// metadata, unless only the content type is printed. When there is no
// handler for the content type, or a handler fails but there is metadata,
// the file doesn't fail, and the error is a warning, as a metadata:note.
func (cli *CLI) describe(ctx context.Context, r *result) error {
	f, err := os.Open(r.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Detect only the content type
	if cli.Type {
		r.ContentType, r.Params, err = metadata.ContentType(f)
		return err
	}

	// Extract the metadata, and the content type
	opts := []metadata.Option{}
	if cli.Password != "" {
		opts = append(opts, metadata.WithPassword(cli.Password))
	}
	if cli.DPI != 0 {
		opts = append(opts, metadata.WithImageDPI(cli.DPI))
	}
	switch {
	case cli.Artwork:
		opts = append(opts, metadata.WithNamespace(metadata.ArtworkNamespace))
	case cli.Markdown:
		opts = append(opts, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(cli.Range))
	case len(cli.Namespace) > 0:
		opts = append(opts, metadata.WithNamespace(cli.Namespace...))
	}
	contentType, meta, err := metadata.Extract(ctx, f, opts...)
	if contentType != "" {
		var parseErr error
		if r.ContentType, r.Params, parseErr = mime.ParseMediaType(contentType); parseErr != nil {
			return errors.Join(err, parseErr)
		}
	}
	// An error along with metadata, or no handler for the content type, is
	// a warning, which is a note in the metadata, rather than a failure
	notes := make(map[string]any, 1)
	switch {
	case err == nil:
	case len(meta) > 0, errors.Is(err, metadata.ErrNotImplemented):
		notes[noteKey] = strings.Join(strings.Split(err.Error(), "\n"), "; ")
	default:
		return err
	}
	if len(meta)+len(notes) == 0 {
		return nil
	}
	r.Metadata = notes
	r.meta = meta
	for _, m := range meta {
		r.Metadata[m.NS()+string(metadata.Separator)+m.Key()] = jsonValue(m)
	}
	if cli.paged {
		cli.paginate(r)
	}
	r.keys = slices.Sorted(maps.Keys(r.Metadata))
	return nil
}

// paginate replaces the offset of the part of the Markdown, and the offset
// of the next part, with the range of the part, as --range takes it, and a
// note of how to read the next part, so the pages can be read in turn
func (cli *CLI) paginate(r *result) {
	if _, exists := r.Metadata[markdownKey]; !exists {
		return
	}
	// The page starts at the offset, which is moved forward to the start of
	// a character, and ends at the next page, or the end of the Markdown
	start := cli.offset
	if v, ok := r.value(markdownOffsetKey); ok {
		start, _ = strconv.ParseUint(v, 10, 64)
	}
	body, _ := r.value(markdownKey)
	end := start + uint64(len(body))
	next, more := r.value(markdownNextKey)
	if more {
		end, _ = strconv.ParseUint(next, 10, 64)
	}
	delete(r.Metadata, markdownOffsetKey)
	delete(r.Metadata, markdownNextKey)

	r.PageRange = formatRange(start, end)
	switch {
	case more:
		r.Metadata[markdownNoteKey] = fmt.Sprintf("This page is bytes %d to %d of the document as Markdown, and there are more pages. To read the next page, use --range %s.", start, end, formatRange(end, end+cli.limit))
	case start > 0:
		r.Metadata[markdownNoteKey] = fmt.Sprintf("This page is bytes %d to %d, the last page of the document as Markdown. There are no more pages.", start, end)
	default:
		r.Metadata[markdownNoteKey] = fmt.Sprintf("This page is the whole document as Markdown, bytes %d to %d. There are no more pages.", start, end)
	}
}

// writeTable writes the results as a table of the path and content type of
// each file, and a row for the key and value of each of its metadata entries, with the path and
// content type only on its first row. A file which failed has no row, as
// its error is returned. Only the path and content type are written when
// only the content type is printed.
func (cli *CLI) writeTable(w io.Writer, results []result) error {
	columns := []string{"Path", "Content Type", "Key", "Value"}
	if cli.Type {
		columns = columns[:2]
	}
	var rows [][]string
	for _, r := range results {
		if r.ContentType == "" {
			continue
		}
		row := []string{r.rel, mime.FormatMediaType(r.ContentType, r.Params)}
		if cli.Type {
			rows = append(rows, row)
			continue
		}
		if len(r.keys) == 0 {
			rows = append(rows, append(row, "", ""))
		}
		for _, key := range r.keys {
			rows = append(rows, append(row, key, tableValue(r.Metadata[key])))
			row = []string{"", ""}
		}
	}
	if len(rows) == 0 {
		return nil
	}
	_, err := fmt.Fprintln(w, renderTable(columns, rows, terminalWidth(w)))
	return err
}

// glob returns the directory, and the glob pattern of the files within it,
// which the path matches. A directory matches every file within it and its
// subdirectories, and a file matches itself, so neither is read as a
// pattern, in case its name has characters which are in patterns.
func (cli *CLI) glob() (string, string) {
	if info, err := os.Stat(cli.Path); err == nil {
		if info.IsDir() {
			return cli.Path, "**"
		}
		return filepath.Dir(cli.Path), escapeMeta(filepath.Base(cli.Path))
	}
	base, pattern := doublestar.SplitPattern(filepath.ToSlash(cli.Path))
	if filepath.Separator == '/' {
		base = unescape(base)
	}
	return filepath.FromSlash(base), pattern
}

// writeMarkdown writes the Markdown of the file, rendered for the terminal
// when it is one, under a heading of its path, and its note, such as how to
// read the next page, or why there's no Markdown, or nothing when the file
// has neither, such as when there's no handler for its content type
func (cli *CLI) writeMarkdown(w io.Writer, r result) error {
	body, _ := r.value(markdownKey)
	note, _ := r.value(markdownNoteKey)
	if strings.TrimSpace(body) == "" && note == "" {
		// Without Markdown, a warning, such as no handler for its content
		// type, is the note
		if note, _ = r.value(noteKey); note == "" {
			return nil
		}
	}

	// Wrap the Markdown to the width of the terminal, or a page
	width := terminalWidth(w)
	if width <= 0 {
		width = defaultMarkdownWidth
	}
	if cli.renderer == nil {
		renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(width), glamour.WithPreservedNewLines())
		if err != nil {
			return err
		}
		cli.renderer = renderer
	}
	var rendered string
	if strings.TrimSpace(body) != "" {
		var err error
		if rendered, err = cli.renderer.Render(body); err != nil {
			return err
		}
	}

	// The heading is the path, with a rule under it
	heading := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "63", Dark: "212"}).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Width(width)
	if _, err := fmt.Fprintf(w, "%s\n%s\n", heading.Render(r.rel), rendered); err != nil {
		return err
	}

	// The note, such as how to read the next page, or why there's no
	// Markdown
	if note != "" {
		style := lipgloss.NewStyle().Faint(true).Italic(true).Width(width)
		if _, err := fmt.Fprintf(w, "%s\n\n", style.Render(note)); err != nil {
			return err
		}
	}
	return nil
}

// writeXMP writes the metadata of the file as an XMP document, as a
// sidecar is, which describes the file by its path, without its artwork or Markdown, which
// aren't text metadata, or nothing when it has none, or it failed
func (cli *CLI) writeXMP(w io.Writer, r result) error {
	var items []metadata.Metadata
	for _, m := range r.meta {
		if m == nil || m.NS() == metadata.ArtworkNamespace || m.NS() == metadata.MarkdownNamespace {
			continue
		}
		items = append(items, m)
	}
	if len(items) == 0 {
		return nil
	}
	x := xmp.FromMetadata(items)
	x.SetAbout(r.Path)
	return x.WriteSidecar(w)
}

// writeArtwork saves each image of the artwork of the file in the current
// directory, named by its key and the extension of its type, such as
// page0001.png, and writes the path of each image which is saved. An image
// which already exists isn't saved, unless it's forced, and is an
// existsError, which stops saving the artwork. A file which has no artwork
// is reported, when the path is the file itself, rather than a pattern.
func (cli *CLI) writeArtwork(w io.Writer, r result) error {
	var images []metadata.Metadata
	for _, key := range r.keys {
		if image, ok := r.Metadata[key].(metadata.Metadata); ok && image.NS() == metadata.ArtworkNamespace && len(image.Bytes()) > 0 {
			images = append(images, image)
		}
	}
	if len(images) == 0 {
		if info, err := os.Stat(cli.Path); err == nil && info.Mode().IsRegular() {
			_, err := fmt.Fprintf(os.Stderr, "%s: no artwork\n", r.Path)
			return err
		}
		return nil
	}

	// Don't overwrite an image, unless it's forced
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if cli.Force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	for _, image := range images {
		path := image.Key() + imageExtension(image.Value())
		if err := writeFile(path, flags, image.Bytes()); errors.Is(err, fs.ErrExist) {
			return existsError(path)
		} else if err != nil {
			return err
		} else if _, err := fmt.Fprintln(w, path); err != nil {
			return err
		}
	}
	return nil
}

// value returns the value of the metadata with the key, as a string, which
// is the value itself, or the value of an entry which marshals its own value
func (r result) value(key string) (string, bool) {
	switch v := r.Metadata[key].(type) {
	case string:
		return v, true
	case metadata.Metadata:
		return v.Value(), true
	}
	return "", false
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// writeFile writes the data to the file, opened with the flags
func writeFile(path string, flags int, data []byte) error {
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return errors.Join(err, f.Close())
	}
	return f.Close()
}

// imageExtension returns the extension of the type of an image, such as
// .png for image/png
func imageExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

// renderTable returns the table of the rows, with a bold header separated
// from the rows by a line, and no other borders. When width is greater than
// zero, and the table is wider, its columns are narrowed to fit, and their
// cells truncated.
func renderTable(headers []string, rows [][]string, width int) string {
	t := newTable(headers, rows)
	if width > 0 && lipgloss.Width(t.String()) > width {
		t = t.Width(width).Wrap(false)
	}
	return t.String()
}

// newTable returns the table of the rows, as wide as its widest cells
func newTable(headers []string, rows [][]string) *table.Table {
	header := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	cell := lipgloss.NewStyle().Padding(0, 1)
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderHeader(true).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return header
			}
			return cell
		})
}

// terminalWidth returns the width of the terminal which w writes to, or zero
// when w isn't a terminal, such as a pipe or a file
func terminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(f.Fd()) {
		return 0
	}
	width, _, err := term.GetSize(f.Fd())
	if err != nil {
		return 0
	}
	return width
}

// formatRange returns a range of bytes as --range takes it, which is
// START:END
func formatRange(start, end uint64) string {
	return fmt.Sprintf("%d:%d", start, end)
}

// escapeMeta returns the name with the characters which are in glob
// patterns escaped, so the name only matches itself
func escapeMeta(name string) string {
	var b strings.Builder
	for _, r := range name {
		if strings.ContainsRune(`\*?[]{}`, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// unescape returns the directory of a pattern with each character escaped
// by a backslash, such as a space in 'Other\ Backups', as the character, as
// it is in the rest of the pattern
func unescape(dir string) string {
	var b strings.Builder
	escaped := false
	for _, r := range dir {
		if r == '\\' && !escaped {
			escaped = true
			continue
		}
		escaped = false
		b.WriteRune(r)
	}
	return b.String()
}

// jsonValue returns the value of a metadata entry to marshal as JSON, which
// is the entry itself when it marshals its own value, or else its value as
// a string
func jsonValue(m metadata.Metadata) any {
	if _, ok := m.(json.Marshaler); ok {
		return m
	}
	return m.Value()
}

// tableValue returns the value of a metadata entry as a single line, which
// is truncated when it's wider than maxValueWidth
func tableValue(v any) string {
	var s string
	if m, ok := v.(metadata.Metadata); ok {
		s = m.Value()
	} else {
		s = fmt.Sprint(v)
	}
	return ansi.Truncate(strings.Join(strings.Fields(s), " "), maxValueWidth, "…")
}
