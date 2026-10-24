package iptc_test

import (
	"bytes"
	"testing"
	"time"

	// Packages
	iptc "github.com/mutablelogic/go-metadata/pkg/iptc"
)

// dataset returns a dataset as IPTC metadata
func dataset(record, number uint8, data string) []byte {
	return append([]byte{0x1C, record, number, byte(len(data) >> 8), byte(len(data))}, data...)
}

func Test_iptc_000(t *testing.T) {
	// The datasets of the application record are metadata, with repeated
	// datasets as a list, and the date and time it was created
	data := bytes.Join([][]byte{
		dataset(1, 90, "\x1b%G"),
		dataset(2, 0, "\x00\x04"),
		dataset(2, 5, "Sample"),
		dataset(2, 25, "one"),
		dataset(2, 25, "two"),
		dataset(2, 80, "Ada Lovelace"),
		dataset(2, 120, "Café"),
		dataset(2, 55, "20240115"),
		dataset(2, 60, "103000+0100"),
		dataset(2, 116, "© 2024"),
	}, nil)
	got := map[string]string{}
	var created time.Time
	for _, m := range iptc.Metadata(iptc.Parse(data)) {
		got[m.NS()+":"+m.Key()] = m.Value()
		if m.Key() == "DateCreated" {
			created, _ = m.Any().(time.Time)
		}
	}
	for key, want := range map[string]string{
		"iptc:ObjectName":      "Sample",
		"iptc:Keywords":        "one; two",
		"iptc:Byline":          "Ada Lovelace",
		"iptc:Caption":         "Café",
		"iptc:CopyrightNotice": "© 2024",
		"iptc:DateCreated":     "2024-01-15T10:30:00+01:00",
	} {
		if got[key] != want {
			t.Errorf("%s: expected %q, got %q", key, want, got[key])
		}
	}
	if !created.Equal(time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("expected a time, got %v", created)
	}
	if len(got) != 6 {
		t.Errorf("expected six fields, got %v", got)
	}
}

func Test_iptc_001(t *testing.T) {
	// Text which isn't UTF-8 is ISO 8859-1, a date without a time is the
	// date, and parsing stops at data which isn't a dataset
	data := bytes.Join([][]byte{
		dataset(2, 120, "Caf\xe9"),
		dataset(2, 55, "20240115"),
		[]byte("junk"),
		dataset(2, 5, "after the junk"),
	}, nil)
	got := map[string]string{}
	for _, m := range iptc.Metadata(iptc.Parse(data)) {
		got[m.NS()+":"+m.Key()] = m.Value()
	}
	if got["iptc:Caption"] != "Café" || got["iptc:DateCreated"] != "2024-01-15T00:00:00Z" || got["iptc:ObjectName"] != "" {
		t.Errorf("unexpected %v", got)
	}

	// A dataset which is cut short isn't returned, but those before it are
	data = append(dataset(2, 5, "Sample"), dataset(2, 120, "a caption")[:7]...)
	if datasets := iptc.Parse(data); len(datasets) != 1 || string(datasets[0].Data) != "Sample" {
		t.Errorf("expected one dataset, got %v", datasets)
	}
}
