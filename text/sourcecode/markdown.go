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
// isn't here: .d, .v, .fs and .pl, which are Perl or Prolog, except .m, which
// is Objective-C, though it's MATLAB too.
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

	// Building and changes
	".mk":    {"text/x-makefile", "makefile"},
	".cmake": {"text/x-cmake", "cmake"},
	".tf":    {"text/x-terraform", "terraform"},
	".hcl":   {"text/x-hcl", "hcl"},
	".diff":  {"text/x-patch", "diff"},
	".patch": {"text/x-patch", "diff"},

	// Documents in plain text, which aren't Markdown
	".rst":  {"text/x-rst", "rst"},
	".adoc": {"text/asciidoc", "asciidoc"},
	".org":  {"text/org", "org"},
	".tex":  {"text/x-tex", "latex"},
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
	"application/yaml":         dc.KindData,
	"application/toml":         dc.KindData,
	"application/x-ipynb+json": dc.KindData,
	"text/x-rst":               dc.KindDocument,
	"text/asciidoc":            dc.KindDocument,
	"text/org":                 dc.KindDocument,
	"text/x-tex":               dc.KindDocument,
}

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
	contentTypes := make([]string, 0, len(languages))
	for contentType := range languages {
		contentTypes = append(contentTypes, regexp.QuoteMeta(contentType))
	}
	slices.Sort(contentTypes)

	// The kind of the source code, and its content, as a code block in its
	// language, only when the markdown namespace is requested
	metadata.AddHandler(regexp.MustCompile(`^(?:`+strings.Join(contentTypes, "|")+`)$`), "sourcecode", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(sourceKind(o.ContentType()))
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return kind, nil
		}
		text, err := itext.Read(r)
		if err != nil {
			return kind, err
		}
		w := imarkdown.NewWriter(o)
		_, err = w.WriteString(codeBlock(languages[o.ContentType()], text))
		meta, err := w.Metadata(err)
		return append(meta, kind...), err
	}, metadata.MarkdownNamespace, metadata.MetadataNamespace, dc.Namespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// sourceKind returns the kind of source code of the content type, which is
// code, unless it's data or a document
func sourceKind(contentType string) string {
	if kind, exists := sourceKinds[contentType]; exists {
		return kind
	}
	return dc.KindCode
}

// codeBlock returns the text as a fenced Markdown code block in the
// language, with a fence of more backticks than any run of them in the text,
// so the text can't end it
func codeBlock(language, text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	fence := strings.Repeat("`", max(3, longestRun(text, '`')+1))
	return fence + language + "\n" + text + fence + "\n"
}

// longestRun returns the length of the longest run of the character in the
// text
func longestRun(text string, c byte) int {
	longest, run := 0, 0
	for i := 0; i < len(text); i++ {
		if text[i] == c {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return longest
}
