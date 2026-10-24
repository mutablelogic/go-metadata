package acoustid

import (
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
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
	want := []string{"musicbrainz:recording-id=r2", "musicbrainz:artist-id=a1", "musicbrainz:artist-id=a2"}
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
