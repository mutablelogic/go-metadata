package vcard_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/text/vcard"
)

const TEST_DIR = "../../testfiles"

// named is a reader with a name
type named struct {
	io.Reader
	name string
}

func (r named) Name() string { return r.name }

// entries returns the metadata of the vCard, by key, as their values, but
// without its kind
func entries(t *testing.T, card, name string, opts ...metadata.Option) map[string]any {
	t.Helper()
	var r io.Reader = strings.NewReader(card)
	if name != "" {
		r = named{r, name}
	}
	_, meta, err := metadata.Extract(context.Background(), r, append(opts, metadata.WithContentType("text/vcard"))...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]any, len(meta))
	for _, m := range meta {
		if key := m.NS() + ":" + m.Key(); key != "metadata:kind" && key != "dc:type" && key != "metadata:hint" {
			result[key] = m.Any()
		}
	}
	return result
}

func Test_vcard_000(t *testing.T) {
	// The formatted name is the title, the note the description, and the
	// UID the identifier, with the contact's details in the vcard
	// namespace, unescaped, with the lines unfolded
	card := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID:people/c1\r\n" +
		"FN:Dr. Ada King\\, Countess\r\n" +
		"N:King;Ada;;Dr.;\r\n" +
		"NICKNAME:Ada\r\n" +
		"EMAIL;TYPE=work:ada@example.com\r\n" +
		"item1.EMAIL:other@example.com\r\n" +
		"TEL;TYPE=\"voice,cell\":+44 20 7946 0000\r\n" +
		"ADR;TYPE=home:;;12 St James's Square;London;;SW1Y 4JH;UK\r\n" +
		"ORG:Analytical\\; Engines;Notes\r\n" +
		"TITLE:Translator\r\n" +
		"BDAY:--1210\r\n" +
		"URL:https://example.com\r\n" +
		"NOTE:Wrote the first program.\\nPublished in 1843 in Taylor's Scientific Mem\r\n" +
		" oirs.\r\n" +
		"END:VCARD\r\n"
	want := map[string]any{
		"dc:title":       "Dr. Ada King, Countess",
		"dc:description": "Wrote the first program.\nPublished in 1843 in Taylor's Scientific Memoirs.",
		"dc:identifier":  "people/c1",
		"vcard:email":    []string{"ada@example.com", "other@example.com"},
		"vcard:tel":      []string{"+44 20 7946 0000"},
		"vcard:url":      []string{"https://example.com"},
		"vcard:nickname": []string{"Ada"},
		"vcard:adr":      []string{"12 St James's Square, London, SW1Y 4JH, UK"},
		"vcard:org":      "Analytical; Engines, Notes",
		"vcard:title":    "Translator",
		"vcard:bday":     "--12-10",
	}
	if got := entries(t, card, "/c1.vcf"); !reflect.DeepEqual(got, want) {
		t.Errorf("expected\n%v\ngot\n%v", want, got)
	}
}

func Test_vcard_001(t *testing.T) {
	// Nicknames are a list, separated with commas which aren't escaped, and
	// URLs aren't escaped as text
	got := entries(t, "BEGIN:VCARD\r\nNICKNAME:Rob,Robbie\r\nNICKNAME:Bob\\, Jr.\r\nURL:https://example.com/a;b?q=x,y\r\nEND:VCARD\r\n", "")
	if !reflect.DeepEqual(got["vcard:nickname"], []string{"Rob", "Robbie", "Bob, Jr."}) || !reflect.DeepEqual(got["vcard:url"], []string{"https://example.com/a;b?q=x,y"}) {
		t.Errorf("unexpected %v", got)
	}
}

func Test_vcard_002(t *testing.T) {
	// Without a formatted name, the title is the name, or the file's name,
	// and not an email address
	if got := entries(t, "BEGIN:VCARD\nN:King;Ada;;;\nEND:VCARD\n", "c1.vcf")["dc:title"]; got != "Ada King" {
		t.Errorf("expected Ada King, got %v", got)
	}
	if got := entries(t, "BEGIN:VCARD\nEMAIL:x@example.com\nEND:VCARD\n", "/Contact_Card.vcf")["dc:title"]; got != "Contact Card" {
		t.Errorf("expected Contact Card, got %v", got)
	}
	if got := entries(t, "BEGIN:VCARD\nEND:VCARD\n", ""); len(got) != 0 {
		t.Errorf("expected none, got %v", got)
	}
}

func Test_vcard_003(t *testing.T) {
	// Without a note, the description is the job title and the
	// organisation, or either
	description := func(card string) any {
		return entries(t, "BEGIN:VCARD\nFN:Ada\n"+card+"END:VCARD\n", "")["dc:description"]
	}
	for card, want := range map[string]any{
		"TITLE:Translator\nORG:Analytical Engines;Notes\n": "Translator at Analytical Engines, Notes",
		"ORG:Analytical Engines\n":                         "Analytical Engines",
		"TITLE:Translator\n":                               "Translator",
		"NOTE:Wrote the first program\nTITLE:Translator\nORG:Analytical Engines\n": "Wrote the first program",
		"EMAIL:ada@example.com\n": nil,
	} {
		if got := description(card); got != want {
			t.Errorf("%q: expected %v, got %v", card, want, got)
		}
	}
}

func Test_vcard_004(t *testing.T) {
	// The first contact in a file is its details, with the number of
	// contacts, lines outside a contact are ignored, only the namespaces
	// asked for are returned, and text which isn't a vCard has only the
	// title from its name
	got := entries(t, "X-HEADER:ignored\nBEGIN:VCARD\nFN:First\nEND:VCARD\nBEGIN:VCARD\nFN:Second\nEMAIL:second@example.com\nEND:VCARD\n", "")
	if want := map[string]any{"dc:title": "First", "vcard:count": 2}; !maps.EqualFunc(got, want, func(a, b any) bool { return reflect.DeepEqual(a, b) }) {
		t.Errorf("expected %v, got %v", want, got)
	}
	card := "BEGIN:VCARD\nFN:Ada\nEMAIL:ada@example.com\nEND:VCARD\n"
	if got := entries(t, card, "", metadata.WithNamespace("dc")); !reflect.DeepEqual(got, map[string]any{"dc:title": "Ada"}) {
		t.Errorf("unexpected %v", got)
	}
	if got := entries(t, card, "", metadata.WithNamespace("vcard")); !reflect.DeepEqual(got, map[string]any{"vcard:email": []string{"ada@example.com"}}) {
		t.Errorf("unexpected %v", got)
	}
	if got := entries(t, "Just some text: with a colon\n", "notes.vcf"); !reflect.DeepEqual(got, map[string]any{"dc:title": "notes"}) {
		t.Errorf("unexpected %v", got)
	}
}

func Test_vcard_005(t *testing.T) {
	// The photo of a contact, embedded as a data URI, or as base64, is its
	// artwork, and one which is linked isn't
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	for line, want := range map[string]bool{
		"PHOTO:data:image/png;base64," + encoded:          true,
		"PHOTO;ENCODING=b;TYPE=PNG:" + encoded:            true,
		"PHOTO;VALUE=uri:https://example.com/ada.jpg":     false,
		"PHOTO:data:image/png;base64,not base64 at all!!": false,
	} {
		got := entries(t, "BEGIN:VCARD\nFN:Ada\n"+line+"\nEND:VCARD\n", "", metadata.WithNamespace(metadata.ArtworkNamespace))
		if _, exists := got["artwork:photo"]; exists != want {
			t.Errorf("%.40q: expected a photo %v, got %v", line, want, got)
		}
	}
}

func Test_vcard_006(t *testing.T) {
	// The contacts as Markdown: a heading of each name, its details as a
	// list, with their types, and its notes
	_, meta, err := metadata.Extract(context.Background(), strings.NewReader("BEGIN:VCARD\nFN:Ada King\nTITLE:Translator\nORG:Analytical Engines\nEMAIL;TYPE=work:ada@example.com\nBDAY:18151210\nNOTE:Wrote the *first* program\nEND:VCARD\nBEGIN:VCARD\nN:Hopper;Grace;;;\nEND:VCARD\n"),
		metadata.WithContentType("text/vcard"), metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if err != nil {
		t.Fatal(err)
	}
	want := "## Ada King\n\n" +
		"- **Organisation:** Translator, Analytical Engines\n" +
		"- **Email:** ada@example.com (work)\n" +
		"- **Birthday:** 1815-12-10\n" +
		"\nWrote the \\*first\\* program  \n" +
		"\n## Grace Hopper\n\n"
	for _, m := range meta {
		if m.Key() == "body" && m.Value() != want {
			t.Errorf("expected\n%q\ngot\n%q", want, m.Value())
		}
	}
}

func Test_vcard_007(t *testing.T) {
	// The anonymized contacts of Google's People API, as omodie writes
	// them, are vCards, which are contacts, with their details and photos
	files, err := filepath.Glob(filepath.Join(TEST_DIR, "vcard_*.vcf"))
	if err != nil || len(files) == 0 {
		t.Fatalf("expected test files, got %v", err)
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		contentType, meta, err := metadata.Extract(context.Background(), f, metadata.WithNamespace("dc", "vcard", "metadata", metadata.ArtworkNamespace))
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		} else if !strings.HasPrefix(contentType, "text/vcard") {
			t.Errorf("%s: expected text/vcard, got %q", path, contentType)
		}
		got := map[string]string{}
		for _, m := range meta {
			got[m.NS()+":"+m.Key()] = m.Value()
		}
		for _, key := range []string{"dc:title", "dc:identifier", "vcard:tel", "artwork:photo"} {
			if got[key] == "" && !(key == "vcard:tel" && got["vcard:email"] != "") {
				t.Errorf("%s: expected %s, got %v", filepath.Base(path), key, got)
			}
		}
		if got["metadata:kind"] != "contact" {
			t.Errorf("%s: expected the kind contact, got %q", filepath.Base(path), got["metadata:kind"])
		}
	}
}
