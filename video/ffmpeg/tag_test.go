//go:build cgo

package ffmpeg

import (
	"image"
	"testing"
	"time"

	// Packages
	gomedia "github.com/mutablelogic/go-media"
	metadata "github.com/mutablelogic/go-metadata"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

// tag is a tag of a container
type tag struct{ key, value string }

func (t tag) Key() string        { return t.key }
func (t tag) Value() string      { return t.value }
func (t tag) Bytes() []byte      { return nil }
func (t tag) Image() image.Image { return nil }
func (t tag) Any() any           { return t.value }

// tagMap returns the metadata of the tags, by key
func tagMap(tags ...tag) map[string]metadata.Metadata {
	in := make([]gomedia.Metadata, len(tags))
	for i, t := range tags {
		in[i] = t
	}
	result := map[string]metadata.Metadata{}
	for _, m := range tagMetadata(in) {
		result[m.NS()+":"+m.Key()] = m
	}
	return result
}

func Test_tag_000(t *testing.T) {
	// The tags which are Dublin Core elements, the synopsis as the
	// description, without the tags which aren't metadata, or are repeated
	// with a language, and the vendors' tags in their namespaces
	got := tagMap(
		tag{"title", "A Film"},
		tag{"director", "Ada"},
		tag{"synopsis", "A synopsis"},
		tag{"major_brand", "isom"},
		tag{"genre", "Drama"},
		tag{"genre-eng", "Drama"},
		tag{"com.apple.quicktime.make", "Apple"},
		tag{"com.android.model", "Pixel XL"},
	)
	for key, want := range map[string]string{
		"dc:title":       "A Film",
		"dc:creator":     "Ada",
		"dc:description": "A synopsis",
		"video:genre":    "Drama",
		"quicktime:make": "Apple",
		"android:model":  "Pixel XL",
	} {
		if m := got[key]; m == nil || m.Value() != want {
			t.Errorf("%s: expected %q, got %v", key, want, m)
		}
	}
	for _, key := range []string{"video:major-brand", "video:genre-eng", "video:synopsis"} {
		if got[key] != nil {
			t.Errorf("expected no %s", key)
		}
	}

	// A description is preferred to the synopsis
	if got := tagMap(tag{"synopsis", "A synopsis"}, tag{"description", "A description"}); got["dc:description"].Value() != "A description" {
		t.Errorf("expected the description, got %q", got["dc:description"].Value())
	}
}

func Test_tag_001(t *testing.T) {
	// The date it was recorded is QuickTime's creation date, in its own time
	// zone, or else the creation time, which can be without a time zone, and
	// a creation time which is the Unix epoch isn't known
	for _, test := range []struct {
		tags []tag
		want string
	}{
		{[]tag{{"creation_time", "2018-02-11T08:40:01.000000Z"}, {"com.apple.quicktime.creationdate", "2018-02-11T09:40:01+0100"}}, "2018-02-11T09:40:01+01:00"},
		{[]tag{{"creation_time", "2018-02-11T08:40:01.000000Z"}}, "2018-02-11T08:40:01Z"},
		{[]tag{{"creation_time", "2014-10-28 11:32:43"}}, "2014-10-28T11:32:43Z"},
		{[]tag{{"creation_time", "1970-01-01T00:00:00.000000Z"}}, ""},
	} {
		got := tagMap(test.tags...)
		if m := got["dc:date"]; test.want == "" {
			if m != nil || got["video:creation-time"] != nil {
				t.Errorf("%v: expected no date, got %v", test.tags, got)
			}
		} else if m == nil || m.Value() != test.want {
			t.Errorf("%v: expected %q, got %v", test.tags, test.want, m)
		} else if _, ok := m.Any().(time.Time); !ok {
			t.Errorf("%v: expected a time", test.tags)
		}
	}
}

func Test_tag_002(t *testing.T) {
	// A location in ISO 6709, of QuickTime or of another container, is a
	// position, and dc:coverage, and one which isn't is its text
	for _, test := range []struct {
		tag tag
		key string
	}{
		{tag{"com.apple.quicktime.location.ISO6709", "+52.4158+013.5896+035.667/"}, "quicktime:location"},
		{tag{"location", "+67.8484+020.5977/"}, "video:location"},
	} {
		got := tagMap(test.tag)
		if _, ok := got[test.key].Any().(geo.LatLong); !ok {
			t.Errorf("%s: expected a position, got %v", test.key, got[test.key])
		}
		if _, ok := got["dc:coverage"].Any().(geo.LatLong); !ok {
			t.Errorf("%s: expected dc:coverage, got %v", test.key, got)
		}
	}
	if got := tagMap(tag{"location", "somewhere"}); got["video:location"].Value() != "somewhere" || got["dc:coverage"] != nil {
		t.Errorf("expected the text, got %v", got)
	}
}
