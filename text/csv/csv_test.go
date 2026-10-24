package csv_test

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/text/csv"
)

// named is a reader with a name, for its extension
type named struct {
	*strings.Reader
	name string
}

func (r named) Name() string { return r.name }

// extract returns the content type and metadata of the named values, by
// key, with the options
func extract(t *testing.T, name, data string, opts ...metadata.Option) (string, map[string]string) {
	t.Helper()
	contentType, meta, err := metadata.Extract(context.Background(), named{strings.NewReader(data), name}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		if m.NS() != "dc" || m.Key() != "title" {
			result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
		}
	}
	return contentType, result
}

// markdown returns the Markdown of the named values, in the range
func markdown(t *testing.T, name, data, r string) map[string]string {
	t.Helper()
	_, got := extract(t, name, data, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(r))
	return got
}

func Test_csv_000(t *testing.T) {
	// The rows, columns, delimiter and headers of comma-separated values
	// with a header, whose values can be quoted, with commas, quotes and
	// line breaks in them
	contentType, got := extract(t, "people.csv", "Name,Born,City\r\n"+
		"Ada Lovelace,1815,London\r\n"+
		"\"Babbage, Charles\",1791,\"London\nEngland\"\r\n"+
		"Grace \"Amazing\" Hopper,1906,New York\r\n")
	want := map[string]string{
		"metadata:kind": "data",
		"dc:type":       "Dataset",
		"csv:rows":      "3",
		"csv:columns":   "3",
		"csv:delimiter": "comma",
		"csv:headers":   "Name, Born, City",
	}
	if contentType != "text/csv; charset=utf-8" || !maps.Equal(got, want) {
		t.Errorf("got %q, %v\nwant %v", contentType, got, want)
	}
}

func Test_csv_001(t *testing.T) {
	// Values whose delimiter is detected: a bar, as in an export from a
	// database, without a header, as its first row is data, and a comma in
	// a value isn't the delimiter, and semicolons, as Excel writes them in
	// Europe, in Windows-1252, with a header
	_, got := extract(t, "export.csv", "B080001122001||PUB|1,5\r\nB080001122002||PUB|2\r\nB080001122003|X|PRV|3\r\n")
	if got["csv:delimiter"] != "bar" || got["csv:rows"] != "3" || got["csv:columns"] != "4" || got["csv:headers"] != "" {
		t.Errorf("bar: got %v", got)
	}
	_, got = extract(t, "prix.csv", "Produit;Prix\r\nCaf\xe9;2,50\r\nTh\xe9;3,00\r\n")
	if got["csv:delimiter"] != "semicolon" || got["csv:rows"] != "2" || got["csv:headers"] != "Produit, Prix" {
		t.Errorf("semicolon: got %v", got)
	}
}

func Test_csv_002(t *testing.T) {
	// Tab-separated values, whose delimiter is a tab, with a byte order
	// mark, as a spreadsheet application writes it
	contentType, got := extract(t, "scores.tsv", "\xef\xbb\xbfTeam\tScore\nRed\t12\nBlue\t9\n")
	if contentType != "text/tab-separated-values; charset=utf-8" || got["csv:delimiter"] != "tab" || got["csv:headers"] != "Team, Score" || got["csv:rows"] != "2" {
		t.Errorf("got %q, %v", contentType, got)
	}
}

func Test_csv_003(t *testing.T) {
	// A first row is a header when its values are all set, different and
	// aren't numbers, and a column of the other rows is numbers, or none is
	for _, test := range []struct {
		data    string
		headers string
	}{
		{"Name,Score\nAda,12\nGrace,9\n", "Name, Score"},
		{"Name,City\nAda,London\nGrace,New York\n", "Name, City"},
		{"1,2\n3,4\n", ""},
		{"Name,\nAda,12\n", ""},
		{"Name,Name\nAda,12\n", ""},
		{"Ada,10\nGrace,12\nAlan,9\n", ""},
		{"Name,Score\n", ""},
	} {
		if _, got := extract(t, "values.csv", test.data); got["csv:headers"] != test.headers {
			t.Errorf("%q: got %q, want %q", test.data, got["csv:headers"], test.headers)
		}
	}
}

func Test_csv_004(t *testing.T) {
	// The rows as a Markdown table, with the header, or the columns'
	// numbers, with bars escaped, and as many cells as the columns, only
	// when the markdown namespace is requested
	got := markdown(t, "values.csv", "Name,Note\nAda,a|b\nGrace\n", ":")
	if want := "| Name | Note |\n| --- | --- |\n| Ada | a\\|b |\n| Grace |  |\n"; got["markdown:body"] != want {
		t.Errorf("got %q\nwant %q", got["markdown:body"], want)
	}
	got = markdown(t, "values.csv", "1,2\n3,4,5\n", ":")
	if want := "| 1 | 2 | 3 |\n| --- | --- | --- |\n| 1 | 2 |  |\n| 3 | 4 | 5 |\n"; got["markdown:body"] != want {
		t.Errorf("got %q\nwant %q", got["markdown:body"], want)
	}
	if _, got := extract(t, "values.csv", "a,b\n1,2\n"); got["markdown:body"] != "" {
		t.Errorf("unexpected Markdown %q", got["markdown:body"])
	}
}

func Test_csv_005(t *testing.T) {
	// More rows than are read to detect the header are all counted, and
	// the Markdown is a page at a time
	var b strings.Builder
	b.WriteString("Row,Value\n")
	for i := range 100 {
		fmt.Fprintf(&b, "%d,%d\n", i, i*i)
	}
	_, got := extract(t, "values.csv", b.String())
	if got["csv:rows"] != "100" || got["csv:headers"] != "Row, Value" {
		t.Errorf("got %v", got)
	}
	got = markdown(t, "values.csv", b.String(), "0:40")
	if !strings.HasPrefix(got["markdown:body"], "| Row | Value |\n| --- | --- |\n") || got["markdown:next"] != "40" {
		t.Errorf("got %q, next %q", got["markdown:body"], got["markdown:next"])
	}
	got = markdown(t, "values.csv", b.String(), ":")
	if !strings.HasSuffix(got["markdown:body"], "| 99 | 9801 |\n") || strings.Count(got["markdown:body"], "\n") != 102 {
		t.Errorf("got %d lines", strings.Count(got["markdown:body"], "\n"))
	}
}
