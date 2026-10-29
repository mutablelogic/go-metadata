# go-metadata

[![Go Reference](https://pkg.go.dev/badge/github.com/mutablelogic/go-metadata.svg)](https://pkg.go.dev/github.com/mutablelogic/go-metadata)

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
| Audio and video (`audio/ffmpeg`, `video/ffmpeg`) | FFmpeg 8 |
| EXIF metadata of photos (`image/exif`) | libexif |
| HEIF and AVIF images (`image/heif`) | libheif |
| Camera RAW images (`image/raw`) | LibRaw |
| Classes of audio: speech, music or silence (`audio/classify`) | libfvad, which is optional |
| Fingerprints of audio, to look it up on AcoustID (`pkg/chromaprint`) | Chromaprint and libfvad, which are optional |

FFmpeg needs to be FFmpeg 8, which go-media's bindings are for: FFmpeg 9 removed parts of its
API which they use. These are only built with cgo. Without it, which is `CGO_ENABLED=0`, or when there's no C
compiler, go-metadata builds without them. It still reads everything else, but returns no
metadata or artwork from audio and video files, no EXIF metadata from photos, and nothing from
HEIF and RAW images other than their type and kind.

libfvad, the WebRTC voice activity detector, which go-media's segmenter classifies and decodes
audio with, is only built with the `libfvad` build tag, which the Makefile sets when it finds
it. Without it, audio isn't classified. Chromaprint is only built with the `chromaprint` build
tag, which the Makefile sets when it finds it and libfvad. Without it, audio is only looked up
on AcoustID by the AcoustID identifier in its tags. Neither Homebrew nor Debian has libfvad, and
Homebrew's `chromaprint` is built with FFmpeg 9, so to classify or fingerprint audio, build
go-media's libraries, and set `GO_MEDIA_PREFIX` (see below).

The C libraries are found with `pkg-config`, so to build with cgo, install them, their
development headers and `pkg-config`:

**macOS** (with [Homebrew](https://brew.sh/)):

```bash
brew install pkg-config ffmpeg@8 libexif libheif libraw
```

Homebrew's `ffmpeg@8` is installed alongside its `ffmpeg`, which is FFmpeg 9, so it isn't found
unless it's on `PKG_CONFIG_PATH`. The Makefile (see below) does that for you; to build with
`go` itself, set:

```bash
export PKG_CONFIG_PATH="$(brew --prefix ffmpeg@8)/lib/pkgconfig"
export CGO_CFLAGS_ALLOW="-Xpreprocessor" CGO_LDFLAGS_ALLOW="-Xpreprocessor"
```

as Homebrew's LibRaw is built with OpenMP, whose `-Xpreprocessor` flag cgo doesn't allow by
default.

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

Many Linux distributions ship a version of FFmpeg other than 8. If yours does,
[go-media](https://github.com/mutablelogic/go-media), which go-metadata uses for its C
bindings, can build FFmpeg 8 and the other libraries from source, as static libraries: run
`make` in a clone of it, and then build go-metadata with `make GO_MEDIA_PREFIX=<go-media>/build/install`,
or with `go` itself, with `PKG_CONFIG_PATH` set to its `build/install/lib/pkgconfig` folder,
`CGO_LDFLAGS_ALLOW="-(W|D).*"` and `CGO_LDFLAGS="-lstdc++"`, and `-tags libfvad,chromaprint` to
classify and fingerprint audio. `make ffmpeg chromaprint libexif libraw libheif libfvad` in go-media builds
the libraries which go-metadata uses.

**Windows** (with [MSYS2](https://www.msys2.org/), in its UCRT64 environment, which provides
the C compiler too):

```bash
pacman -S mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkgconf \
  mingw-w64-ucrt-x86_64-ffmpeg mingw-w64-ucrt-x86_64-libexif \
  mingw-w64-ucrt-x86_64-libheif mingw-w64-ucrt-x86_64-libraw
```

and build from the UCRT64 shell, with its `bin` folder on your `PATH`, when its FFmpeg is
FFmpeg 8. go-metadata is built and tested on macOS; on Windows, it's expected to work this way,
but isn't tested.

### Building with make

In a clone of go-metadata, the Makefile checks for the C libraries with `pkg-config`, and builds
with cgo when they're all found, and FFmpeg is FFmpeg 8, or else without it:

```bash
make deps             # Report the C libraries, and whether the build uses cgo
make                  # Build the metadata command in build/
make install          # Install the metadata command
make test             # Test everything
make test-race        # Test everything with the race detector
make test-coverage    # Test everything, and report the coverage, in build/coverage.html
make test-nocgo       # Test everything without cgo
make test-image       # Test one package, such as image, audio, video, text, pkg, ...
make help             # List the targets, and the variables which change them
```

`CGO_ENABLED=0` or `1` builds without or with cgo, whatever is found, `GO_MEDIA_PREFIX` uses
go-media's static libraries, and `ARGS` are arguments to `go test`, such as `ARGS=-v`.

## The `metadata` command

Install the `metadata` command with:

```bash
go install github.com/mutablelogic/go-metadata/cmd/metadata@latest
```

which builds with cgo when there's a C compiler, with the C libraries found as above, or without
it with:

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
`TMDB_TOKEN` environment variable, to your TMDB API Read Access Token. To look up the
MusicBrainz recording and artists of audio on AcoustID, set `--acoustid-key`, or the
`CHROMAPRINT_KEY` environment variable, to the API key of an application which is
[registered with AcoustID](https://acoustid.org/new-application). Audio is classified as speech,
music, silence or other sound, as `audio:class`, with the fractions which are speech, music and
silence, as `audio:speech`, `audio:music` and `audio:silence`, from up to its first 10 minutes,
when it's built with libfvad. That decodes it, which takes about a second for each 8 minutes of
audio. `metadata --help` lists every flag.

## Using the library

The API reference is on [pkg.go.dev](https://pkg.go.dev/github.com/mutablelogic/go-metadata).

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
handler for the content type, with a title from the file's name, a `metadata:kind` of
`unknown`, and an empty `metadata:hint`, as there's nothing more to read of it.

The `metadata` namespace describes the file itself: `metadata:kind` is what it is, such as a
book or a conversation, and `metadata:hint` is a list of hints of what to read of it next, as
the handlers which read it found, so an indexer knows what's worth the work:

| Hint | |
| --- | --- |
| `markdown` | It has content which can be read as Markdown, such as a document's text, to index |
| `artwork` | It has artwork which can be extracted, such as a cover, a thumbnail, or a video's frames |
| `ocr` | It has pages which are images, such as a scanned PDF, whose text can be recognised |
| `transcription` | It's audio without MusicBrainz or AcoustID tags, which may be speech |
| `collection` | It's part of a collection: a track with an album, or an episode of a show |
| `event` | It has events, such as a calendar's, a contact's birthday, or an invitation |
| `geo` | It has places, such as where a photo was taken, a contact's address, or GeoJSON |

`metadata:tag` is a list of tags, which are identifiers in lower case, of what a file is or has,
from all the handlers which read it, so an indexer can select files by them, and search for them:
a calendar of events is tagged `event`, and with what its events are, such as `birthday`,
`all-day`, `recurring`, `task` or `out-of-office`, and an email with a calendar attached to it,
such as an invitation, and a mailbox with one, are tagged `event`. A handler returns them with
`TagMetadata`, which leaves out those which aren't identifiers.

`metadata:note` is a list of notes about reading a file: for each handler which found it invalid,
and returned `ErrBadParameter`, such as for a PDF which is cut short, that it couldn't be read and
why, such as `It can't be read, so it may be damaged or incomplete: not a pdf document: …`. It's returned with the error, so an indexer can store
the file with what was read of it, and show why the rest wasn't.

Options change what's extracted:

| Option | |
| --- | --- |
| `WithNamespace(ns...)` | Only the metadata in these namespaces, and only the handlers which return them |
| `WithNamespace(metadata.MarkdownNamespace)` | The content of a document, as Markdown, as `markdown:body` |
| `WithRange("0:4096")` | The range of bytes of the Markdown, which is the first kilobyte when it isn't set, with `markdown:next` when there's more |
| `WithNamespace(metadata.ArtworkNamespace)` | Artwork, whose images are `Image()` and `Bytes()` |
| `WithMaxImageDimension(pixels)`, `WithImageDPI(dpi)` | The size of artwork, and of the images of pages |
| `WithMinImageDimension(pixels)` | The smallest thumbnail embedded in an image, such as a photo's EXIF thumbnail, which is its artwork, or else the image itself is (256 pixels by default) |
| `WithPassword(password)` | The password of an encrypted document |
| `WithContentType(type)` | The content type, rather than detecting it |
| `WithMaxSize(bytes)` | The largest input which is read |
| `WithTMDB(client)` | Look up videos on TMDB, with a client from `pkg/tmdb` |
| `WithAcoustID(client)` | Look up audio on AcoustID, with a client from `pkg/acoustid` |
| `WithTracer(tracer)` | Trace extraction with OpenTelemetry |

A handler of another type of file is a function which is registered with `AddHandler`, for a
regular expression of content types, and the namespaces of the metadata it returns.
`AddHandlerAfter` registers a handler which uses the metadata of other handlers, such as a
video's duration, which it gets with `Opts.Get`. `AddSignature` and `AddTextExtensionType`
register how a type of file is detected, from the bytes it starts with, or the extension of
text.

The formats which go-metadata reads are also packages of their own, in `pkg/`, which can be
used without it, such as [`pkg/eml`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/eml) for email messages, [`pkg/mbox`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/mbox)
for mailboxes, [`pkg/ical`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/ical) and [`pkg/vcard`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/vcard) for calendars and
contacts, [`pkg/rtf`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/rtf), [`pkg/postscript`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/postscript),
[`pkg/iwork`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/iwork), [`pkg/exif`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/exif), [`pkg/xmp`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/xmp),
[`pkg/tmdb`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/tmdb), [`pkg/acoustid`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/acoustid) and
[`pkg/chromaprint`](https://pkg.go.dev/github.com/mutablelogic/go-metadata/pkg/chromaprint).

## License & Distribution

go-metadata is licensed under the **Apache License 2.0**.

> **go-metadata**  
> [https://github.com/mutablelogic/go-metadata/](https://github.com/mutablelogic/go-metadata/)  
> Copyright (c) 2026 The go-metadata authors, All rights reserved.

Under the Apache License 2.0, you are free to use, modify, and distribute this software for any
purpose, including commercial applications. **Attribution requirement:** When redistributing
this software or derivative works (in source or binary form), you must include the copyright
notice above, and a copy of the [LICENSE](LICENSE) and [NOTICE](NOTICE) files. If you modify the
code, you must state your changes. See the
[Apache License 2.0](http://www.apache.org/licenses/LICENSE-2.0) for complete terms.

**Important:** When distributing binaries built with cgo, you must also comply with the licenses
of the C libraries they link to:

| Library | Used for | License |
| --- | --- | --- |
| [FFmpeg](https://ffmpeg.org/legal.html) | Audio and video | LGPL 2.1 or later by default; GPL as Homebrew and go-media build it (see below) |
| [libexif](https://libexif.github.io/) | The EXIF metadata of photos | LGPL 2.1 or later |
| [libheif](https://github.com/strukturag/libheif) | HEIF and AVIF images | LGPL 3.0 |
| [LibRaw](https://www.libraw.org/) | Camera RAW images | LGPL 2.1 or CDDL 1.0, with bundled code under BSD and MIT licenses |
| [Chromaprint](https://acoustid.org/chromaprint) | Fingerprints of audio, with the `chromaprint` tag | LGPL 2.1, as it includes parts of FFmpeg; its own code is MIT |
| [libfvad](https://github.com/dpirch/libfvad) | Classifying audio, with the `libfvad` tag, and decoding it to fingerprint it | BSD 3-clause, with a grant of patent rights |

**Requirements when distributing binaries which include the LGPL libraries:**

1. Include their notices and licenses, which the [NOTICE](NOTICE) file has, and a copy of the
   [GNU GPL v3](https://www.gnu.org/licenses/gpl-3.0.html), which LGPL v3 (libheif) requires
   alongside it
2. Provide access to the source code of the versions you used
3. Allow users to relink the application with modified versions of the libraries. A binary
   which links to them dynamically, such as one built with the libraries from Homebrew or a
   Linux distribution, meets this; one which links to them statically, such as one built with
   go-media's libraries (`GO_MEDIA_PREFIX`), must also provide its object files, or its source

A binary has the license of the FFmpeg it includes, and FFmpeg's license depends on how it's
built:

- **Homebrew's FFmpeg** includes x264 and x265, so it's GPL.
- **go-media's libraries** (`GO_MEDIA_PREFIX`, from go-media v1.11.0) build FFmpeg as
  **"GPL version 3 or later"**.
- **`FFMPEG_NONFREE=1`** in go-media builds it as **"nonfree and unredistributable"**, so a
  binary built with it is for your own use only.

A binary which includes a GPL build of FFmpeg must be distributed under the GPL, with its
complete source code. A binary built without cgo (`CGO_ENABLED=0`) includes none of these
libraries.

## Dependencies

These are the modules which go-metadata imports directly, which are all under permissive
licences. `go.mod` lists the modules they depend on in turn, and `go list -m all` lists every
module with its version.

| Module | Used for | Licence |
| --- | --- | --- |
| [`github.com/mutablelogic/go-media`](https://github.com/mutablelogic/go-media) | The C bindings of FFmpeg, libexif, libheif, LibRaw, Chromaprint and libfvad, and the segmenter which classifies audio and decodes it to fingerprint it | Apache 2.0 |
| [`github.com/mutablelogic/go-client`](https://github.com/mutablelogic/go-client) | The TMDB and AcoustID clients | Apache 2.0 |
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

## Contributing

Please file feature requests and bugs at
[github.com/mutablelogic/go-metadata/issues](https://github.com/mutablelogic/go-metadata/issues).
Pull Requests are welcome, after discussion of the proposed changes.
