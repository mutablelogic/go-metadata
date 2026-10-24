package archive_test

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/application/archive"
)

// extract returns the metadata of the data, of the content type, by key
func extract(t *testing.T, data []byte, contentType string, opts ...metadata.Option) (map[string]string, error) {
	t.Helper()
	opts = append(opts, metadata.WithContentType(contentType))
	_, meta, err := metadata.Extract(context.Background(), bytes.NewReader(data), opts...)
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return result, err
}

// tarFile returns a tar file of n files, of a byte each
func tarFile(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for i := 0; i < n; i++ {
		header := &tar.Header{Name: filepath.Join("files", string(rune('a'+i%26))+".txt"), Size: 1, Mode: 0o644, ModTime: time.Date(2024, 5, 1, 9, i, 0, 0, time.UTC), Typeflag: tar.TypeReg}
		if err := w.WriteHeader(header); err != nil {
			t.Fatal(err)
		} else if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func Test_archive_000(t *testing.T) {
	// An archive which is cut short has the metadata of the entries before
	// it, and an error
	data := tarFile(t, 10)
	got, err := extract(t, data[:5*1024+100], "application/x-tar")
	if err == nil {
		t.Error("expected an error")
	}
	if got["archive:format"] != "tar" || got["archive:files"] == "" || got["archive:files"] == "10" {
		t.Errorf("expected some of the files, got %v", got)
	}
}

func Test_archive_001(t *testing.T) {
	// The table of contents is read a part at a time, and the newest entry is
	// when the archive was modified
	data := tarFile(t, 30)
	got, err := extract(t, data, "application/x-tar")
	if err != nil {
		t.Fatal(err)
	} else if got["archive:files"] != "30" || got["archive:size"] != "30" || got["archive:modified"] != "2024-05-01T09:29:00Z" {
		t.Errorf("unexpected metadata %v", got)
	}
	got, err = extract(t, data, "application/x-tar", metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange("0:200"))
	if err != nil {
		t.Fatal(err)
	} else if body := got["markdown:body"]; !strings.HasPrefix(body, "| Name | Size | Modified |") || len(body) > 200 || got["markdown:next"] == "" {
		t.Errorf("expected the first part, and the next, got %v", got)
	}
}

func Test_archive_002(t *testing.T) {
	// A file which isn't an archive is a bad parameter
	for _, contentType := range []string{"application/zip", "application/x-gzip"} {
		if _, err := extract(t, []byte("not an archive"), contentType); !errors.Is(err, metadata.ErrBadParameter) {
			t.Errorf("%s: expected ErrBadParameter, got %v", contentType, err)
		}
	}
}

func Test_archive_003(t *testing.T) {
	// Reading an archive is cancelled with its context
	data, err := os.ReadFile(filepath.Join("..", "..", "testfiles", "sample.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := metadata.Extract(ctx, bytes.NewReader(data), metadata.WithContentType("application/x-gzip")); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func Test_archive_004(t *testing.T) {
	// Java archives, Android apps and comic books are zip files
	data, err := os.ReadFile(filepath.Join("..", "..", "testfiles", "sample.zip"))
	if err != nil {
		t.Fatal(err)
	}
	for _, contentType := range []string{"application/java-archive", "application/vnd.android.package-archive", "application/vnd.comicbook+zip"} {
		if got, err := extract(t, data, contentType); err != nil || got["archive:format"] != "zip" || got["archive:files"] != "3" {
			t.Errorf("%s: expected a zip file of 3 files, got %v, %v", contentType, got, err)
		}
	}

	// A file which isn't a bzip2 file is a bad parameter
	if _, err := extract(t, []byte("not bzip2"), "application/x-bzip2"); !errors.Is(err, metadata.ErrBadParameter) {
		t.Errorf("expected ErrBadParameter, got %v", err)
	}
}
