package input_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	// Packages
	input "github.com/mutablelogic/go-metadata/internal/input"
)

// stream is a reader which can't be read at an offset or seeked
type stream struct{ io.Reader }

func Test_input_000(t *testing.T) {
	// A reader which can be seeked is read from its position, and a stream
	// is copied to a temporary file, which is removed when it's closed, and
	// each section is read from the start
	seekable := strings.NewReader("..hello")
	if _, err := seekable.Seek(2, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	for name, r := range map[string]io.Reader{
		"seekable": seekable,
		"stream":   stream{strings.NewReader("hello")},
	} {
		in, err := input.New(context.Background(), r, 0)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if data, err := io.ReadAll(in.Reader()); err != nil || string(data) != "hello" {
				t.Errorf("%s: expected hello, got %q %v", name, data, err)
			}
		}
		if err := in.Close(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func Test_input_001(t *testing.T) {
	// An input larger than its maximum size is too large, and one of its
	// maximum size isn't
	for name, r := range map[string]func() io.Reader{
		"seekable": func() io.Reader { return strings.NewReader("hello") },
		"stream":   func() io.Reader { return stream{strings.NewReader("hello")} },
	} {
		if _, err := input.New(context.Background(), r(), 4); !errors.Is(err, input.ErrTooLarge) {
			t.Errorf("%s: expected ErrTooLarge, got %v", name, err)
		}
		in, err := input.New(context.Background(), r(), 5)
		if err != nil {
			t.Errorf("%s: %v", name, err)
		} else {
			in.Close()
		}
	}
}

func Test_input_002(t *testing.T) {
	// The name of a file is kept on each section
	f, err := os.Open("input.go")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	in, err := input.New(context.Background(), f, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	if named, ok := in.Reader().(interface{ Name() string }); !ok || named.Name() != "input.go" {
		t.Errorf("expected the name input.go, got %v", in.Reader())
	}
}
