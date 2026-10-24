package contentline_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	// Packages
	contentline "github.com/mutablelogic/go-metadata/internal/contentline"
)

func Test_contentline_000(t *testing.T) {
	// A text value is escaped and unescaped, and split at separators which
	// aren't escaped
	for _, value := range []string{"plain", `a\b`, "a,b;c", "line\nbreak", ""} {
		if got := contentline.Unescape(contentline.Escape(value)); got != value {
			t.Errorf("%q: got %q", value, got)
		}
	}
	if got := contentline.Escape("a,b;c\r\nd"); got != `a\,b\;c\nd` {
		t.Errorf("unexpected %q", got)
	}
	if got := contentline.Split(`Lovelace;Ada\;Augusta;;Countess\, of Lovelace;`, ';'); !reflect.DeepEqual(got, []string{"Lovelace", "Ada;Augusta", "", "Countess, of Lovelace", ""}) {
		t.Errorf("unexpected %q", got)
	}
}

func Test_contentline_001(t *testing.T) {
	// A long line is folded at 75 octets, without splitting a character,
	// and unfolded as it was
	value := strings.Repeat("é", 100)
	var buf bytes.Buffer
	w := contentline.NewWriter(&buf)
	if err := w.Write("NOTE", nil, value); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(buf.String(), "\r\n"), "\r\n") {
		if len(line) > 75 || !utf8.ValidString(line) {
			t.Errorf("line of %d octets, or which splits a character: %q", len(line), line)
		}
	}
	var lines []string
	if err := contentline.Unfold(&buf, 1<<20, func(line string) error { lines = append(lines, line); return nil }); err != nil {
		t.Fatal(err)
	} else if len(lines) != 1 || lines[0] != "NOTE:"+value {
		t.Errorf("unexpected %q", lines)
	}
}

func Test_contentline_002(t *testing.T) {
	// A line is a property's group, name, parameters and value, whose value
	// starts after the first colon which isn't quoted, and a parameter
	// without a name is a TYPE
	for _, test := range []struct {
		line string
		want contentline.Property
	}{
		{"FN:Ada Lovelace", contentline.Property{Name: "FN", Value: "Ada Lovelace"}},
		{"item1.EMAIL;type=INTERNET,pref:ada@example.com", contentline.Property{Group: "ITEM1", Name: "EMAIL", Params: map[string][]string{"TYPE": {"INTERNET", "pref"}}, Value: "ada@example.com"}},
		{"TEL;HOME;VOICE:+44 20 7946 0000", contentline.Property{Name: "TEL", Params: map[string][]string{"TYPE": {"HOME", "VOICE"}}, Value: "+44 20 7946 0000"}},
		{`ADR;LABEL="1 Main St: Flat 2";TYPE=home:;;1 Main St;London;;;UK`, contentline.Property{Name: "ADR", Params: map[string][]string{"LABEL": {"1 Main St: Flat 2"}, "TYPE": {"home"}}, Value: ";;1 Main St;London;;;UK"}},
		{"URL:https://example.com/a:b", contentline.Property{Name: "URL", Value: "https://example.com/a:b"}},
	} {
		got, ok := contentline.Parse(test.line)
		if !ok || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%q: expected %+v, got %+v", test.line, test.want, got)
		}
	}
	if _, ok := contentline.Parse("not a property"); ok {
		t.Error("expected no property")
	}
}
