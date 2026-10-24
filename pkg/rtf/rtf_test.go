package rtf_test

import (
	"reflect"
	"testing"
	"time"

	// Packages
	rtf "github.com/mutablelogic/go-metadata/pkg/rtf"
)

func Test_rtf_000(t *testing.T) {
	// The paragraphs, without the text of the header, picture and field's
	// instruction, and the properties of the info group
	doc := rtf.Parse([]byte(`{\rtf1\ansi\ansicpg1252\uc1{\fonttbl{\f0 Times;}}{\*\generator Riched20 5.40;}
{\info{\title A \'93Title\'94}{\author Ada}{\operator Charles}{\creatim\yr2004\mo6\dy24\hr18\min4}{\nofpages3}{\nofwords12}{\nofchars60}}
{\header Header\par}\pard The {\b first} caf\'e9, na\u239?ve.\par
{\pict 0123abcd}Second {\field{\*\fldinst HYPERLINK "x"}{\fldrslt link}}\line next\tab tab\par
\par One\cell Two\cell\row}`))
	want := &rtf.Document{
		Paragraphs: []string{"The first café, naïve.", "Second link\nnext\ttab", "One\tTwo"},
		Info: rtf.Info{
			Title: "A “Title”", Author: "Ada", Operator: "Charles", Generator: "Riched20 5.40",
			Created: time.Date(2004, 6, 24, 18, 4, 0, 0, time.UTC),
			Pages:   3, Words: 12, Characters: 60,
		},
	}
	if !reflect.DeepEqual(doc, want) {
		t.Errorf("got %#v\nwant %#v", doc, want)
	}
}

func Test_rtf_001(t *testing.T) {
	// A Mac document, in Mac Roman, from Cocoa, with a Unicode character
	// which is followed by two characters to skip, and escaped characters
	doc := rtf.Parse([]byte(`{\rtf1\mac\ansicpg10000\cocoartf102 Caf\'8e \uc2\u8364\'80\'80 \{\}\\\par}`))
	want := &rtf.Document{Paragraphs: []string{"Café € {}\\"}, Info: rtf.Info{Generator: "Cocoa"}}
	if !reflect.DeepEqual(doc, want) {
		t.Errorf("got %#v\nwant %#v", doc, want)
	}
}
