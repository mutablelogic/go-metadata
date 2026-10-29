package metadata_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	// Packages
	. "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

// gifHandler records the content type and header of the last reader it
// was given
type gifHandler struct {
	sync.Mutex
	contentType string
	header      []byte
}

func (h *gifHandler) Extract(_ context.Context, r io.Reader, o *Opts) ([]Metadata, error) {
	header := make([]byte, 6)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	h.Lock()
	defer h.Unlock()
	h.contentType, h.header = o.ContentType(), header
	return nil, nil
}

var gif = sync.OnceValue(func() *gifHandler {
	h := new(gifHandler)
	AddHandler(regexp.MustCompile(`^image/gif$`), "test", h.Extract)
	return h
})

func Test_metadata_000(t *testing.T) {
	// The content type is detected from a file, and the handler still reads
	// the header, without moving the file's position
	h := gif()
	f, err := os.Open(filepath.Join(TEST_DIR, "sample.gif"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, _, err := Extract(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if h.contentType != "image/gif" {
		t.Fatalf("expected image/gif, got %q", h.contentType)
	}
	if !bytes.HasPrefix(h.header, []byte("GIF8")) {
		t.Fatalf("expected a GIF header, got %q", h.header)
	}
	if pos, err := f.Seek(0, io.SeekCurrent); err != nil {
		t.Fatal(err)
	} else if pos != 0 {
		t.Fatalf("expected the file at position 0, got %d", pos)
	}
}

func Test_metadata_001(t *testing.T) {
	// The content type is detected for a reader which is not seekable, and
	// the handler still reads the header
	h := gif()
	data, err := os.ReadFile(filepath.Join(TEST_DIR, "sample.gif"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Extract(context.Background(), io.MultiReader(bytes.NewReader(data))); err != nil {
		t.Fatal(err)
	}
	if h.contentType != "image/gif" {
		t.Fatalf("expected image/gif, got %q", h.contentType)
	}
	if !bytes.HasPrefix(h.header, []byte("GIF8")) {
		t.Fatalf("expected a GIF header, got %q", h.header)
	}
}

func Test_metadata_002(t *testing.T) {
	// The content type is set, without its parameters, for a reader which
	// is not seekable
	h := gif()
	data, err := os.ReadFile(filepath.Join(TEST_DIR, "sample.gif"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Extract(context.Background(), io.MultiReader(bytes.NewReader(data)), WithContentType("Image/GIF; foo=bar")); err != nil {
		t.Fatal(err)
	}
	if h.contentType != "image/gif" {
		t.Fatalf("expected image/gif, got %q", h.contentType)
	}
	if !bytes.HasPrefix(h.header, []byte("GIF8")) {
		t.Fatalf("expected a GIF header, got %q", h.header)
	}
}

func Test_metadata_003(t *testing.T) {
	// An invalid content type is a bad parameter
	_, _, err := Extract(context.Background(), bytes.NewReader(nil), WithContentType(""))
	if !errors.Is(err, ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_metadata_004(t *testing.T) {
	// A stream is copied to a temporary file, so every handler can seek in
	// it, and the file is removed when done
	const size = 1 << 20
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)

	addStreamHandlers()
	_, meta, err := Extract(context.Background(), stream(size), WithContentType("application/x-test-stream"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keys(meta), []string{"test:1048576", "test:1048576"}; !slices.Equal(got, want) {
		t.Fatalf("expected two handlers to seek to %v, got %v", want, got)
	}
	assertEmpty(t, temp)
}

func Test_metadata_005(t *testing.T) {
	// An input larger than the maximum size is a bad parameter, and a
	// stream's temporary file is removed
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	gif()

	_, _, err := Extract(context.Background(), stream(1000), WithContentType("image/gif"), WithMaxSize(999))
	if !errors.Is(err, ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
	assertEmpty(t, temp)

	f, err := os.Open(filepath.Join(TEST_DIR, "sample.gif"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, _, err := Extract(context.Background(), f, WithMaxSize(10)); !errors.Is(err, ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}

	// An input of the maximum size is read
	addStreamHandlers()
	if _, _, err := Extract(context.Background(), stream(1000), WithContentType("application/x-test-stream"), WithMaxSize(1000)); err != nil {
		t.Fatal(err)
	}
}

func Test_metadata_006(t *testing.T) {
	// A stream isn't copied when there are no handlers for its content type
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	pr, pw := io.Pipe()
	defer pw.Close()
	_, _, err := Extract(context.Background(), pr, WithContentType("application/x-test-none"))
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
	assertEmpty(t, temp)
}

// addStreamHandlers registers handlers which seek to the end of their
// reader, and return its size as the key of their metadata
var addStreamHandlers = sync.OnceFunc(func() {
	seekEnd := func(_ context.Context, r io.Reader, _ *Opts) ([]Metadata, error) {
		seeker, ok := r.(io.ReadSeeker)
		if !ok {
			return nil, errors.New("expected a seekable reader")
		}
		n, err := seeker.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, err
		}
		return []Metadata{testMeta{"test", strconv.FormatInt(n, 10)}}, nil
	}
	re := regexp.MustCompile(`^application/x-test-stream$`)
	AddHandler(re, "a", seekEnd)
	AddHandler(re, "b", seekEnd)
})

// stream returns a reader of size zero bytes, which can't be seeked
func stream(size int64) io.Reader {
	return io.MultiReader(io.LimitReader(zeros{}, size))
}

// assertEmpty fails when dir isn't empty
func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 0 {
		t.Fatalf("expected %s to be empty, got %v", dir, entries)
	}
}

// zeros is an endless stream of zero bytes
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// testMeta is metadata in a namespace, with a key, which is the name of the
// handler which returned it
type testMeta struct {
	ns, key string
}

func (m testMeta) NS() string         { return m.ns }
func (m testMeta) Key() string        { return m.key }
func (m testMeta) Value() string      { return m.key }
func (m testMeta) Bytes() []byte      { return []byte(m.key) }
func (m testMeta) Image() image.Image { return nil }
func (m testMeta) Any() any           { return m.key }

// returns returns a handler which returns metadata with its name as the key,
// in each of the namespaces, or the test namespace when there are none, and
// err
func returns(name string, err error, namespaces ...string) HandlerFunc {
	if len(namespaces) == 0 {
		namespaces = []string{"test"}
	}
	return func(context.Context, io.Reader, *Opts) ([]Metadata, error) {
		var meta []Metadata
		for _, ns := range namespaces {
			meta = append(meta, testMeta{ns, name})
		}
		return meta, err
	}
}

// keys returns the sorted keys of the metadata, with their namespaces
func keys(meta []Metadata) []string {
	var keys []string
	for _, m := range meta {
		keys = append(keys, m.NS()+string(Separator)+m.Key())
	}
	slices.Sort(keys)
	return keys
}

// addNamespaceHandlers registers handlers for different namespaces, which
// return metadata in each of them, and an artwork handler which, like an
// expensive handler, runs only when its namespace is explicitly requested
var addNamespaceHandlers = sync.OnceFunc(func() {
	re := regexp.MustCompile(`^application/x-test-ns$`)
	AddHandler(re, "exif", returns("exif", nil, "exif"), "exif")
	AddHandler(re, "tiff", returns("tiff", nil, "tiff", "xmp"), "tiff", "xmp")
	AddHandler(re, "any", returns("any", nil))
	AddHandler(re, "artwork", func(_ context.Context, _ io.Reader, o *Opts) ([]Metadata, error) {
		if !o.HasNamespace("artwork") {
			return nil, nil
		}
		return []Metadata{testMeta{"artwork", "artwork"}}, nil
	}, "artwork")
})

func Test_metadata_007(t *testing.T) {
	// Only the handlers registered for one of the requested namespaces run,
	// and only their metadata in the requested namespaces is returned
	addNamespaceHandlers()
	tests := []struct {
		namespaces []string
		want       []string
	}{
		{nil, []string{"exif:exif", "test:any", "tiff:tiff", "xmp:tiff"}},
		{[]string{"exif"}, []string{"exif:exif"}},
		{[]string{"xmp"}, []string{"xmp:tiff"}},
		{[]string{"XMP"}, []string{"xmp:tiff"}},
		{[]string{"tiff", "exif"}, []string{"exif:exif", "tiff:tiff"}},
		{[]string{"artwork"}, []string{"artwork:artwork"}},
		{[]string{"exif", "artwork"}, []string{"artwork:artwork", "exif:exif"}},
		{[]string{"other"}, nil},
	}
	for _, test := range tests {
		opts := []Option{WithContentType("application/x-test-ns")}
		if test.namespaces != nil {
			opts = append(opts, WithNamespace(test.namespaces...))
		}
		_, meta, err := Extract(context.Background(), bytes.NewReader(nil), opts...)
		if err != nil {
			t.Fatalf("%v: %v", test.namespaces, err)
		}
		if got := keys(meta); !slices.Equal(got, test.want) {
			t.Errorf("%v: expected %v, got %v", test.namespaces, test.want, got)
		}
	}
}

func Test_metadata_008(t *testing.T) {
	// An invalid namespace is a bad parameter
	_, _, err := Extract(context.Background(), bytes.NewReader(nil), WithContentType("application/x-test-ns"), WithNamespace("not a namespace"))
	if !errors.Is(err, ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

// errA and errB are the errors returned by the error handlers
var errA, errB = errors.New("a"), errors.New("b")

// addErrorHandlers registers handlers which return metadata, two of which
// also return an error
var addErrorHandlers = sync.OnceFunc(func() {
	re := regexp.MustCompile(`^application/x-test-errors$`)
	AddHandler(re, "a", returns("a", errA))
	AddHandler(re, "b", returns("b", errB))
	AddHandler(re, "c", returns("c", nil))
})

// addUnreadableHandlers registers handlers which find the input invalid, and
// one which reads it
var addUnreadableHandlers = sync.OnceFunc(func() {
	re := regexp.MustCompile(`^application/x-test-unreadable$`)
	AddHandler(re, "truncated", returns("truncated", ErrBadParameter.With("not a pdf document: unexpected eof")))
	AddHandler(re, "failed", returns("failed", errA))
	AddHandler(re, "nested", returns("nested", errors.Join(errB, ErrBadParameter.Withf("the text of the book: %s", "too deep"))))
	AddHandler(re, "wrapped", returns("wrapped", ErrBadParameter.Withf("not an epub: %w", io.ErrUnexpectedEOF)))
	AddHandler(re, "wrapped-join", returns("wrapped-join", fmt.Errorf("handler: %w", errors.Join(ErrBadParameter.With("truncated"), errA))))
	AddHandler(re, "read", returns("read", nil))
})

func Test_metadata_unreadable(t *testing.T) {
	// A note says why the input couldn't be read, for each handler which
	// found it invalid, but not for other errors, which are returned too
	addUnreadableHandlers()

	_, meta, err := Extract(context.Background(), bytes.NewReader(nil), WithContentType("application/x-test-unreadable"))
	if !errors.Is(err, ErrBadParameter) || !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("expected the errors, got %v", err)
	}
	var notes [][]string
	for _, m := range meta {
		if m.NS() == MetadataNamespace && m.Key() == NoteKey {
			note, _ := m.Any().([]string)
			notes = append(notes, note)
		}
	}
	if len(notes) != 1 {
		t.Fatalf("expected one metadata:note, got %v", notes)
	}
	slices.Sort(notes[0])
	if want := []string{
		"It can't be read, so it may be damaged or incomplete: not a pdf document: unexpected eof",
		"It can't be read, so it may be damaged or incomplete: not an epub: unexpected EOF",
		"It can't be read, so it may be damaged or incomplete: the text of the book: too deep",
		"It can't be read, so it may be damaged or incomplete: truncated",
	}; !slices.Equal(notes[0], want) {
		t.Fatalf("expected %v, got %v", want, notes[0])
	}

	// Input which is read has no note
	addErrorHandlers()
	_, meta, _ = Extract(context.Background(), bytes.NewReader(nil), WithContentType("application/x-test-errors"))
	if slices.ContainsFunc(meta, func(m Metadata) bool { return m.Key() == NoteKey }) {
		t.Fatal("expected no metadata:note")
	}
}

func Test_metadata_009(t *testing.T) {
	// The metadata from every handler is returned, along with the errors
	// of the handlers which failed
	addErrorHandlers()

	_, meta, err := Extract(context.Background(), bytes.NewReader(nil), WithContentType("application/x-test-errors"))
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("expected errors a and b, got %v", err)
	}
	if got, want := keys(meta), []string{"test:a", "test:b", "test:c"}; !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func Test_metadata_010(t *testing.T) {
	// GetHandlers returns the handlers for a content type, and an unknown
	// content type has none, and is not implemented by Extract
	addNamespaceHandlers()
	if n := len(GetHandlers("application/x-test-ns")); n != 4 {
		t.Fatalf("expected 4 handlers, got %d", n)
	}
	if handlers := GetHandlers("application/x-test-unknown"); handlers != nil {
		t.Fatalf("expected no handlers, got %d", len(handlers))
	}
	_, meta, err := Extract(context.Background(), bytes.NewReader(nil), WithContentType("application/x-test-unknown"))
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}

	// It has no hints, as an empty list, so there's nothing more to read of
	// it, unless the metadata namespace isn't asked for
	var hints []string
	found := false
	for _, m := range meta {
		if m.NS() == MetadataNamespace && m.Key() == HintKey {
			hints, found = m.Any().([]string)
		}
	}
	if !found || hints == nil || len(hints) != 0 {
		t.Fatalf("expected an empty list of hints, got %v, %v", found, hints)
	}
	_, meta, _ = Extract(context.Background(), bytes.NewReader(nil), WithContentType("application/x-test-unknown"), WithNamespace("dc"))
	for _, m := range meta {
		if m.Key() == HintKey {
			t.Fatalf("expected no hints without the metadata namespace")
		}
	}
}

// addCancelHandler registers a handler which returns an error when it runs
var addCancelHandler = sync.OnceFunc(func() {
	AddHandler(regexp.MustCompile(`^application/x-test-cancel$`), "cancel", returns("cancel", errors.New("expected the handler not to run")))
})

func Test_metadata_011(t *testing.T) {
	// A cancelled context returns its error, without running any handler
	addCancelHandler()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Extract(ctx, bytes.NewReader(nil), WithContentType("application/x-test-cancel"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func Test_metadata_012(t *testing.T) {
	// A nil reader, regex or handler is a bad parameter
	if _, _, err := Extract(context.Background(), nil); !errors.Is(err, ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
	for _, add := range []func(){
		func() { AddHandler(nil, "nil", returns("nil", nil)) },
		func() { AddHandler(regexp.MustCompile(`.`), "nil", nil) },
	} {
		func() {
			defer func() {
				if err, ok := recover().(error); !ok || !errors.Is(err, ErrBadParameter) {
					t.Errorf("expected a panic with ErrBadParameter, got %v", err)
				}
			}()
			add()
		}()
	}
}

// addTitleHandlers registers a handler which returns no metadata, and a
// handler which titles the input
var addTitleHandlers = sync.OnceFunc(func() {
	AddHandler(regexp.MustCompile(`^application/x-test-untitled$`), "untitled", func(context.Context, io.Reader, *Opts) ([]Metadata, error) {
		return nil, nil
	}, "test")
	AddHandler(regexp.MustCompile(`^application/x-test-titled$`), "titled", returns("title", nil, "dc"), "dc")
})

func Test_metadata_013(t *testing.T) {
	// The input is titled from its name, when no handler titled it, and
	// when the title is in a requested namespace
	addTitleHandlers()
	named := func() io.Reader {
		return namedReader{bytes.NewReader(nil), filepath.Join("photos", "My_Holiday  photo.jpg")}
	}
	for _, test := range []struct {
		r           io.Reader
		contentType string
		namespaces  []string
		want        []string
	}{
		{named(), "application/x-test-untitled", nil, []string{"dc:title=My Holiday photo"}},
		{named(), "application/x-test-titled", nil, []string{"dc:title=title"}},
		{named(), "application/x-test-untitled", []string{"test"}, nil},
		{bytes.NewReader(nil), "application/x-test-untitled", nil, nil},
	} {
		opts := []Option{WithContentType(test.contentType)}
		if test.namespaces != nil {
			opts = append(opts, WithNamespace(test.namespaces...))
		}
		_, meta, err := Extract(context.Background(), test.r, opts...)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, m := range meta {
			got = append(got, m.NS()+string(Separator)+m.Key()+"="+m.Value())
		}
		if !slices.Equal(got, test.want) {
			t.Errorf("%s %v: expected %v, got %v", test.contentType, test.namespaces, test.want, got)
		}
	}
}

func Test_metadata_014(t *testing.T) {
	// The content type is returned, with its parameters, whether it was
	// detected or set
	gif()
	f, err := os.Open(filepath.Join(TEST_DIR, "sample.gif"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if contentType, _, err := Extract(context.Background(), f); err != nil {
		t.Fatal(err)
	} else if contentType != "image/gif" {
		t.Fatalf("expected image/gif, got %q", contentType)
	}
	addTitleHandlers()
	if contentType, _, err := Extract(context.Background(), bytes.NewReader(nil), WithContentType("Application/X-Test-Titled; Foo=bar")); err != nil {
		t.Fatal(err)
	} else if contentType != "application/x-test-titled; foo=bar" {
		t.Fatalf("expected application/x-test-titled; foo=bar, got %q", contentType)
	}
}

func Test_metadata_015(t *testing.T) {
	// When there's no handler for the content type, the title from the name
	// is returned, along with ErrNotImplemented, and the content type
	r := namedReader{bytes.NewReader(nil), "My_Notes.xyz"}
	contentType, meta, err := Extract(context.Background(), r, WithContentType("application/x-test-unknown"))
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
	if contentType != "application/x-test-unknown" {
		t.Fatalf("expected application/x-test-unknown, got %q", contentType)
	}
	title := slices.IndexFunc(meta, func(m Metadata) bool { return m.Key() == "title" })
	if got, want := keys(meta), []string{"dc:title", "metadata:hint", "metadata:kind"}; !slices.Equal(got, want) || title < 0 || meta[title].Value() != "My Notes" {
		t.Fatalf("expected the title My Notes, an unknown kind, and no hints, got %v", meta)
	}

	// The title isn't returned when another namespace is requested
	if _, meta, _ := Extract(context.Background(), r, WithContentType("application/x-test-unknown"), WithNamespace("exif")); len(meta) != 0 {
		t.Fatalf("expected no metadata, got %v", keys(meta))
	}
}

// addKindHandler registers a handler which returns its own kind and type
var addKindHandler = sync.OnceFunc(func() {
	AddHandler(regexp.MustCompile(`^application/x-test-kind$`), "kind", func(context.Context, io.Reader, *Opts) ([]Metadata, error) {
		return KindMetadata("form"), nil
	}, "metadata", "dc")
})

func Test_metadata_016(t *testing.T) {
	// The type and kind of the input are only those a handler returns, in
	// the requested namespaces, and not from its content type, and an input
	// no handler reads is of an unknown kind, with no type, and no hints
	addKindHandler()
	for _, test := range []struct {
		contentType string
		opts        []Option
		want        map[string]string
	}{
		{"application/x-test-unknown", nil, map[string]string{"metadata:hint": "", "metadata:kind": "unknown"}},
		{"video/mp4", nil, map[string]string{"metadata:hint": "", "metadata:kind": "unknown"}},
		{"application/x-test-kind", nil, map[string]string{"dc:type": "InteractiveResource", "metadata:kind": "form"}},
		{"application/x-test-kind", []Option{WithNamespace("dc")}, map[string]string{"dc:type": "InteractiveResource"}},
		{"application/x-test-kind", []Option{WithNamespace("metadata")}, map[string]string{"metadata:kind": "form"}},
	} {
		_, meta, _ := Extract(context.Background(), bytes.NewReader(nil), append(test.opts, WithContentType(test.contentType))...)
		got := map[string]string{}
		for _, m := range meta {
			got[m.NS()+string(Separator)+m.Key()] = m.Value()
		}
		if !maps.Equal(got, test.want) {
			t.Errorf("%s: expected %v, got %v", test.contentType, test.want, got)
		}
	}
}

func Test_metadata_017(t *testing.T) {
	// The metadata of a kind is the kind, and its DCMI type, and none for no
	// kind
	got := map[string]string{}
	for _, m := range KindMetadata("book") {
		got[m.NS()+string(Separator)+m.Key()] = m.Value()
	}
	if want := map[string]string{"metadata:kind": "book", "dc:type": "Text"}; !maps.Equal(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
	if got := KindMetadata(""); got != nil {
		t.Errorf("expected none, got %v", got)
	}
}

func Test_metadata_priority_000(t *testing.T) {
	// The metadata of the handlers with the highest priority is first, and
	// in place of the metadata with the same namespace and key from those
	// with a lower priority, but handlers with the same priority each
	// return theirs
	re := regexp.MustCompile(`^application/x-test-priority$`)
	value := func(key, value string) HandlerFunc {
		return func(context.Context, io.Reader, *Opts) ([]Metadata, error) {
			return []Metadata{meta.New("test", key, value)}, nil
		}
	}
	AddHandler(re, "low", value("thumbnail", "low"))
	AddHandler(re, "low-only", value("size", "low"))
	AddHandlerWithPriority(re, "high", 10, value("thumbnail", "high"))
	AddHandlerWithPriority(re, "low-negative", -1, value("thumbnail", "negative"))
	AddHandlerWithPriority(re, "same-a", 5, value("creator", "a"))
	AddHandlerWithPriority(re, "same-b", 5, value("creator", "b"))

	for i := 0; i < 10; i++ {
		_, result, err := Extract(context.Background(), bytes.NewReader([]byte("data")), WithContentType("application/x-test-priority"))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, m := range result {
			if m.NS() == "test" {
				got = append(got, m.Key()+"="+m.Value())
			}
		}
		want := []string{"thumbnail=high", "creator=a", "creator=b", "size=low"}
		if !slices.Equal(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func Test_metadata_priority_001(t *testing.T) {
	// The tags of every handler, whatever its priority, are combined as one
	// metadata:tag, sorted and without duplicates
	re := regexp.MustCompile(`^application/x-test-priority-tags$`)
	tags := func(tags ...string) HandlerFunc {
		return func(context.Context, io.Reader, *Opts) ([]Metadata, error) {
			return TagMetadata(tags...), nil
		}
	}
	AddHandler(re, "low", tags("recurring", "event"))
	AddHandlerWithPriority(re, "high", 10, tags("inbox", "event"))
	AddHandlerWithPriority(re, "same-a", 5, tags("work", "birthday"))
	AddHandlerWithPriority(re, "same-b", 5, tags("birthday", "all-day"))

	for i := 0; i < 10; i++ {
		_, result, err := Extract(context.Background(), bytes.NewReader([]byte("data")), WithContentType("application/x-test-priority-tags"))
		if err != nil {
			t.Fatal(err)
		}
		var got [][]string
		for _, m := range result {
			if m.NS() == MetadataNamespace && m.Key() == TagKey {
				tags, _ := m.Any().([]string)
				got = append(got, tags)
			}
		}
		want := []string{"all-day", "birthday", "event", "inbox", "recurring", "work"}
		if len(got) != 1 || !slices.Equal(got[0], want) {
			t.Fatalf("expected one metadata:tag of %v, got %v", want, got)
		}
	}
}

func Test_metadata_018(t *testing.T) {
	// When the context is done while a handler which doesn't stop is
	// running, Extract returns at once, with the context's error, and the
	// handler carries on in the background
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	AddHandler(regexp.MustCompile(`^application/x-test-blocking$`), "blocking", func(context.Context, io.Reader, *Opts) ([]Metadata, error) {
		close(started)
		<-release
		return KindMetadata("data"), nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()
	done := make(chan error, 1)
	go func() {
		_, meta, err := Extract(ctx, bytes.NewReader([]byte("data")), WithContentType("application/x-test-blocking"))
		if meta != nil {
			err = errors.Join(err, errors.New("expected no metadata"))
		}
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("expected Extract to return when the context is done")
	}
}

// addAfterHandlers registers a handler which returns a duration, and two
// which run after it: one returns the duration as its own metadata, and the
// other returns whether it saw the metadata of the first
var addAfterHandlers = sync.OnceFunc(func() {
	re := regexp.MustCompile(`^application/x-test-after$`)
	AddHandler(re, "first", func(context.Context, io.Reader, *Opts) ([]Metadata, error) {
		time.Sleep(10 * time.Millisecond)
		return []Metadata{meta.New("video", "duration", time.Minute)}, nil
	}, "video")
	AddHandlerAfter(re, "after", []string{"video"}, func(_ context.Context, _ io.Reader, o *Opts) ([]Metadata, error) {
		m := o.Get("video", "duration")
		if m == nil {
			return nil, nil
		}
		return []Metadata{meta.New("after", "duration", m.Any())}, nil
	}, "after")
	AddHandlerAfter(re, "other", []string{"after"}, func(_ context.Context, _ io.Reader, o *Opts) ([]Metadata, error) {
		return []Metadata{meta.New("other", "seen", o.Get("after", "duration") != nil)}, nil
	}, "other")
})

func Test_metadata_after_000(t *testing.T) {
	// A handler added with AddHandlerAfter gets the metadata of the
	// handlers which ran first, which run even when their namespaces
	// weren't requested, but whose metadata isn't returned. It doesn't get
	// the metadata of another handler added with AddHandlerAfter.
	addAfterHandlers()
	tests := []struct {
		namespaces []string
		want       map[string]string
	}{
		{nil, map[string]string{"video:duration": "1m0s", "after:duration": "1m0s", "other:seen": "false"}},
		{[]string{"after"}, map[string]string{"after:duration": "1m0s"}},
		{[]string{"other"}, map[string]string{"other:seen": "false"}},
	}
	for _, test := range tests {
		opts := []Option{WithContentType("application/x-test-after")}
		if test.namespaces != nil {
			opts = append(opts, WithNamespace(test.namespaces...))
		}
		_, meta, err := Extract(context.Background(), bytes.NewReader(nil), opts...)
		if err != nil {
			t.Fatalf("%v: %v", test.namespaces, err)
		}
		got := make(map[string]string, len(meta))
		for _, m := range meta {
			got[m.NS()+string(Separator)+m.Key()] = m.Value()
		}
		if !maps.Equal(got, test.want) {
			t.Errorf("%v: expected %v, got %v", test.namespaces, test.want, got)
		}
	}
}

// tmdbOpts are the options which the TMDB test handler was last given
var tmdbOpts struct {
	sync.Mutex
	*Opts
}

// addTMDBHandler registers a handler which records its options
var addTMDBHandler = sync.OnceFunc(func() {
	AddHandler(regexp.MustCompile(`^application/x-test-tmdb$`), "tmdb", func(_ context.Context, _ io.Reader, o *Opts) ([]Metadata, error) {
		tmdbOpts.Lock()
		defer tmdbOpts.Unlock()
		tmdbOpts.Opts = o
		return nil, nil
	})
})

func Test_metadata_tmdb_000(t *testing.T) {
	addTMDBHandler()
	// A movie or an episode on TMDB is valid, and replaces the other
	for _, test := range []struct {
		opts                  []Option
		err                   bool
		movie                 uint64
		show, season, episode uint64
	}{
		{opts: []Option{WithTMDBMovie(603)}, movie: 603},
		{opts: []Option{WithTMDBEpisode(1396, 1, 2)}, show: 1396, season: 1, episode: 2},
		{opts: []Option{WithTMDBEpisode(1396, 0, 1)}, show: 1396, season: 0, episode: 1},
		{opts: []Option{WithTMDBMovie(603), WithTMDBEpisode(1396, 1, 2)}, show: 1396, season: 1, episode: 2},
		{opts: []Option{WithTMDBEpisode(1396, 1, 2), WithTMDBMovie(603)}, movie: 603},
		{opts: []Option{WithTMDBMovie(0)}, err: true},
		{opts: []Option{WithTMDBEpisode(0, 1, 2)}, err: true},
		{opts: []Option{WithTMDBEpisode(1396, 1, 0)}, err: true},
	} {
		opts := append(test.opts, WithContentType("application/x-test-tmdb"))
		_, _, err := Extract(context.Background(), bytes.NewReader(nil), opts...)
		if test.err {
			if !errors.Is(err, ErrBadParameter) {
				t.Errorf("%v: expected ErrBadParameter, got %v", test, err)
			}
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		tmdbOpts.Lock()
		o := tmdbOpts.Opts
		tmdbOpts.Unlock()
		show, season, episode := o.TMDBEpisode()
		if o.TMDBMovie() != test.movie || show != test.show || season != test.season || episode != test.episode {
			t.Errorf("%v: got movie %d, episode %d/%d/%d", test, o.TMDBMovie(), show, season, episode)
		}
	}
}

// addSignatures registers the signatures of two test formats, one of which
// starts with the other
var addSignatures = sync.OnceFunc(func() {
	for prefix, contentType := range map[string]string{
		"\x00TST":     "application/x-test-signature",
		"\x00TST\x02": "application/x-test-signature-2",
		"TSTTEXT":     "application/x-test-text",
	} {
		if err := AddSignature([]byte(prefix), contentType); err != nil {
			panic(err)
		}
	}
})

// signatureReader is a reader with a name, for its extension
type signatureReader struct {
	*bytes.Reader
	name string
}

func (r signatureReader) Name() string { return r.name }

func Test_signature_000(t *testing.T) {
	// Binary content is the type of the longest signature it starts with,
	// rather than of its extension, and text, or other binary content, is
	// as it was
	addSignatures()
	for _, test := range []struct {
		data, name, want string
	}{
		{"\x00TST\x01\x00\x00", "", "application/x-test-signature"},
		{"\x00TST\x02\x00\x00", "", "application/x-test-signature-2"},
		{"\x00TST\x01\x00\x00", "file.pdf", "application/x-test-signature"},
		{"\x00XYZ\x01\x00\x00", "file.pdf", "application/pdf"},
		{"\x00XYZ\x01\x00\x00", "", "application/octet-stream"},
		{"TST is plain text", "", "text/plain"},

		// Text which starts with a signature is its type, whatever its
		// extension
		{"TSTTEXT is a test document", "", "application/x-test-text"},
		{"TSTTEXT is a test document", "report.doc", "application/x-test-text"},
		{"Not TSTTEXT", "report.json", "application/json"},
	} {
		contentType, _, err := ContentType(signatureReader{bytes.NewReader([]byte(test.data)), test.name})
		if err != nil {
			t.Fatal(err)
		} else if contentType != test.want {
			t.Errorf("%q %q: got %q, want %q", test.data, test.name, contentType, test.want)
		}
	}
}

func Test_signature_001(t *testing.T) {
	// A signature which is empty, or too long, or whose content type isn't
	// one, is a bad parameter
	for _, test := range []struct {
		prefix      []byte
		contentType string
	}{
		{nil, "application/x-test"},
		{make([]byte, 513), "application/x-test"},
		{[]byte("XYZ"), "not a content type;;"},
	} {
		if err := AddSignature(test.prefix, test.contentType); !errors.Is(err, ErrBadParameter) {
			t.Errorf("%q %q: expected ErrBadParameter, got %v", test.prefix, test.contentType, err)
		}
	}
}

func Test_ogg_000(t *testing.T) {
	// Ogg, which is sniffed as application/ogg, is refined by its extension,
	// as audio or video, and is application/ogg otherwise
	data := "OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00"
	for name, want := range map[string]string{
		"voice.opus":  "audio/ogg",
		"song.ogg":    "audio/ogg",
		"speech.spx":  "audio/ogg",
		"clip.ogv":    "video/ogg",
		"recording":   "application/ogg",
		"archive.zip": "application/ogg",
	} {
		contentType, _, err := ContentType(signatureReader{bytes.NewReader([]byte(data)), name})
		if err != nil {
			t.Fatal(err)
		} else if contentType != want {
			t.Errorf("%s: got %q, want %q", name, contentType, want)
		}
	}
}

func Test_metadata_tags(t *testing.T) {
	// Tags are identifiers in lower case, without duplicates, and those which
	// aren't identifiers are left out
	meta := TagMetadata(" Event", "event", "1bad", "out-of-office", "", "two words")
	if len(meta) != 1 || meta[0].NS() != MetadataNamespace || meta[0].Key() != TagKey {
		t.Fatalf("expected metadata:tag, got %v", meta)
	}
	if got, ok := meta[0].Any().([]string); !ok || !slices.Equal(got, []string{"event", "out-of-office"}) {
		t.Fatalf("expected [event out-of-office], got %v", meta[0].Any())
	}
	if meta := TagMetadata("", "1bad"); meta != nil {
		t.Fatalf("expected no tags, got %v", meta)
	}
}
