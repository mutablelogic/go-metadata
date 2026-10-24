package vcard_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"reflect"
	"strings"
	"testing"
	"time"

	// Packages
	vcard "github.com/mutablelogic/go-metadata/pkg/vcard"
)

// ada is a contact, as omodie's Google backend writes it
var ada = &vcard.Card{
	UID:       "people/c1",
	FN:        "Dr. Ada King, Countess",
	Name:      vcard.Name{Family: "King", Given: "Ada", Prefix: "Dr."},
	Nicknames: []string{"Ada"},
	Emails:    []vcard.Value{{Value: "ada@example.com", Types: []string{"Work"}}, {Value: "other@example.com", Types: []string{"a custom type"}}},
	URLs:      []vcard.Value{{Value: "https://example.com", Types: []string{"homePage"}}},
	Addresses: []vcard.Address{{Types: []string{"home"}, Street: "12 St James's Square", Locality: "London", PostalCode: "SW1Y 4JH", Country: "UK"}, {Types: []string{"work"}}},
	Orgs:      []vcard.Organization{{Name: "Analytical; Engines", Units: []string{"Notes"}}},
	Titles:    []string{"Translator", "Mathematician"},
	Birthday:  &vcard.Date{Month: 12, Day: 10},
	Notes:     []string{"Wrote the first program.\nPublished 1843."},
	Rev:       time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC),
}

func Test_encode_000(t *testing.T) {
	// A contact is vCard 4.0, with each value escaped, and its types as
	// parameters, when they can be, and values which are empty aren't written
	data, err := ada.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	want := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID;VALUE=text:people/c1\r\n" +
		"FN:Dr. Ada King\\, Countess\r\n" +
		"N:King;Ada;;Dr.;\r\n" +
		"NICKNAME:Ada\r\n" +
		"EMAIL;TYPE=work:ada@example.com\r\n" +
		"EMAIL:other@example.com\r\n" +
		"URL;TYPE=homepage:https://example.com\r\n" +
		"ADR;TYPE=home:;;12 St James's Square;London;;SW1Y 4JH;UK\r\n" +
		"ORG:Analytical\\; Engines;Notes\r\n" +
		"TITLE:Translator\r\n" +
		"TITLE:Mathematician\r\n" +
		"BDAY:--1210\r\n" +
		"NOTE:Wrote the first program.\\nPublished 1843.\r\n" +
		"REV:20261018T093000Z\r\n" +
		"END:VCARD\r\n"
	if string(data) != want {
		t.Errorf("expected\n%s\ngot\n%s", want, data)
	}
}

func Test_encode_001(t *testing.T) {
	// The formatted name is the display name, a birthday with a year has
	// it, a URL isn't escaped as text but can't add a property, and a
	// preferred value is PREF
	data, err := (&vcard.Card{
		Emails:   []vcard.Value{{Value: "x@example.com", Pref: true}},
		Birthday: &vcard.Date{Year: 1815, Month: 12, Day: 10},
		URLs:     []vcard.Value{{Value: "https://example.com/a;b?q=x,y"}, {Value: "https://example.com/\r\nEMAIL:evil@example.com"}},
	}).MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\r\nFN:x@example.com\r\n",
		"\r\nEMAIL;PREF=1:x@example.com\r\n",
		"\r\nBDAY:18151210\r\n",
		"\r\nURL:https://example.com/a;b?q=x,y\r\n",
		"\r\nURL:https://example.com/%0D%0AEMAIL:evil@example.com\r\n",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in\n%s", want, data)
		}
	}
	for _, unwanted := range []string{"\r\nN:", "REV:", "\r\nEMAIL:evil"} {
		if strings.Contains(string(data), unwanted) {
			t.Errorf("expected no %q in\n%s", unwanted, data)
		}
	}
}

func Test_decode_000(t *testing.T) {
	// A contact which is encoded is decoded as it was, with its types in
	// lower case, and without the parts which are empty
	data, err := ada.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var got vcard.Card
	if err := got.UnmarshalText(data); err != nil {
		t.Fatal(err)
	}
	want := *ada
	want.Emails = []vcard.Value{{Value: "ada@example.com", Types: []string{"work"}}, {Value: "other@example.com"}}
	want.URLs = []vcard.Value{{Value: "https://example.com", Types: []string{"homepage"}}}
	want.Addresses = want.Addresses[:1]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected\n%+v\ngot\n%+v", want, got)
	}
}

func Test_decode_001(t *testing.T) {
	// vCard 3.0, with groups, types as a list and as a pref, quoted
	// parameters, folded lines, a list of nicknames, and a date with
	// hyphens, and vCard 2.1, with types without names, and quoted-printable
	// text, and more than one contact
	data := "BEGIN:VCARD\r\n" +
		"VERSION:3.0\r\n" +
		"FN:Grace Hopper\r\n" +
		"N:Hopper;Grace;Brewster Murray;Rear Admiral;\r\n" +
		"item1.EMAIL;TYPE=INTERNET,pref:grace@example.com\r\n" +
		"TEL;TYPE=\"voice,cell\":+1 555 0100\r\n" +
		"NICKNAME:Amazing Grace,Grandma COBOL\r\n" +
		"BDAY:1906-12-09\r\n" +
		"NOTE:Found the first\r\n" +
		"  bug.\r\n" +
		"CATEGORIES:navy,computing\r\n" +
		"END:VCARD\r\n" +
		"BEGIN:VCARD\r\n" +
		"VERSION:2.1\r\n" +
		"N:Turing;Alan\r\n" +
		"TEL;HOME;VOICE:+44 20 7946 0001\r\n" +
		"NOTE;ENCODING=QUOTED-PRINTABLE;CHARSET=UTF-8:Caf=C3=A9 owner\r\n" +
		"END:VCARD\r\n"
	cards, err := vcard.Decode(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	} else if len(cards) != 2 {
		t.Fatalf("expected two contacts, got %d", len(cards))
	}
	grace, alan := cards[0], cards[1]
	if grace.DisplayName() != "Grace Hopper" || grace.Name.Middle != "Brewster Murray" || grace.Name.Prefix != "Rear Admiral" {
		t.Errorf("unexpected name %+v", grace)
	}
	if want := []vcard.Value{{Value: "grace@example.com", Types: []string{"internet"}, Pref: true}}; !reflect.DeepEqual(grace.Emails, want) {
		t.Errorf("expected %+v, got %+v", want, grace.Emails)
	}
	if want := []vcard.Value{{Value: "+1 555 0100", Types: []string{"voice", "cell"}}}; !reflect.DeepEqual(grace.Phones, want) {
		t.Errorf("expected %+v, got %+v", want, grace.Phones)
	}
	if !reflect.DeepEqual(grace.Nicknames, []string{"Amazing Grace", "Grandma COBOL"}) || !reflect.DeepEqual(grace.Categories, []string{"navy", "computing"}) {
		t.Errorf("unexpected lists %+v", grace)
	}
	if grace.Birthday == nil || *grace.Birthday != (vcard.Date{Year: 1906, Month: 12, Day: 9}) {
		t.Errorf("unexpected birthday %+v", grace.Birthday)
	}
	if !reflect.DeepEqual(grace.Notes, []string{"Found the first bug."}) {
		t.Errorf("unexpected notes %q", grace.Notes)
	}
	if alan.DisplayName() != "Alan Turing" || !reflect.DeepEqual(alan.Phones, []vcard.Value{{Value: "+44 20 7946 0001", Types: []string{"home", "voice"}}}) || !reflect.DeepEqual(alan.Notes, []string{"Café owner"}) {
		t.Errorf("unexpected %+v", alan)
	}
}

func Test_photo_000(t *testing.T) {
	// A photo is embedded as a data URI, in vCard 4.0, and as base64, with
	// its type, in vCard 3.0, and decoded as an image, and a link is a URI
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	for _, test := range []struct {
		line string
		want vcard.Photo
	}{
		{"PHOTO:data:image/png;base64," + encoded, vcard.Photo{MediaType: "image/png", Data: buf.Bytes()}},
		{"PHOTO;ENCODING=b;TYPE=PNG:" + encoded, vcard.Photo{MediaType: "image/png", Data: buf.Bytes()}},
		{"PHOTO;ENCODING=BASE64:" + encoded, vcard.Photo{MediaType: "image/png", Data: buf.Bytes()}},
		{"PHOTO;VALUE=uri:https://example.com/ada.jpg", vcard.Photo{URI: "https://example.com/ada.jpg"}},
	} {
		cards, err := vcard.Decode(strings.NewReader("BEGIN:VCARD\r\nFN:Ada\r\n" + test.line + "\r\nEND:VCARD\r\n"))
		if err != nil || len(cards) != 1 || cards[0].Photo == nil {
			t.Fatalf("%q: expected a photo, got %v %v", test.line, cards, err)
		}
		if photo := cards[0].Photo; !reflect.DeepEqual(*photo, test.want) {
			t.Errorf("%q: expected %+v, got %+v", test.line, test.want, *photo)
		} else if (photo.Image() != nil) != (test.want.URI == "") {
			t.Errorf("%q: expected an image only when it's embedded", test.line)
		}
	}

	// An embedded photo is encoded as a data URI, and decoded as it was
	data, err := (&vcard.Card{FN: "Ada", Photo: &vcard.Photo{Data: buf.Bytes()}}).MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var got vcard.Card
	if err := got.UnmarshalText(data); err != nil {
		t.Fatal(err)
	} else if got.Photo == nil || !bytes.Equal(got.Photo.Data, buf.Bytes()) || got.Photo.MediaType != "image/png" {
		t.Errorf("unexpected photo %+v", got.Photo)
	}
}

func Test_decode_002(t *testing.T) {
	// Text without a contact is none, which isn't an error, and a contact
	// which isn't ended is returned
	if cards, err := vcard.Decode(strings.NewReader("not a vcard\r\n")); err != nil || len(cards) != 0 {
		t.Errorf("expected none, got %v %v", cards, err)
	}
	if cards, err := vcard.Decode(strings.NewReader("BEGIN:VCARD\r\nFN:Ada\r\n")); err != nil || len(cards) != 1 || cards[0].FN != "Ada" {
		t.Errorf("expected Ada, got %v %v", cards, err)
	}
	var card vcard.Card
	if err := card.UnmarshalText([]byte("nothing")); err == nil {
		t.Error("expected an error")
	}
}
