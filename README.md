# go-metadata

Files carry a lot of information about themselves, in a different format for every kind of
file: EXIF tags in a photo, ID3 tags in an MP3, document properties in a Word document, a
`Subject:` header in an email. go-metadata reads them all into one set of metadata, so a
photo, a song, a PDF and an email can be indexed and searched the same way. Common properties,
such as a file's title, creator, date and kind, use the same keys whatever the format, mostly
from Dublin Core, alongside the format's own properties in a namespace of their own, such as
`exif:`, `pdf:` or `email:`. Beyond metadata, it extracts the content of documents as
Markdown, a page at a time, and artwork, such as album covers, the thumbnails of photos, video
frames, and images of the pages of documents. It can also look up the movie or TV episode a
video is on [The Movie Database](https://www.themoviedb.org/).

It reads images (JPEG, PNG, GIF, WebP, TIFF, HEIF/AVIF, camera RAW, SVG, Photoshop), audio and
video (anything FFmpeg can read, and playlists), documents (PDF, Word, Excel and PowerPoint, old
and new, OpenDocument, Pages and Keynote, RTF, PostScript, EPUB and Kindle books), text
(Markdown, HTML, XML, CSV, source code, vCards and iCalendars), email messages and mailboxes,
and archives.

## Requirements

go-metadata needs Go 1.27 or later. Most of it is pure Go, but a few of its handlers use C
libraries, through cgo:

| Handler | C library |
| --- | --- |
| Audio and video (`audio/ffmpeg`, `video/ffmpeg`) | FFmpeg 8 or later |
| EXIF metadata of photos (`image/exif`) | libexif |
| HEIF and AVIF images (`image/heif`) | libheif |
| Camera RAW images (`image/raw`) | LibRaw |

These are only built with cgo. Without it, which is `CGO_ENABLED=0`, or when there's no C
compiler, go-metadata builds without them. It still reads everything else, but returns no
metadata or artwork from audio and video files, no EXIF metadata from photos, and nothing from
HEIF and RAW images other than their type and kind.

The C libraries are found with `pkg-config`, so to build with cgo, install them, their
development headers and `pkg-config`:

**macOS** (with [Homebrew](https://brew.sh/)):

```bash
brew install pkg-config ffmpeg libexif libheif libraw
```

**Linux** (Debian and Ubuntu):

```bash
apt install build-essential pkg-config libexif-dev libheif-dev libraw-dev \
  libavcodec-dev libavformat-dev libavutil-dev libavdevice-dev libavfilter-dev \
  libswscale-dev libswresample-dev
```

and on Fedora:

```bash
dnf install gcc pkg-config libexif-devel libheif-devel LibRaw-devel ffmpeg-free-devel
```

Many Linux distributions ship a version of FFmpeg older than 8. If yours does,
[go-media](https://github.com/mutablelogic/go-media), which go-metadata uses for its C
bindings, can build FFmpeg and the other libraries from source, as static libraries: run `make`
in a clone of it, and then set `PKG_CONFIG_PATH` to its `build/install/lib/pkgconfig` folder,
and `CGO_LDFLAGS_ALLOW="-(W|D).*"`, when you build go-metadata.

**Windows** (with [MSYS2](https://www.msys2.org/), in its UCRT64 environment, which provides
the C compiler too):

```bash
pacman -S mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkgconf \
  mingw-w64-ucrt-x86_64-ffmpeg mingw-w64-ucrt-x86_64-libexif \
  mingw-w64-ucrt-x86_64-libheif mingw-w64-ucrt-x86_64-libraw
```

and build from the UCRT64 shell, with its `bin` folder on your `PATH`. go-metadata is built and
tested on macOS; on Windows, it's expected to work this way, but isn't tested.

## The `metadata` command

Install the `metadata` command with:

```bash
go install github.com/mutablelogic/go-metadata/cmd/metadata@latest
```

which builds with cgo when there's a C compiler, or without it with:

```bash
CGO_ENABLED=0 go install github.com/mutablelogic/go-metadata/cmd/metadata@latest
```

`metadata` prints the content type and metadata of a file, of every file in a folder, or of the
files which match a glob pattern, which is quoted so the shell doesn't expand it:

```bash
metadata photo.jpg                      # A table of the metadata of a file
metadata ~/Documents                    # Of every file in a folder, and its folders
metadata 'Music/**/*.mp3'               # Of the files which match a pattern
metadata --ns dc,exif photo.jpg         # Only the metadata in these namespaces
metadata --json report.pdf              # As JSON
metadata --type ~/Downloads             # Only the content types
metadata --xmp photo.jpg > photo.xmp    # As an XMP sidecar
```

It prints the content of a document as Markdown, all of it, or a range of bytes of it, a page
at a time:

```bash
metadata --markdown report.pdf
metadata --markdown --range 0:4096 report.pdf
```

and saves the artwork of files as images in the current folder, such as the images of a PDF
document's pages, or an album's cover, at a resolution you choose for the images of pages:

```bash
metadata --artwork --dpi 300 scan.pdf
```

The password of an encrypted document is `--password`, or the `METADATA_PASSWORD` environment
variable. To look up the movie or TV episode which a video is on TMDB, set `--tmdb-key`, or the
`TMDB_TOKEN` environment variable, to your TMDB API Read Access Token. `metadata --help` lists
every flag.

## Using the library

Import the go-metadata package, and the packages of the handlers of the files you want to read,
which register themselves when they're imported:

```go
import (
    "context"
    "fmt"
    "os"

    metadata "github.com/mutablelogic/go-metadata"

    // Handlers
    _ "github.com/mutablelogic/go-metadata/application" // PDF, office documents, books, archives
    _ "github.com/mutablelogic/go-metadata/audio"       // Audio and playlists
    _ "github.com/mutablelogic/go-metadata/image"       // Images
    _ "github.com/mutablelogic/go-metadata/message"     // Email messages and mailboxes
    _ "github.com/mutablelogic/go-metadata/text"        // Text, HTML, Markdown, CSV, XML, ...
    _ "github.com/mutablelogic/go-metadata/video"       // Video and subtitles
)

func main() {
    f, err := os.Open("report.pdf")
    if err != nil {
        panic(err)
    }
    defer f.Close()

    contentType, meta, err := metadata.Extract(context.Background(), f)
    if err != nil {
        panic(err)
    }
    fmt.Println(contentType)
    for _, m := range meta {
        fmt.Printf("%s:%s = %s\n", m.NS(), m.Key(), m.Value())
    }
}
```

`Extract` detects the content type of the reader from its content, and from its name when it
has one, as an `*os.File` does, and runs every handler of that type concurrently. Each piece of
metadata has a namespace and a key, such as `dc` and `title`, and a value, which is text, a
typed value, such as a time or a number, or an image. When a handler fails, `Extract` returns
the metadata which was extracted along with the error, and `ErrNotImplemented` when there's no
handler for the content type.

Options change what's extracted:

| Option | |
| --- | --- |
| `WithNamespace(ns...)` | Only the metadata in these namespaces, and only the handlers which return them |
| `WithNamespace(metadata.MarkdownNamespace)` | The content of a document, as Markdown, as `markdown:body` |
| `WithRange("0:4096")` | The range of bytes of the Markdown, which is the first kilobyte when it isn't set, with `markdown:next` when there's more |
| `WithNamespace(metadata.ArtworkNamespace)` | Artwork, whose images are `Image()` and `Bytes()` |
| `WithMaxImageDimension(pixels)`, `WithImageDPI(dpi)` | The size of artwork, and of the images of pages |
| `WithPassword(password)` | The password of an encrypted document |
| `WithContentType(type)` | The content type, rather than detecting it |
| `WithMaxSize(bytes)` | The largest input which is read |
| `WithTMDB(client)` | Look up videos on TMDB, with a client from `pkg/tmdb` |
| `WithTracer(tracer)` | Trace extraction with OpenTelemetry |

A handler of another type of file is a function which is registered with `AddHandler`, for a
regular expression of content types, and the namespaces of the metadata it returns.
`AddHandlerAfter` registers a handler which uses the metadata of other handlers, such as a
video's duration, which it gets with `Opts.Get`. `AddSignature` and `AddTextExtensionType`
register how a type of file is detected, from the bytes it starts with, or the extension of
text.

The formats which go-metadata reads are also packages of their own, in `pkg/`, which can be
used without it, such as `pkg/eml` for email messages, `pkg/mbox` for mailboxes, `pkg/ical` and
`pkg/vcard` for calendars and contacts, `pkg/rtf`, `pkg/postscript`, `pkg/iwork`, `pkg/exif`,
`pkg/xmp` and `pkg/tmdb`.

## License

go-metadata is licensed under the [Apache License 2.0](LICENSE).

When it's built with cgo, it links to FFmpeg, libexif, libheif and LibRaw, which are licensed
under the LGPL (LibRaw is also available under the CDDL), so a binary which includes them is
subject to their licences too. A build of FFmpeg with GPL components, such as Homebrew's, which
includes x264 and x265, makes such a binary subject to the GPL.

## Dependencies

These are the modules which go-metadata imports directly, which are all under permissive
licences. `go.mod` lists the modules they depend on in turn, and `go list -m all` lists every
module with its version.

| Module | Used for | Licence |
| --- | --- | --- |
| [`github.com/mutablelogic/go-media`](https://github.com/mutablelogic/go-media) | The C bindings of FFmpeg, libexif, libheif and LibRaw | Apache 2.0 |
| [`github.com/mutablelogic/go-client`](https://github.com/mutablelogic/go-client) | The TMDB client | Apache 2.0 |
| [`github.com/mutablelogic/go-server`](https://github.com/mutablelogic/go-server) | Errors and types | Apache 2.0 |
| [`github.com/mutablelogic/go-tokenizer`](https://github.com/mutablelogic/go-tokenizer) | Markdown documents | Apache 2.0 |
| [`github.com/pdfcpu/pdfcpu`](https://github.com/pdfcpu/pdfcpu) | The properties of PDF documents, and the thumbnails stored in them | Apache 2.0 |
| [`github.com/giraffesyo/pdf`](https://github.com/giraffesyo/pdf) | The text of PDF documents, and the images of scanned pages | Apache 2.0 |
| [`github.com/JohannesKaufmann/html-to-markdown/v2`](https://github.com/JohannesKaufmann/html-to-markdown) | HTML documents and email messages as Markdown | MIT |
| [`github.com/richardlehane/mscfb`](https://github.com/richardlehane/mscfb), [`msoleps`](https://github.com/richardlehane/msoleps) | Word, Excel, PowerPoint, Visio, Project and Works documents before Office 2007 | Apache 2.0 |
| [`github.com/oov/psd`](https://github.com/oov/psd) | Photoshop documents | MIT |
| [`go.yaml.in/yaml/v3`](https://github.com/yaml/go-yaml) | The front matter of Markdown documents | MIT and Apache 2.0 |
| [`golang.org/x/image`](https://pkg.go.dev/golang.org/x/image), [`x/net`](https://pkg.go.dev/golang.org/x/net), [`x/text`](https://pkg.go.dev/golang.org/x/text) | Images, HTML, and character sets | BSD 3-Clause |
| [`go.opentelemetry.io/otel`](https://github.com/open-telemetry/opentelemetry-go) | Tracing | Apache 2.0 |
| [`github.com/alecthomas/kong`](https://github.com/alecthomas/kong) | The `metadata` command's flags | MIT |
| [`github.com/bmatcuk/doublestar/v4`](https://github.com/bmatcuk/doublestar) | The `metadata` command's glob patterns | MIT |
| [`github.com/charmbracelet/glamour`](https://github.com/charmbracelet/glamour), [`lipgloss`](https://github.com/charmbracelet/lipgloss), [`x/ansi`](https://github.com/charmbracelet/x), [`x/term`](https://github.com/charmbracelet/x) | The `metadata` command's tables and Markdown in a terminal | MIT |
