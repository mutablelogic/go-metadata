package metadata

import (
	"cmp"
	"context"
	"errors"
	"image"
	"io"
	"mime"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	dublincore "github.com/mutablelogic/go-metadata/dublincore"
	input "github.com/mutablelogic/go-metadata/internal/input"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Metadata is an entry of metadata, with a key and a value. The value can be
// retrieved as a string, as bytes, as an image (for artwork), or as is.
type Metadata interface {
	// Return the namespace of the metadata, such as "exif"
	NS() string

	// Return the metadata key within its namespace, such as
	// "DateTimeOriginal"
	Key() string

	// Return the value as a string. Returns the mimetype if the value is a
	// byte slice, and the mimetype can be detected.
	Value() string

	// Returns the value as a byte slice
	Bytes() []byte

	// Returns the value as an image, or nil if it isn't one
	Image() image.Image

	// Returns the value as an interface
	Any() any
}

// NamedReader is a reader with a name, such as a file. The extension of the
// name is used to detect the content type, and a handler may use the name
// itself (e.g. to look up TMDB metadata).
type NamedReader interface {
	io.Reader
	Name() string
}

// HandlerFunc is a function that can be used to extract metadata from a given
// reader. The context can be used to cancel or time out long-running
// extraction. Opts carries the options passed to Extract; in particular
// Opts.HasNamespace can be used to check whether a specific namespace (e.g.
// "artwork") was requested via WithNamespace, which handlers that do
// expensive work for a single namespace should use to opt in only when that
// namespace was explicitly requested. A handler can return metadata in any
// of its namespaces, as Extract returns only the metadata in the namespaces
// which were requested.
type HandlerFunc func(context.Context, io.Reader, *Opts) ([]Metadata, error)

type entry struct {
	name       string
	re         *regexp.Regexp
	priority   int
	namespaces []string
	handler    HandlerFunc

	// The namespaces of the metadata of other handlers which the handler
	// uses, so it runs after them (see AddHandlerAfter)
	after []string
}

// result is the metadata which a handler returned, and its priority
type result struct {
	priority int
	meta     []Metadata
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Separator is between the namespace and key of metadata, such as
	// dc:title
	Separator = ':'

	// DefaultPriority is the priority of a handler which is added with
	// AddHandler. A handler with a higher priority has its metadata first,
	// and in place of the metadata with the same namespace and key from a
	// handler with a lower priority (see AddHandlerWithPriority).
	DefaultPriority = 0

	// defaultMaxImageDimension is the largest width or height of an image,
	// in pixels, when it isn't set with WithMaxImageDimension
	defaultMaxImageDimension = 1024

	// defaultImageDPI is the resolution of an image of a page, in dots per
	// inch, when it isn't set with WithImageDPI
	defaultImageDPI = 150

	// MetadataNamespace is the namespace of the metadata about the input
	// itself, such as its kind, as metadata:kind
	MetadataNamespace = "metadata"

	// MarkdownNamespace is the namespace of a document's content as
	// Markdown, which a handler returns only when it's requested
	MarkdownNamespace = "markdown"

	// KindKey is the key of the kind of the input, such as a book, in the
	// metadata namespace (see the kinds in the dublincore package)
	KindKey = "kind"

	// HintKey is the key of the hints of what to read of the input next, in
	// the metadata namespace, such as its Markdown, which are a list of the
	// hints of all the handlers which read it (see HintMetadata), or an empty
	// list when no handler can read it, as there's nothing more to read of it
	HintKey = "hint"

	// ArtworkNamespace is the namespace of images which represent a
	// document, such as the thumbnails of its pages, which a handler
	// returns only when it's requested, as extracting them is expensive
	ArtworkNamespace = "artwork"

	// Namespace for artwork
	NamespaceArtwork = "artwork"

	// Namespace for markdown text
	NamespaceMarkdown = "markdown"
)

var handlerlock sync.Mutex
var handlers []entry
var cached = make(map[string][]entry)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Add a metadata handler for a given regular expression, along with a short
// name identifying it (e.g. "ffmpeg", "tmdb", "artwork" - used to name its
// span, "metadata.Handler.<name>", so it's distinguishable from other
// handlers in a trace) and the namespaces (e.g. "exif", "tiff") of metadata
// it can produce. A caller that requests specific namespaces via
// WithNamespace will only run handlers registered for one of those
// namespaces. The handler has the default priority, DefaultPriority.
func AddHandler(re *regexp.Regexp, name string, fn HandlerFunc, namespaces ...string) {
	AddHandlerWithPriority(re, name, DefaultPriority, fn, namespaces...)
}

// AddHandlerWithPriority adds a metadata handler, as AddHandler does, with
// a priority, which is DefaultPriority, zero, or higher for a handler whose
// metadata is better than others', such as the thumbnail embedded in a
// photo, rather than the photo itself. Extract returns the metadata of the
// handlers with the highest priority first, and of those with the same
// priority in the order they were added. Metadata with the same namespace
// and key as metadata from a handler with a higher priority isn't
// returned, but handlers with the same priority can each return it, such
// as more than one dc:creator.
func AddHandlerWithPriority(re *regexp.Regexp, name string, priority int, fn HandlerFunc, namespaces ...string) {
	if re == nil || fn == nil {
		panic(ErrBadParameter.With("nil regex or handler"))
	}
	handlerlock.Lock()
	defer handlerlock.Unlock()
	handlers = append(handlers, entry{name: name, re: re, priority: priority, namespaces: namespaces, handler: fn})
	cached = make(map[string][]entry)
}

// AddHandlerAfter adds a metadata handler, as AddHandler does, which uses
// the metadata of other handlers, in the namespaces after, such as a video's
// duration in the video namespace. It runs once the handlers which can
// produce them have finished, and gets their metadata from Opts.Get. Those
// handlers run even when their namespaces weren't requested, though their
// metadata isn't returned. A handler doesn't get the metadata of other
// handlers added with AddHandlerAfter, which run alongside it. The handler
// has the default priority, DefaultPriority.
func AddHandlerAfter(re *regexp.Regexp, name string, after []string, fn HandlerFunc, namespaces ...string) {
	AddHandlerAfterWithPriority(re, name, DefaultPriority, after, fn, namespaces...)
}

// AddHandlerAfterWithPriority adds a metadata handler which uses the
// metadata of other handlers, as AddHandlerAfter does, with a priority, as
// AddHandlerWithPriority does, such as a handler whose title is better than
// the title of the handlers which ran first.
func AddHandlerAfterWithPriority(re *regexp.Regexp, name string, priority int, after []string, fn HandlerFunc, namespaces ...string) {
	if re == nil || fn == nil {
		panic(ErrBadParameter.With("nil regex or handler"))
	}
	handlerlock.Lock()
	defer handlerlock.Unlock()
	handlers = append(handlers, entry{name: name, re: re, priority: priority, namespaces: namespaces, handler: fn, after: after})
	cached = make(map[string][]entry)
}

// HintMetadata returns the metadata of the hints of what to read of an input
// next, which a handler returns for an input it reads, such as HintMarkdown
// when it has content which can be read as Markdown, as metadata:hint, or
// nil when there are none. Extract returns the hints of all the handlers
// which read an input as one list, in order, whatever their priorities. A
// handler which returns it registers the metadata namespace.
func HintMetadata(hints ...string) []Metadata {
	if len(hints) == 0 {
		return nil
	}
	return []Metadata{hintMeta(slices.Clone(hints))}
}

// KindMetadata returns the metadata of the kind of an input, such as a book
// (see the kinds in the dublincore package), which a handler returns for the
// inputs it handles: the kind, as metadata:kind, and its DCMI type, such as
// Text, as dc:type. It returns nil when the kind is empty. A handler which
// returns it registers the metadata and dc namespaces.
func KindMetadata(kind string) []Metadata {
	if kind == "" {
		return nil
	}
	result := []Metadata{valueMeta{MetadataNamespace, KindKey, kind}}
	if typ := dublincore.TypeOf(kind); typ != "" {
		result = append(result, valueMeta{dublincore.Namespace, dublincore.Type, typ})
	}
	return result
}

// GetHandlers returns all handlers registered for a given content type, or
// nil if no handler is registered for that content type.
func GetHandlers(contentType string) []HandlerFunc {
	entries := getEntries(contentType)
	if len(entries) == 0 {
		return nil
	}
	fns := make([]HandlerFunc, len(entries))
	for i, entry := range entries {
		fns[i] = entry.handler
	}
	return fns
}

// Extract runs every handler registered for the content type of r against
// it, concurrently, and returns the content type, with its parameters, such
// as "text/html; charset=utf-8", and the combined metadata from all of them.
// The content type is set with WithContentType, or else detected from r.
// When r has a name, and no handler returned a title, its title is from its
// name, which is returned even when there is no handler for the content
// type, along with ErrNotImplemented. The metadata which was extracted is
// returned even when a handler fails.
//
// r is read from its current position, and each handler reads, and can seek
// in, its own section of it. When r can't be read at an offset and seeked,
// such as a pipe, it's copied to a temporary file first, which is removed
// when Extract returns. Use WithMaxSize to limit the size of r.
func Extract(parent context.Context, r io.Reader, opts ...Option) (_ string, _ []Metadata, err error) {
	// Check arguments
	if r == nil {
		return "", nil, ErrBadParameter.With("nil reader")
	}

	// Apply options
	o, err := applyOptions(opts...)
	if err != nil {
		return "", nil, err
	}

	// Start the span
	ctx, endSpan := otel.StartSpan(o.tracer, parent, "metadata.Extract",
		attribute.String("r", types.Stringify(r)),
	)
	defer func() { endSpan(err) }()

	// Check for cancellation before doing any work
	if err := parent.Err(); err != nil {
		return "", nil, err
	}

	// Read the input, which is removed when done, when it was copied to a
	// temporary file
	var in *input.Input
	defer func() {
		if in != nil {
			err = errors.Join(err, in.Close())
		}
	}()

	// Detect the content type, when it wasn't set, from the input
	if o.contentType == "" {
		if in, err = newInput(ctx, r, o.maxSize); err != nil {
			return "", nil, err
		}
		if o.contentType, o.params, err = ContentType(in.Reader()); err != nil {
			return "", nil, err
		}
	}
	contentType := mime.FormatMediaType(o.contentType, o.params)

	// Set the content type in the span for tracing purposes
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("contentType", contentType))

	// Check for handlers registered for this content type. If none, return
	// the title from the name of r, its kind, which is unknown, and no
	// hints, as an empty list, as there is nothing more to read of it
	entries := getEntries(o.contentType)
	if len(entries) == 0 {
		unknown := append(KindMetadata(dublincore.KindUnknown), hintMeta{})
		return contentType, o.result(r, unknown), ErrNotImplemented.With("no handler for content type ", o.contentType)
	}

	// Narrow down to the handlers that can produce one of the requested
	// namespaces. If none, return the title from the name of r
	var selected []entry
	if len(o.namespaces) > 0 {
		selected = selectEntries(entries, o.HasNamespace)
	} else {
		selected = entries
	}
	if len(selected) == 0 {
		return contentType, o.result(r, nil), nil
	}

	// Read the input, when it wasn't read to detect the content type, only
	// now there are handlers to run, so a stream isn't copied needlessly
	if in == nil {
		if in, err = newInput(ctx, r, o.maxSize); err != nil {
			return contentType, nil, err
		}
	}

	// Run the handlers which don't use the metadata of others, and then
	// those which do, with it
	first, second := splitEntries(selected)
	results, errs := run(ctx, o, in, first)
	if err := ctx.Err(); err != nil {
		return contentType, nil, err
	}
	if len(second) > 0 {
		after := *o
		after.prior = combine(slices.Clone(results))
		more, err := run(ctx, &after, in, second)
		if err := ctx.Err(); err != nil {
			return contentType, nil, err
		}
		results, errs = append(results, more...), errors.Join(errs, err)
	}

	// Return the content type, the metadata, and any errors
	return contentType, o.result(r, combine(results)), errs
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// run runs the handlers concurrently, each with its own section of the
// input, and returns their metadata, and any errors they returned, or
// nothing when the context is done first
func run(ctx context.Context, o *Opts, in *input.Input, selected []entry) ([]result, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make([]result, len(selected))
		errs    error
	)

	// Give every handler its own section of the input, so they can read it
	// independently and concurrently, and seek in it
	wg.Add(len(selected))
	for i, sel := range selected {
		go func(i int, sel entry, r io.Reader) {
			defer wg.Done()

			// Start the span, as a child of Extract's own span, named
			// for this handler (e.g. "metadata.Handler.tmdb") so it's
			// distinguishable from other handlers in a trace.
			spanName := "metadata.Handler"
			if sel.name != "" {
				spanName += "." + sel.name
			}
			var err error
			ctx, endSpan := otel.StartSpan(o.tracer, ctx, spanName)
			defer func() { endSpan(err) }()

			// Execute the handler and collect its metadata and any error it returned
			meta, err := sel.handler(ctx, r, o)

			// Collate errors and metadata
			results[i] = result{priority: sel.priority, meta: meta}
			mu.Lock()
			defer mu.Unlock()
			errs = errors.Join(errs, err)
		}(i, sel, in.Reader())
	}

	// Wait for all handlers to complete, or for the context to be done, when
	// a handler which doesn't stop, such as one which calls a C library,
	// carries on in the background, and its metadata is discarded
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Return the metadata, and any errors
	return results, errs
}

// selectEntries returns the entries which can produce a requested
// namespace, and those which can produce the namespaces which they use the
// metadata of (see AddHandlerAfter)
func selectEntries(entries []entry, requested func(string) bool) []entry {
	var selected []entry
	var after []string
	for _, entry := range entries {
		if slices.ContainsFunc(entry.namespaces, requested) {
			selected = append(selected, entry)
			after = append(after, entry.after...)
		}
	}
	if len(after) == 0 {
		return selected
	}
	for _, entry := range entries {
		if len(entry.after) == 0 && !slices.ContainsFunc(entry.namespaces, requested) && slices.ContainsFunc(entry.namespaces, func(ns string) bool {
			return slices.Contains(after, ns)
		}) {
			selected = append(selected, entry)
		}
	}
	return selected
}

// splitEntries returns the entries which don't use the metadata of others,
// and those which do (see AddHandlerAfter)
func splitEntries(entries []entry) (first, second []entry) {
	for _, entry := range entries {
		if len(entry.after) == 0 {
			first = append(first, entry)
		} else {
			second = append(second, entry)
		}
	}
	return first, second
}

// getEntries returns all registered entries matching contentType, caching
// the result until the next AddHandler call.
func getEntries(contentType string) []entry {
	handlerlock.Lock()
	defer handlerlock.Unlock()

	// Check the cache first
	if entries, ok := cached[contentType]; ok {
		return entries
	}

	// Make a list of the handlers that match the content type
	var matches []entry
	for _, entry := range handlers {
		if entry.re.MatchString(contentType) {
			matches = append(matches, entry)
		}
	}

	cached[contentType] = matches
	return matches
}

// result returns the metadata from the handlers, with the title from the
// name of r, when no handler returned a title, in the requested namespaces,
// as a handler may return others
func (o *Opts) result(r io.Reader, meta []Metadata) []Metadata {
	if named, ok := r.(NamedReader); ok {
		if title := nameTitle(named.Name()); title != "" && !slices.ContainsFunc(meta, has(dublincore.Namespace, dublincore.Title)) {
			meta = append(meta, valueMeta{dublincore.Namespace, dublincore.Title, title})
		}
	}
	return slices.DeleteFunc(meta, func(m Metadata) bool {
		return m == nil || (len(o.namespaces) > 0 && !o.HasNamespace(m.NS()))
	})
}

// combine returns the metadata of the handlers' results, of those with the
// highest priority first, and of those with the same priority in the order
// they were added, without the metadata with the same namespace and key as
// metadata from a handler with a higher priority
func combine(results []result) []Metadata {
	slices.SortStableFunc(results, func(a, b result) int {
		return cmp.Compare(b.priority, a.priority)
	})
	var meta []Metadata
	var hints hintMeta
	claimed := make(map[string]int)
	for i, r := range results {
		// The keys which handlers with the same priority claim once
		// they've all been combined
		if i > 0 && r.priority != results[i-1].priority {
			for _, m := range meta {
				if m != nil {
					if _, exists := claimed[metaKey(m)]; !exists {
						claimed[metaKey(m)] = results[i-1].priority
					}
				}
			}
		}
		for _, m := range r.meta {
			if m == nil {
				continue
			} else if hint, ok := m.(hintMeta); ok {
				// The hints of every handler, whatever its priority
				hints = append(hints, hint...)
				continue
			} else if priority, exists := claimed[metaKey(m)]; exists && priority > r.priority {
				continue
			}
			meta = append(meta, m)
		}
	}
	if len(hints) > 0 {
		slices.Sort(hints)
		meta = append(meta, slices.Compact(hints))
	}
	return meta
}

// metaKey returns the namespace and key of metadata, such as dc:title
func metaKey(m Metadata) string {
	return m.NS() + string(Separator) + m.Key()
}

// newInput returns the input of r, which each handler reads its own section
// of, and which is a bad parameter when it's larger than maxSize
func newInput(ctx context.Context, r io.Reader, maxSize int64) (*input.Input, error) {
	in, err := input.New(ctx, r, maxSize)
	if errors.Is(err, input.ErrTooLarge) {
		return nil, ErrBadParameter.Withf("input is larger than %d bytes", maxSize)
	}
	return in, err
}

// nameTitle returns a title from the name of the input, which is its
// filename without its extension, and with underscores as spaces, or an
// empty string when the input has no name
func nameTitle(name string) string {
	name = filepath.Base(filepath.ToSlash(name))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.Join(strings.Fields(strings.ReplaceAll(name, "_", " ")), " ")
}

// has returns a function which returns true when the metadata has the
// namespace and key
func has(ns, key string) func(Metadata) bool {
	return func(m Metadata) bool {
		return m != nil && m.NS() == ns && m.Key() == key
	}
}

////////////////////////////////////////////////////////////////////////////////
// VALUE METADATA

// valueMeta is metadata which Extract adds, such as the title from the
// input's name, or which a handler returns, such as its kind (see
// KindMetadata)
// hintMeta is the hints of what to read of an input next, as metadata:hint
type hintMeta []string

func (m hintMeta) NS() string         { return MetadataNamespace }
func (m hintMeta) Key() string        { return HintKey }
func (m hintMeta) Value() string      { return strings.Join(m, ",") }
func (m hintMeta) Bytes() []byte      { return []byte(m.Value()) }
func (m hintMeta) Image() image.Image { return nil }
func (m hintMeta) Any() any           { return []string(m) }

type valueMeta struct {
	ns, key, value string
}

func (m valueMeta) NS() string         { return m.ns }
func (m valueMeta) Key() string        { return m.key }
func (m valueMeta) Value() string      { return m.value }
func (m valueMeta) Bytes() []byte      { return []byte(m.value) }
func (m valueMeta) Image() image.Image { return nil }
func (m valueMeta) Any() any           { return m.value }
