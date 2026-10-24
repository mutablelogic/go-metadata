package vcard_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	// Packages
	vcard "github.com/mutablelogic/go-metadata/pkg/vcard"
)

// testdata returns the contacts of the vCards in the test files, which are
// anonymized cards of Google's People API, as omodie writes them, by name
func testdata(t *testing.T) map[string]*vcard.Card {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "testfiles", "vcard_*.vcf"))
	if err != nil || len(files) == 0 {
		t.Fatalf("expected test data, got %v", err)
	}
	result := make(map[string]*vcard.Card, len(files))
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		cards, err := vcard.Decode(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		} else if len(cards) != 1 {
			t.Fatalf("%s: expected one contact, got %d", path, len(cards))
		}
		result[filepath.Base(path)] = cards[0]
	}
	return result
}

func Test_testdata_000(t *testing.T) {
	// Each contact has a name, a UID, when it was changed, and a photo,
	// which is an image, and is the same when it's encoded and decoded
	for name, card := range testdata(t) {
		if card.DisplayName() == "" || card.Name.IsZero() || card.UID == "" || card.Rev.IsZero() {
			t.Errorf("%s: expected a name, UID and revision, got %+v", name, card)
		}
		if card.Photo == nil || card.Photo.Image() == nil {
			t.Errorf("%s: expected a photo which is an image", name)
		}
		data, err := card.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var got vcard.Card
		if err := got.UnmarshalText(data); err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(&got, card) {
			t.Errorf("%s: expected\n%+v\ngot\n%+v", name, card, &got)
		}
	}
}

func Test_testdata_001(t *testing.T) {
	// The details of the contacts, with their types
	cards := testdata(t)
	alan := cards["vcard_google1.vcf"]
	if alan.FN != "Alan Lamarr" || alan.Name != (vcard.Name{Family: "Lamarr", Given: "Alan"}) {
		t.Errorf("unexpected name %+v", alan)
	}
	if want := []vcard.Value{{Value: "alan.lamarr1@example.com", Types: []string{"home"}}}; !reflect.DeepEqual(alan.Emails, want) {
		t.Errorf("expected %+v, got %+v", want, alan.Emails)
	}
	if want := []vcard.Value{{Value: "+44 20 7946 0782", Types: []string{"mobile"}}}; !reflect.DeepEqual(alan.Phones, want) {
		t.Errorf("expected %+v, got %+v", want, alan.Phones)
	}
	if want := []vcard.Address{{Types: []string{"home"}, Street: "1 Example Street", Locality: "Sampleford", PostalCode: "EX1 1AA"}}; !reflect.DeepEqual(alan.Addresses, want) {
		t.Errorf("expected %+v, got %+v", want, alan.Addresses)
	}
	if want := []vcard.Organization{{Name: "Analytical Engines Ltd", Units: []string{"Research"}}}; !reflect.DeepEqual(alan.Orgs, want) || !reflect.DeepEqual(alan.Titles, []string{"Analyst"}) {
		t.Errorf("unexpected organisation %+v %+v", alan.Orgs, alan.Titles)
	}
	ada := cards["vcard_google2.vcf"]
	if ada.Birthday == nil || *ada.Birthday != (vcard.Date{Month: 11, Day: 28}) || !reflect.DeepEqual(ada.Nicknames, []string{"Ada"}) {
		t.Errorf("unexpected birthday or nicknames %+v %+v", ada.Birthday, ada.Nicknames)
	}
}
