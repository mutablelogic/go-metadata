// Package sourcecode registers a handler of source code, and the data and
// build files with it, such as YAML, JSON and TOML, which returns its content
// as Markdown, as a code block in its language
package sourcecode

import (
	"context"
	"io"
	"mime"
	"regexp"
	"slices"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	itext "github.com/mutablelogic/go-metadata/internal/text"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// sourceType is the content type of source code, and the name of its
// language in the info string of a Markdown code block
type sourceType struct {
	contentType string
	language    string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// sourceTypes are the types of source code, and of the data and build files
// with it, by extension, which are the types freedesktop.org's shared MIME
// info has, or the registered type, when there is one. Source code is
// detected as plain text, and refined by its extension.
//
// An extension of source code which is also another type of file, which
// isn't text, is in textSourceTypes. An extension of more than one language
// isn't here: .d, .v, .fs and .pl, which are Perl or Prolog, and .cls,
// which is a LaTeX class or Visual Basic, except .m, which is Objective-C,
// though it's MATLAB too.
var sourceTypes = map[string]sourceType{
	// C and C++, and Objective-C and Objective-C++
	".c":   {"text/x-csrc", "c"},
	".h":   {"text/x-chdr", "c"},
	".cc":  {"text/x-c++src", "cpp"},
	".cpp": {"text/x-c++src", "cpp"},
	".cxx": {"text/x-c++src", "cpp"},
	".hh":  {"text/x-c++hdr", "cpp"},
	".hpp": {"text/x-c++hdr", "cpp"},
	".hxx": {"text/x-c++hdr", "cpp"},
	".m":   {"text/x-objcsrc", "objectivec"},
	".mm":  {"text/x-objc++src", "objectivec"},

	// Languages
	".go":     {"text/x-go", "go"},
	".rs":     {"text/rust", "rust"},
	".py":     {"text/x-python", "python"},
	".pyi":    {"text/x-python", "python"},
	".java":   {"text/x-java", "java"},
	".kt":     {"text/x-kotlin", "kotlin"},
	".kts":    {"text/x-kotlin", "kotlin"},
	".scala":  {"text/x-scala", "scala"},
	".groovy": {"text/x-groovy", "groovy"},
	".gradle": {"text/x-gradle", "groovy"},
	".cs":     {"text/x-csharp", "csharp"},
	".vb":     {"text/x-vb", "vbnet"},
	".swift":  {"text/x-swift", "swift"},
	".dart":   {"text/x-dart", "dart"},
	".rb":     {"application/x-ruby", "ruby"},
	".php":    {"application/x-php", "php"},
	".pm":     {"application/x-perl", "perl"},
	".lua":    {"text/x-lua", "lua"},
	".r":      {"text/x-r", "r"},
	".jl":     {"text/x-julia", "julia"},
	".hs":     {"text/x-haskell", "haskell"},
	".ml":     {"text/x-ocaml", "ocaml"},
	".mli":    {"text/x-ocaml", "ocaml"},
	".fsx":    {"text/x-fsharp", "fsharp"},
	".fsi":    {"text/x-fsharp", "fsharp"},
	".ex":     {"text/x-elixir", "elixir"},
	".exs":    {"text/x-elixir", "elixir"},
	".erl":    {"text/x-erlang", "erlang"},
	".hrl":    {"text/x-erlang", "erlang"},
	".clj":    {"text/x-clojure", "clojure"},
	".cljs":   {"text/x-clojure", "clojure"},
	".cljc":   {"text/x-clojure", "clojure"},
	".lisp":   {"text/x-common-lisp", "lisp"},
	".el":     {"text/x-emacs-lisp", "elisp"},
	".scm":    {"text/x-scheme", "scheme"},
	".zig":    {"text/x-zig", "zig"},
	".nim":    {"text/x-nim", "nim"},
	".elm":    {"text/x-elm", "elm"},
	".tcl":    {"text/x-tcl", "tcl"},

	// JavaScript and the languages compiled to it, and web components
	".js":     {"text/javascript", "javascript"},
	".mjs":    {"text/javascript", "javascript"},
	".cjs":    {"text/javascript", "javascript"},
	".jsx":    {"text/jsx", "jsx"},
	".tsx":    {"text/x-typescript", "typescript"},
	".cts":    {"text/x-typescript", "typescript"},
	".coffee": {"text/x-coffeescript", "coffeescript"},
	".vue":    {"text/x-vue", "vue"},
	".svelte": {"text/x-svelte", "svelte"},
	".css":    {"text/css", "css"},
	".scss":   {"text/x-scss", "scss"},
	".sass":   {"text/x-sass", "sass"},
	".less":   {"text/x-less", "less"},

	// Shells and scripts
	".sh":   {"application/x-shellscript", "sh"},
	".bash": {"application/x-shellscript", "sh"},
	".zsh":  {"application/x-shellscript", "sh"},
	".ksh":  {"application/x-shellscript", "sh"},
	".fish": {"application/x-fishscript", "fish"},
	".ps1":  {"application/x-powershell", "powershell"},
	".psm1": {"application/x-powershell", "powershell"},
	".bat":  {"application/x-bat", "batch"},
	".cmd":  {"application/x-bat", "batch"},

	// Data, schemas and queries
	".json":    {"application/json", "json"},
	".yaml":    {"application/yaml", "yaml"},
	".yml":     {"application/yaml", "yaml"},
	".toml":    {"application/toml", "toml"},
	".sql":     {"application/sql", "sql"},
	".proto":   {"text/x-protobuf", "protobuf"},
	".graphql": {"application/graphql", "graphql"},
	".gql":     {"application/graphql", "graphql"},
	".ipynb":   {"application/x-ipynb+json", "json"},
	".geojson": {"application/geo+json", "json"},
	".ndjson":  {"application/x-ndjson", "json"},
	".jsonl":   {"application/x-ndjson", "json"},

	// Building and changes
	".mk":    {"text/x-makefile", "makefile"},
	".cmake": {"text/x-cmake", "cmake"},
	".tf":    {"text/x-terraform", "terraform"},
	".hcl":   {"text/x-hcl", "hcl"},
	".diff":  {"text/x-patch", "diff"},
	".patch": {"text/x-patch", "diff"},

	// Documents in plain text, which aren't Markdown
	".rst":   {"text/x-rst", "rst"},
	".adoc":  {"text/asciidoc", "asciidoc"},
	".org":   {"text/org", "org"},
	".tex":   {"text/x-tex", "latex"},
	".latex": {"text/x-tex", "latex"},
	".ltx":   {"text/x-tex", "latex"},
	".sty":   {"text/x-tex", "latex"},
	".bib":   {"text/x-bibtex", "bibtex"},
}

// textSourceTypes are the types of source code which refine a file which is
// text, which aren't in sourceTypes when their extensions are also another
// type of file, which isn't text, such as .ts and .mts, which are TypeScript,
// or else an MPEG transport stream, such as a camcorder's AVCHD video. Web
// components are in both: they sniff as HTML, as they start with a <script>
// tag or a comment, which only these types refine, and sourceTypes gives a
// file its type by its extension alone.
var textSourceTypes = map[string]sourceType{
	".ts":     {"text/x-typescript", "typescript"},
	".mts":    {"text/x-typescript", "typescript"},
	".vue":    {"text/x-vue", "vue"},
	".svelte": {"text/x-svelte", "svelte"},
}

// sourceKinds are the kinds of the content types of source code which isn't
// code, such as data and documents in plain text
var sourceKinds = map[string]string{
	"application/json":         dc.KindData,
	"application/geo+json":     dc.KindData,
	"application/x-ndjson":     dc.KindData,
	"application/yaml":         dc.KindData,
	"application/toml":         dc.KindData,
	"application/x-ipynb+json": dc.KindData,
	"text/x-rst":               dc.KindDocument,
	"text/asciidoc":            dc.KindDocument,
	"text/org":                 dc.KindDocument,
	"text/x-tex":               dc.KindDocument,
	"text/x-bibtex":            dc.KindData,
}

// aliasTypes are the other names of types of source code, by the type
// they're another name of
var aliasTypes = map[string]string{
	"application/javascript":   "text/javascript",
	"application/x-javascript": "text/javascript",
	"application/x-yaml":       "application/yaml",
	"text/yaml":                "application/yaml",
	"text/x-yaml":              "application/yaml",
	"application/x-sh":         "application/x-shellscript",
	"text/x-sh":                "application/x-shellscript",
	"text/x-shellscript":       "application/x-shellscript",
	"application/jsonl":        "application/x-ndjson",
	"application/x-jsonlines":  "application/x-ndjson",
}

// geoJSONType is the type of GeoJSON, whose data is places
const geoJSONType = "application/geo+json"

// reJSON matches the types of data which is JSON, such as
// application/ld+json, by their suffix, whose subtypes have the characters
// RFC 6838 allows
var reJSON = regexp.MustCompile(`^application/[A-Za-z0-9!#$&^_.+-]+\+json$`)

// languages are the names of the languages of the content types of source
// code, in the info string of a Markdown code block
var languages = make(map[string]string)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// Source code is detected as plain text, and refined by its extension
	for ext, source := range sourceTypes {
		if err := mime.AddExtensionType(ext, source.contentType); err != nil {
			panic(err)
		}
		languages[source.contentType] = source.language
	}
	for ext, source := range textSourceTypes {
		if err := metadata.AddTextExtensionType(ext, source.contentType); err != nil {
			panic(err)
		}
		languages[source.contentType] = source.language
	}
	for alias, contentType := range aliasTypes {
		languages[alias] = languages[contentType]
	}
	contentTypes := make([]string, 0, len(languages)+1)
	for contentType := range languages {
		contentTypes = append(contentTypes, regexp.QuoteMeta(contentType))
	}
	slices.Sort(contentTypes)
	contentTypes = append(contentTypes, reJSON.String()[1:len(reJSON.String())-1])

	// The kind of the source code, and its content, as a code block in its
	// language, only when the markdown namespace is requested
	metadata.AddHandler(regexp.MustCompile(`^(?:`+strings.Join(contentTypes, "|")+`)$`), "sourcecode", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := append(metadata.KindMetadata(sourceKind(o.ContentType())), hints(o.ContentType(), r)...)
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return kind, nil
		}
		w := imarkdown.NewWriter(o)
		meta, err := w.Metadata(itext.WriteCodeBlock(w, language(o.ContentType()), r))
		return append(meta, kind...), err
	}, metadata.MarkdownNamespace, metadata.MetadataNamespace, dc.Namespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// sourceKind returns the kind of source code of the content type, or the
// type it's another name of, which is code, unless it's data, such as JSON,
// or a document
func sourceKind(contentType string) string {
	if alias, exists := aliasTypes[contentType]; exists {
		contentType = alias
	}
	if kind, exists := sourceKinds[contentType]; exists {
		return kind
	} else if reJSON.MatchString(contentType) {
		return dc.KindData
	}
	return dc.KindCode
}

// hints returns the hints of source code which has text: that it can be
// read as Markdown, and for GeoJSON, that it has places, which is read
// without moving r
func hints(contentType string, r io.Reader) []metadata.Metadata {
	if !itext.HasText(r) {
		return nil
	} else if contentType == geoJSONType {
		return metadata.HintMetadata(metadata.HintMarkdown, metadata.HintGeo)
	}
	return metadata.HintMetadata(metadata.HintMarkdown)
}

// language returns the name of the language of the content type, in the
// info string of a code block, which is json for data which is JSON
func language(contentType string) string {
	if language, exists := languages[contentType]; exists {
		return language
	} else if reJSON.MatchString(contentType) {
		return "json"
	}
	return ""
}
