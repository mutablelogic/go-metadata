package acoustid

import (
	"context"
	"io"
	"regexp"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	acoustid "github.com/mutablelogic/go-metadata/pkg/acoustid"
	schema "github.com/mutablelogic/go-metadata/pkg/acoustid/schema"
)

func Test_bestResult_000(t *testing.T) {
	// The match with the highest score
	best := bestResult([]schema.Result{
		{ID: "a", Score: 0.6},
		{ID: "b", Score: 0.97},
		{ID: "c", Score: 0.4},
	})
	if best == nil || best.ID != "b" {
		t.Errorf("unexpected match %v", best)
	}
}

func Test_bestResult_001(t *testing.T) {
	// No matches, or one without an identifier, is no match
	for _, results := range [][]schema.Result{
		nil,
		{{ID: "", Score: 0.9}},
	} {
		if best := bestResult(results); best != nil {
			t.Errorf("%v: expected no match, got %v", results, best)
		}
	}
}

func Test_recordingMetadata_000(t *testing.T) {
	// The recording which most sources have, and its artists
	md := recordingMetadata(&metadata.Opts{}, []schema.Recording{
		{ID: "r1", Sources: 2, Artists: []schema.Artist{{ID: "x"}}},
		{ID: "r2", Sources: 5, Artists: []schema.Artist{{ID: "a1"}, {ID: "a2"}}},
		{ID: "", Sources: 9},
	})
	var got []string
	for _, m := range md {
		got = append(got, m.NS()+":"+m.Key()+"="+m.Value())
	}
	want := []string{"metadata:kind=recording", "dc:type=Sound", "musicbrainz:recording-id=r2", "musicbrainz:artist-id=a1", "musicbrainz:artist-id=a2"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	// A match without recordings has a note
	if md := recordingMetadata(&metadata.Opts{}, nil); len(md) != 1 || md[0].Key() != "note" {
		t.Errorf("expected a note, got %v", md)
	}
}

func Test_hasRecording(t *testing.T) {
	// A match has a recording when one has an identifier
	for _, test := range []struct {
		result schema.Result
		want   bool
	}{
		{schema.Result{ID: "a"}, false},
		{schema.Result{ID: "a", Recordings: []schema.Recording{{}}}, false},
		{schema.Result{ID: "a", Recordings: []schema.Recording{{}, {ID: "r"}}}, true},
	} {
		if got := hasRecording(&test.result); got != test.want {
			t.Errorf("%v: got %v, want %v", test.result, got, test.want)
		}
	}
}

func Test_playlist(t *testing.T) {
	// A playlist isn't looked up, even with a client
	c, err := acoustid.New("key")
	if err != nil {
		t.Fatal(err)
	}
	for _, contentType := range []string{"audio/x-mpegurl", "audio/x-scpls"} {
		_, md, err := metadata.Extract(context.Background(), strings.NewReader("#EXTM3U\nmusic/track01.mp3\n"), metadata.WithAcoustID(c), metadata.WithContentType(contentType), metadata.WithNamespace(Namespace))
		if err != nil {
			t.Errorf("%s: unexpected error %v", contentType, err)
		}
		for _, m := range md {
			if m.NS() == Namespace {
				t.Errorf("%s: unexpected %s:%s", contentType, m.NS(), m.Key())
			}
		}
	}
}

func Test_protected(t *testing.T) {
	// Audio which is protected isn't fingerprinted, which isn't an error, and
	// has a note of why it wasn't looked up, when all the namespaces are
	// read, and when only AcoustID's are, as it depends on audio's
	metadata.AddHandler(regexp.MustCompile(`^audio/x-test-protected$`), "protected", func(context.Context, io.Reader, *metadata.Opts) ([]metadata.Metadata, error) {
		return []metadata.Metadata{meta.New(audioNamespace, "drm", true)}, nil
	}, audioNamespace)
	c, err := acoustid.New("key")
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range [][]metadata.Option{nil, {metadata.WithNamespace(Namespace)}} {
		opts = append(opts, metadata.WithAcoustID(c), metadata.WithContentType("audio/x-test-protected"))
		_, md, err := metadata.Extract(context.Background(), strings.NewReader("not audio"), opts...)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		found := false
		for _, m := range md {
			if m.NS() == Namespace && m.Key() == "note" {
				found = strings.Contains(m.Value(), "protected")
			}
		}
		if !found {
			t.Errorf("expected a note that the audio is protected, got %v", md)
		}
	}
}
