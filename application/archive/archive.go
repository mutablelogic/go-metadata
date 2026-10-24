// Package archive registers handlers of archives, which are zip files, such
// as Java archives, Android apps and comic books, tar files, and gzip and
// bzip2 files, and tar files in them, which return the number of files
// in them, their size, and when they were last modified, and a table of
// their contents as Markdown
package archive

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"maps"
	"mime"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// entry is a file or directory in an archive
type entry struct {
	name     string
	size     int64
	modified time.Time
	dir      bool
}

// info is what's known about an archive, as a whole
type info struct {
	format  string
	comment string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of an archive
	Namespace = "archive"

	// The formats of archives
	formatZip      = "zip"
	formatTar      = "tar"
	formatTarGzip  = "tar+gzip"
	formatGzip     = "gzip"
	formatTarBzip2 = "tar+bzip2"
	formatBzip2    = "bzip2"

	// tarMagic is at tarMagicOffset in the header of a tar file, which isn't
	// a tar file from before POSIX
	tarMagic       = "ustar"
	tarMagicOffset = 257
)

var (
	reContentType = regexp.MustCompile(`^application/(?:` + strings.Join([]string{
		`zip`, `x-zip-compressed`, `java-archive`, `vnd\.android\.package-archive`, `vnd\.comicbook\+zip`,
		`x-tar`, `gzip`, `x-gzip`, `x-bzip2`, `x-bzip`,
	}, "|") + `)$`)

	// zipTypes are the content types of archives which are zip files, such
	// as Java archives, Android apps and comic books
	zipTypes = []string{
		"application/zip", "application/x-zip-compressed", "application/java-archive",
		"application/vnd.android.package-archive", "application/vnd.comicbook+zip",
	}

	// archiveKinds are the kinds of archives which aren't collections of
	// files, such as apps and books, by their content type
	archiveKinds = map[string]string{
		"application/java-archive":                dc.KindSoftware,
		"application/vnd.android.package-archive": dc.KindSoftware,
		"application/vnd.comicbook+zip":           dc.KindBook,
	}

	// archiveTypes are the content types of archives, by extension, which
	// are archives, or compressed files, which are detected by their
	// extension, or which are refined by it, from a zip file
	archiveTypes = map[string]string{
		".tgz":  "application/gzip",
		".bz2":  "application/x-bzip2",
		".tbz":  "application/x-bzip2",
		".tbz2": "application/x-bzip2",
		".jar":  "application/java-archive",
		".war":  "application/java-archive",
		".apk":  "application/vnd.android.package-archive",
		".cbz":  "application/vnd.comicbook+zip",
	}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	for ext, contentType := range archiveTypes {
		if err := mime.AddExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	metadata.AddHandler(reContentType, Namespace, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		meta, err := archiveMetadata(ctx, r, o.ContentType())
		return append(meta, metadata.KindMetadata(archiveKind(o.ContentType()))...), err
	}, dc.Namespace, Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// archiveMetadata returns the metadata of the archive: its format, as
// archive:format, the number of files and directories in it, as
// archive:files and archive:directories, the size of its files when
// they're extracted, as archive:size, when it was last modified, which is
// when its newest entry was, as archive:modified and dc:date, and its
// comment, when it has one, as archive:comment. An archive which is cut
// short, or corrupt, has the metadata of the entries before it, and an
// error.
func archiveMetadata(ctx context.Context, r io.Reader, contentType string) ([]metadata.Metadata, error) {
	var files, dirs, size int64
	var modified time.Time
	info, err := walk(ctx, r, contentType, func(e entry) error {
		if e.dir {
			dirs++
		} else {
			files++
			size += e.size
		}
		if e.modified.After(modified) {
			modified = e.modified
		}
		return nil
	})
	if info.format == "" {
		return nil, err
	}

	entries := make(map[string]metadata.Metadata)
	add := func(ns, key string, value any) {
		if entry := meta.New(ns, key, value); entry != nil {
			entries[ns+string(metadata.Separator)+key] = entry
		}
	}
	add(Namespace, "format", info.format)
	add(Namespace, "files", files)
	add(Namespace, "directories", dirs)
	add(Namespace, "size", size)
	if !modified.IsZero() {
		add(Namespace, "modified", modified.UTC())
		add(dc.Namespace, dc.Date, modified.UTC())
	}
	if comment := strings.TrimSpace(info.comment); comment != "" {
		add(Namespace, "comment", comment)
	}
	return slices.Collect(maps.Values(entries)), err
}

// walk calls the function for each entry of the archive, in the order
// they're in the archive, and returns the format of the archive, and its
// comment, until the function returns an error, which is returned. The
// format is empty when the archive can't be read at all.
func walk(ctx context.Context, r io.Reader, contentType string, fn func(entry) error) (info, error) {
	switch {
	case slices.Contains(zipTypes, contentType):
		return walkZip(ctx, r, fn)
	case contentType == "application/x-tar":
		return info{format: formatTar}, walkTar(ctx, r, fn)
	case contentType == "application/x-bzip2", contentType == "application/x-bzip":
		return walkBzip2(ctx, r, fn)
	default:
		return walkGzip(ctx, r, fn)
	}
}

// walkZip calls the function for each entry of a zip file, from its central
// directory, so none of the entries is decompressed
func walkZip(ctx context.Context, r io.Reader, fn func(entry) error) (info, error) {
	ra, size, err := readerAt(r)
	if err != nil {
		return info{}, err
	}
	archive, err := zip.NewReader(ra, size)
	if err != nil {
		return info{}, metadata.ErrBadParameter.Withf("not a zip file: %v", err)
	}
	result := info{format: formatZip, comment: archive.Comment}
	for _, f := range archive.File {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		dir := f.FileInfo().IsDir()
		if err := fn(entry{name: f.Name, size: int64(f.UncompressedSize64), modified: f.Modified, dir: dir}); err != nil {
			return result, err
		}
	}
	return result, nil
}

// walkTar calls the function for each entry of a tar file, which is read
// from start to end, as it has no index of its entries
func walkTar(ctx context.Context, r io.Reader, fn func(entry) error) error {
	archive := tar.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return metadata.ErrBadParameter.Withf("not a tar file: %v", err)
		}
		e := entry{name: header.Name, size: header.Size, modified: header.ModTime, dir: header.Typeflag == tar.TypeDir}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeDir, tar.TypeSymlink, tar.TypeLink:
			if err := fn(e); err != nil {
				return err
			}
		}
	}
}

// walkGzip calls the function for each entry of a tar file in a gzip file,
// or for the file in the gzip file, when it isn't a tar file, which is its
// name from its header, or else the name of the gzip file without .gz,
// whose size is how many bytes it decompresses to
func walkGzip(ctx context.Context, r io.Reader, fn func(entry) error) (info, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return info{}, metadata.ErrBadParameter.Withf("not a gzip file: %v", err)
	}
	defer gz.Close()
	name := gz.Name
	if name == "" {
		name = compressedName(r, ".gz", ".tgz")
	}
	return walkCompressed(ctx, gz, info{format: formatTarGzip, comment: gz.Comment}, info{format: formatGzip, comment: gz.Comment}, entry{name: name, modified: gz.ModTime}, fn)
}

// walkBzip2 calls the function for each entry of a tar file in a bzip2
// file, or for the file in the bzip2 file, when it isn't a tar file, which
// is the name of the bzip2 file without .bz2, as a bzip2 file has no header
// with a name or a time, whose size is how many bytes it decompresses to
func walkBzip2(ctx context.Context, r io.Reader, fn func(entry) error) (info, error) {
	br := bufio.NewReader(r)
	if magic, err := br.Peek(3); err != nil || string(magic) != "BZh" {
		return info{}, metadata.ErrBadParameter.With("not a bzip2 file")
	}
	file := entry{name: compressedName(r, ".bz2", ".tbz2", ".tbz")}
	return walkCompressed(ctx, bzip2.NewReader(br), info{format: formatTarBzip2}, info{format: formatBzip2}, file, fn)
}

// walkCompressed calls the function for each entry of a tar file, which is
// the decompressed data, with the info of a tar file, or else for the file
// which is the data, with the info of a compressed file, whose size is how
// many bytes it decompresses to
func walkCompressed(ctx context.Context, data io.Reader, tarInfo, fileInfo info, file entry, fn func(entry) error) (info, error) {
	// A tar file has its magic in its first header
	br := bufio.NewReaderSize(data, 1024)
	header, _ := br.Peek(tarMagicOffset + len(tarMagic))
	if bytes.HasPrefix(header[min(len(header), tarMagicOffset):], []byte(tarMagic)) {
		return tarInfo, walkTar(ctx, br, fn)
	}

	// Otherwise, it's the one file which is compressed
	size, err := io.Copy(io.Discard, contextReader{ctx, br})
	if err != nil {
		return fileInfo, err
	}
	file.size = size
	return fileInfo, fn(file)
}

// compressedName returns the name of the file in a compressed file, which is
// the name of the compressed file without its extension, or with .tar in
// place of the extension of a tar file, such as .tgz, or an empty string
// when the compressed file has no name
func compressedName(r io.Reader, ext string, tarExts ...string) string {
	named, ok := r.(metadata.NamedReader)
	if !ok {
		return ""
	}
	name := path.Base(filepath.ToSlash(named.Name()))
	lower := strings.ToLower(name)
	for _, tarExt := range tarExts {
		if strings.HasSuffix(lower, tarExt) {
			return name[:len(name)-len(tarExt)] + ".tar"
		}
	}
	if strings.HasSuffix(lower, ext) {
		return name[:len(name)-len(ext)]
	}
	return name
}

// readerAt returns the archive to read at any offset, and its size, which
// is the reader itself when it can be, as Extract's readers can, and
// otherwise the archive read into memory
func readerAt(r io.Reader) (io.ReaderAt, int64, error) {
	if ra, ok := r.(interface {
		io.ReaderAt
		Size() int64
	}); ok {
		return ra, ra.Size(), nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, err
	}
	return bytes.NewReader(data), int64(len(data)), nil
}

////////////////////////////////////////////////////////////////////////////////
// CONTEXT READER

// contextReader is a reader which returns an error once its context is done,
// so decompressing a large file can be cancelled
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// archiveKind returns the kind of an archive of the content type, which is
// an archive, unless it's an app or a book
func archiveKind(contentType string) string {
	if kind, exists := archiveKinds[contentType]; exists {
		return kind
	}
	return dc.KindArchive
}
