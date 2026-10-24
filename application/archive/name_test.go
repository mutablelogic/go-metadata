package archive

import (
	"io"
	"strings"
	"testing"
)

// named is a reader with a name
type named struct {
	io.Reader
	name string
}

func (r named) Name() string { return r.name }

func Test_name_000(t *testing.T) {
	// The name of the file in a compressed file is its name without its
	// extension, or with .tar for the extension of a tar file
	for _, test := range []struct {
		name, want string
	}{
		{"/backups/notes.txt.bz2", "notes.txt"},
		{"/backups/site.tbz2", "site.tar"},
		{"/backups/site.TBZ", "site.tar"},
		{"/backups/site.tar.bz2", "site.tar"},
		{"/backups/data", "data"},
	} {
		if got := compressedName(named{strings.NewReader(""), test.name}, ".bz2", ".tbz2", ".tbz"); got != test.want {
			t.Errorf("%s: expected %q, got %q", test.name, test.want, got)
		}
	}
	if got := compressedName(strings.NewReader(""), ".bz2"); got != "" {
		t.Errorf("expected no name without a name, got %q", got)
	}
}
