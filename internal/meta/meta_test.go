package meta_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	// Packages
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

func Test_meta_000(t *testing.T) {
	// The namespace and key are separate
	m := meta.New("dc", "title", "Title")
	if m == nil {
		t.Fatal("expected an entry")
	}
	if m.NS() != "dc" {
		t.Fatalf("expected dc, got %q", m.NS())
	}
	if m.Key() != "title" {
		t.Fatalf("expected title, got %q", m.Key())
	}
}

func Test_meta_001(t *testing.T) {
	// A namespace or key which isn't an identifier has no entry
	for _, test := range []struct{ ns, key string }{
		{"", "title"},
		{"dc", ""},
		{"1dc", "title"},
		{"dc", "og:title"},
		{"dc", "has space"},
	} {
		if m := meta.New(test.ns, test.key, "value"); m != nil {
			t.Errorf("%q %q: expected no entry, got %q", test.ns, test.key, m.Key())
		}
	}
}

func Test_meta_002(t *testing.T) {
	// The value is returned as a string, and as bytes of the string, which
	// is RFC3339 for a time, and the string form of a duration
	for _, test := range []struct {
		value any
		want  string
	}{
		{"Title", "Title"},
		{"", ""},
		{time.Date(2026, 10, 24, 12, 30, 0, 0, time.UTC), "2026-10-24T12:30:00Z"},
		{time.Date(2026, 10, 24, 12, 30, 0, 0, time.FixedZone("", 3600)), "2026-10-24T12:30:00+01:00"},
		{90 * time.Second, "1m30s"},
		{1500 * time.Millisecond, "1.5s"},
		{time.Duration(0), "0s"},
		{42, "42"},
		{1.5, "1.5"},
		{true, "true"},
	} {
		m := meta.New("test", "value", test.value)
		if got := m.Value(); got != test.want {
			t.Errorf("%#v: expected %q, got %q", test.value, test.want, got)
		}
		if got := string(m.Bytes()); got != test.want {
			t.Errorf("%#v: expected bytes %q, got %q", test.value, test.want, got)
		}
	}
}

func Test_meta_003(t *testing.T) {
	// The value is returned as is, and isn't an image
	m := meta.New("test", "duration", 90*time.Second)
	if d, ok := m.Any().(time.Duration); !ok || d != 90*time.Second {
		t.Fatalf("expected a duration of 1m30s, got %#v", m.Any())
	}
	if m.Image() != nil {
		t.Fatal("expected no image")
	}
}

func Test_meta_004(t *testing.T) {
	// The value is marshalled as JSON, with the string form of a duration,
	// and the characters which are HTML's as they are, with an encoder which
	// doesn't escape them
	for _, test := range []struct {
		value any
		want  string
	}{
		{"Title", `"Title"`},
		{90 * time.Second, `"1m30s"`},
		{time.Date(2026, 10, 24, 12, 30, 0, 0, time.UTC), `"2026-10-24T12:30:00Z"`},
		{42, `42`},
		{true, `true`},
		{[]string{"a", "b"}, `["a","b"]`},
		{"Fish & chips <hot>", `"Fish & chips <hot>"`},
	} {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(meta.New("test", "value", test.value)); err != nil {
			t.Fatalf("%#v: %v", test.value, err)
		}
		if data := strings.TrimSpace(buf.String()); data != test.want {
			t.Errorf("%#v: expected %s, got %s", test.value, test.want, data)
		}
	}
}

func Test_meta_005(t *testing.T) {
	// An entry with text is the text as its value and bytes, and its value
	// as any and in JSON, and the value itself without text
	m := meta.NewText("exif", "FNumber", 2.8, "f/2.8")
	if m.Value() != "f/2.8" || string(m.Bytes()) != "f/2.8" || m.Any() != 2.8 {
		t.Errorf("expected f/2.8 and 2.8, got %q %v", m.Value(), m.Any())
	}
	if data, err := json.Marshal(m); err != nil || string(data) != "2.8" {
		t.Errorf("expected 2.8 in JSON, got %s %v", data, err)
	}
	if m := meta.NewText("exif", "FNumber", 2.8, ""); m.Value() != "2.8" {
		t.Errorf("expected 2.8, got %q", m.Value())
	}
	if m := meta.NewText("exif", "not a key", 2.8, "f/2.8"); m != nil {
		t.Errorf("expected nil, got %v", m)
	}
}
