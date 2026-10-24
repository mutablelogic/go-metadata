package mobi

import (
	"bytes"
	"testing"
)

func Test_text_000(t *testing.T) {
	// PalmDOC decompression: bytes as themselves, a run of bytes after its
	// count, a space and a character, and a back-reference
	for _, test := range []struct {
		data []byte
		want string
	}{
		{[]byte("plain"), "plain"},
		{[]byte{0x02, 0xE9, 0xFF, 'a'}, "\xe9\xffa"},
		{[]byte{'a', 0xC2 ^ 0x80 | 0x80, 'c'}, "a Bc"},
		// "abc", then 3 bytes from 3 back, then 6 bytes from 3 back, which
		// overlap what's copied
		{[]byte{'a', 'b', 'c', 0x80, 0x18, 0x80, 0x1B}, "abcabcabcabc"},
		// A back-reference beyond the start, which is skipped
		{[]byte{'a', 0x87, 0xF8}, "a"},
	} {
		if got := string(decompress(test.data)); got != test.want {
			t.Errorf("% x: expected %q, got %q", test.data, test.want, got)
		}
	}
}

func Test_text_001(t *testing.T) {
	// The trailing entries of a record: an entry whose size is a backward
	// variable-width integer, and the bytes of a character which overlaps
	// the next record
	record := append([]byte("text"), 0x01, 'x', 'y', 'z', 0x84)
	if got := trailingSize(record, 0b10); got != 4 {
		t.Errorf("an entry: expected 4, got %d", got)
	}
	if got := trailingSize(record, 0b11); got != 6 {
		t.Errorf("an entry, and an overlap of two bytes: expected 6, got %d", got)
	}
	if got := trailingSize(record, 0); got != 0 {
		t.Errorf("no entries: expected 0, got %d", got)
	}
	if got := trailingSize([]byte{0x7F}, 0b10); got != 1 {
		t.Errorf("an entry larger than the record: expected 1, got %d", got)
	}
	if !bytes.Equal(record[:len(record)-trailingSize(record, 0b11)], []byte("tex")) {
		t.Error("expected the text before the entries")
	}
}
