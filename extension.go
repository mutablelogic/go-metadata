package metadata

import (
	"bytes"
	"slices"
	"strings"
	"sync"

	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type extensionRule struct {
	ContentType   string
	Preferred     bool // Extension is preferred when a type has multiple aliases
	Authoritative bool // Authoritative means the extension itself is trusted immediately
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// textExtensionTypes are the types of text, by extension, which only refine
// content sniffed as plain text, and are never the type of content by its
// extension alone, as content of another type has the extension too, such
// as .ts, which is TypeScript, or an MPEG transport stream
// (see AddTextExtensionType)
var (
	textExtensionTypes   = map[string]string{}
	textExtensionTypesMu sync.RWMutex

	// signatures are the types of binary content which http.DetectContentType
	// doesn't detect, by the bytes it starts with (see AddSignature), the
	// longest first
	signatures   []signature
	signaturesMu sync.RWMutex
)

// extensionContentTypes stores extension -> media type mappings and whether
// this extension should be preferred when a type has multiple aliases.
// It is used both for content type detection and extension selection.
var extensionContentTypes = map[string]extensionRule{
	".m4a":  {ContentType: "audio/mp4", Preferred: true, Authoritative: true},
	".m4p":  {ContentType: "audio/mp4", Authoritative: true}, // iTunes and Apple Music protected audio
	".jpg":  {ContentType: "image/jpeg", Preferred: true},
	".jpeg": {ContentType: "image/jpeg"},
	".tga":  {ContentType: "image/x-tga", Preferred: true},

	// HEIF images, and MPEG transport streams, which aren't sniffed, and
	// whose types differ on each platform, such as image/heif for .heic, and
	// Qt's translations for .ts on Linux. TypeScript, which is also .ts, is
	// text, so it's refined by its extension (see AddTextExtensionType).
	".heic":  {ContentType: "image/heic", Preferred: true},
	".heics": {ContentType: "image/heic-sequence", Preferred: true},
	".heif":  {ContentType: "image/heif", Preferred: true},
	".heifs": {ContentType: "image/heif-sequence", Preferred: true},
	".ts":    {ContentType: "video/mp2t", Preferred: true},
	".m2ts":  {ContentType: "video/mp2t"},
	".mts":   {ContentType: "video/mp2t"},

	// Ogg, which is sniffed as application/ogg, is refined by its extension,
	// as audio, such as Opus voice notes, or video
	".ogg":  {ContentType: "audio/ogg", Preferred: true},
	".oga":  {ContentType: "audio/ogg"},
	".opus": {ContentType: "audio/ogg"},
	".spx":  {ContentType: "audio/ogg"},
	".ogv":  {ContentType: "video/ogg", Preferred: true},

	// Playlists, which are text, have different types on each platform
	".m3u":  {ContentType: "audio/x-mpegurl", Preferred: true},
	".m3u8": {ContentType: "application/vnd.apple.mpegurl", Preferred: true},
	".pls":  {ContentType: "audio/x-scpls", Preferred: true},
}

// iconTypes are the content types of an icon, which differ on each platform,
// so an icon isn't refined by another type of icon
var iconTypes = []string{"image/x-icon", "image/vnd.microsoft.icon"}

// textTypes are the content types of text documents which aren't text or
// application types, such as playlists, so they refine plain text
var textTypes = []string{"audio/x-mpegurl", "audio/mpegurl", "audio/x-scpls"}

// zipTypes are the content types, and the prefixes and suffixes of the
// content types, of documents which are zip archives, except the content
// types with the excluded suffixes, such as OpenDocument's flat XML
// documents (.fodt), which are XML
var (
	zipTypes         = []string{"application/java-archive", "application/vnd.android.package-archive", "application/vnd.apple.pages", "application/vnd.apple.keynote", "application/vnd.apple.numbers"}
	zipTypePrefixes  = []string{"application/vnd.openxmlformats-officedocument.", "application/vnd.oasis.opendocument."}
	zipTypeSuffixes  = []string{"+zip"}
	zipTypeExcluding = []string{"-flat-xml"}
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// textExtensionType returns the type of text registered with the extension,
// or an empty string
func textExtensionType(ext string) string {
	textExtensionTypesMu.RLock()
	defer textExtensionTypesMu.RUnlock()
	return textExtensionTypes[ext]
}

// refines returns true when the content type of an extension is a more
// specific type of the sniffed content type. Plain text is refined by a
// text type, such as text/markdown, for example
func refinesExtension(sniffed, ext string) bool {
	switch sniffed {
	case ContentTypeTextPlain, ContentTypeTextXML, ContentTypeTextHTML:
		if isXMLType(ext) {
			return true
		}
	}
	switch sniffed {
	case ContentTypeTextPlain:
		switch {
		case ext == sniffed:
			return false
		case strings.HasPrefix(ext, "text/"), slices.Contains(textTypes, ext):
			return true
		case strings.HasPrefix(ext, "application/"):
			return ext != types.ContentTypeBinary && ext != "application/zip" && !isZipType(ext)
		}
	case "application/zip":
		return isZipType(ext)
	case "application/ogg":
		return strings.HasPrefix(ext, "audio/") || strings.HasPrefix(ext, "video/")
	case "image/x-icon":
		return strings.HasPrefix(ext, "image/") && !slices.Contains(iconTypes, ext)
	}
	return false
}

// isXMLType returns true when the content type is of an XML document with a
// more specific type, such as image/svg+xml
func isXMLType(contentType string) bool {
	return strings.HasSuffix(contentType, "+xml")
}

// isZipType returns true when the content type is of a document which is a
// zip archive, but not a flat XML document
func isZipType(contentType string) bool {
	if slices.Contains(zipTypes, contentType) {
		return true
	}
	for _, suffix := range zipTypeExcluding {
		if strings.HasSuffix(contentType, suffix) {
			return false
		}
	}
	for _, prefix := range zipTypePrefixes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	for _, suffix := range zipTypeSuffixes {
		if strings.HasSuffix(contentType, suffix) {
			return true
		}
	}
	return false
}

// signature is the type of binary content which starts with a prefix
type signature struct {
	prefix      []byte
	contentType string
}

// signatureType returns the type of binary content, by the longest
// signature which it starts with, or an empty string when none is
func signatureType(data []byte) string {
	signaturesMu.RLock()
	defer signaturesMu.RUnlock()
	for _, s := range signatures {
		if bytes.HasPrefix(data, s.prefix) {
			return s.contentType
		}
	}
	return ""
}
