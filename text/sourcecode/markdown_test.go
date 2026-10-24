package sourcecode_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/text/sourcecode"
)

// named is a reader of the data, with a name
type named struct {
	*bytes.Reader
	name string
}

func (r named) Name() string { return r.name }

// markdown returns the content type of the named data, and its Markdown
func markdown(t *testing.T, name string, data []byte) (string, string) {
	t.Helper()
	contentType, meta, err := metadata.Extract(context.Background(), named{bytes.NewReader(data), name},
		metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	for _, m := range meta {
		if m.NS() == metadata.MarkdownNamespace && m.Key() == "body" {
			return contentType, m.Value()
		}
	}
	return contentType, ""
}

func Test_sourcecode_000(t *testing.T) {
	// Source code, and data files, are a code block in their language
	for _, test := range []struct {
		name, data, contentType, want string
	}{
		{"config.yaml", "name: demo\nitems:\n  - a\n", "application/yaml; charset=utf-8", "```yaml\nname: demo\nitems:\n  - a\n```\n"},
		{"config.yml", "a: 1", "application/yaml; charset=utf-8", "```yaml\na: 1\n```\n"},
		{"data.json", `{"a": [1, 2]}`, "application/json; charset=utf-8", "```json\n{\"a\": [1, 2]}\n```\n"},
		{"app.toml", "[server]\r\nport = 8080\r\n", "application/toml; charset=utf-8", "```toml\n[server]\nport = 8080\n```\n"},
		{"main.go", "package main\n", "text/x-go; charset=utf-8", "```go\npackage main\n```\n"},
		{"script.ts", "const a: number = 1\n", "text/x-typescript; charset=utf-8", "```typescript\nconst a: number = 1\n```\n"},
	} {
		contentType, got := markdown(t, test.name, []byte(test.data))
		if contentType != test.contentType {
			t.Errorf("%s: expected %q, got %q", test.name, test.contentType, contentType)
		}
		if got != test.want {
			t.Errorf("%s: expected %q, got %q", test.name, test.want, got)
		}
	}
}

func Test_sourcecode_001(t *testing.T) {
	// A fence is longer than any run of backticks in the code, and an
	// empty file is an empty code block
	if _, got := markdown(t, "notes.go", []byte("// ```quoted``` and ````more````\n")); got != "`````go\n// ```quoted``` and ````more````\n`````\n" {
		t.Errorf("got %q", got)
	}
	if _, got := markdown(t, "empty.yaml", nil); got != "```yaml\n```\n" {
		t.Errorf("got %q", got)
	}
}

func Test_sourcecode_002(t *testing.T) {
	// .ts which isn't text, such as an MPEG transport stream, isn't
	// TypeScript
	data := bytes.Repeat([]byte{0x47, 0x00, 0x11, 0x10}, 188)
	contentType, meta, err := metadata.Extract(context.Background(), named{bytes.NewReader(data), "video.ts"}, metadata.WithNamespace(metadata.MarkdownNamespace))
	if !errors.Is(err, metadata.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	} else if contentType != "video/mp2t" || len(meta) != 0 {
		t.Errorf("expected video/mp2t, and no metadata, got %q with %d entries", contentType, len(meta))
	}
}
