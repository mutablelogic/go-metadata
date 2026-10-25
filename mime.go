package metadata

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	ContentTypeTextPlain = "text/plain"
	ContentTypeTextHTML  = "text/html"
	ContentTypeTextXML   = "text/xml"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Type returns the MIME type of the given file, along with a map of any additional
// metadata that was extracted from the file. If the MIME type cannot be determined,
// an error is returned.
func ContentType(r io.Reader) (string, map[string]string, error) {
	if r == nil {
		return "", nil, ErrBadParameter.With("nil reader")
	}

	var extType, textType string
	if named, ok := r.(NamedReader); ok {
		ext := strings.ToLower(filepath.Ext(named.Name()))
		textType = textExtensionType(ext)
		if forced, ok := extensionContentTypes[ext]; ok {
			if forced.Authoritative {
				return mime.ParseMediaType(forced.ContentType)
			}
			// Preferred without Authoritative (e.g. .jpg/.jpeg): use this as
			// the extension-based fallback below, but still sniff first -
			// unlike .m4a, there's no reason not to. Extensions not in this
			// map at all (e.g. .ts, which is either TypeScript source or an
			// MPEG transport stream) go through mime.TypeByExtension the
			// same way.
			extType = forced.ContentType
		} else if ext != "" {
			extType = mime.TypeByExtension(ext)
		}
	}

	// Try the http.DetectContentType function first
	buf := make([]byte, 512)
	n, err := r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", nil, ErrInternalError.With(err.Error())
	}
	if mediaType := http.DetectContentType(buf[:n]); mediaType != types.ContentTypeBinary {
		contentType, params, err := mime.ParseMediaType(mediaType)
		if err != nil {
			return "", nil, err
		}
		// Plain text which starts with the signature of a type of text, such
		// as {\rtf for RTF, is that type, whatever its extension, such as an
		// RTF document saved as .doc
		if contentType == ContentTypeTextPlain {
			if signed := signatureType(buf[:n]); signed != "" {
				return mime.ParseMediaType(signed)
			}
		}

		// Plain text is refined by the type of text of its extension first,
		// such as text/x-typescript for .ts, which is otherwise a video, as
		// is text sniffed as HTML, such as a Vue component, which starts
		// with a <script> tag or a comment
		if (contentType == ContentTypeTextPlain || contentType == ContentTypeTextHTML) && textType != "" {
			return textType, params, nil
		}
		// Plain text, or a zip archive, is refined by the type of its
		// extension, when that's more specific (see refines), such as
		// text/markdown for .md, keeping the sniffed parameters, such as its
		// charset. Extensions are registered with mime.AddExtensionType.
		if extType != "" {
			if extContentType, _, err := mime.ParseMediaType(extType); err == nil && refinesExtension(contentType, extContentType) {
				return extContentType, params, nil
			}
		}
		return contentType, params, nil
	}

	// By the bytes it starts with second, such as 8BPS for a Photoshop
	// document, which is known from its content, whatever its name
	if contentType := signatureType(buf[:n]); contentType != "" {
		return mime.ParseMediaType(contentType)
	}

	// By extension third
	if extType != "" {
		return mime.ParseMediaType(extType)
	}

	// Unknown type
	return types.ContentTypeBinary, nil, nil
}

// AddSignature registers the type of content which starts with the prefix,
// such as image/vnd.adobe.photoshop for 8BPS, or application/rtf for {\rtf,
// which content which http.DetectContentType detects as binary, or as plain
// text, has, rather than the type of its extension, so it's detected without
// its extension, or with the wrong one. The prefix is at most 512 bytes, and
// the longest prefix which content starts with is its type.
func AddSignature(prefix []byte, contentType string) error {
	if len(prefix) == 0 || len(prefix) > 512 {
		return ErrBadParameter.Withf("invalid signature %q", prefix)
	} else if mediatype, _, err := mime.ParseMediaType(contentType); err != nil {
		return ErrBadParameter.Withf("invalid content type %q: %v", contentType, err)
	} else {
		contentType = mediatype
	}
	signaturesMu.Lock()
	defer signaturesMu.Unlock()
	signatures = append(signatures, signature{bytes.Clone(prefix), contentType})
	slices.SortStableFunc(signatures, func(a, b signature) int {
		return len(b.prefix) - len(a.prefix)
	})
	return nil
}

// usualExtensions are the usual extensions of common types, which
// ExtensionByType returns, as the first of a type's extensions in the mime
// package, which reads the system's types, can be another, such as .conf for
// text/plain, or .jfif for image/jpeg
var usualExtensions = map[string]string{
	"text/plain":       ".txt",
	"text/html":        ".html",
	"text/markdown":    ".md",
	"text/csv":         ".csv",
	"text/calendar":    ".ics",
	"text/vcard":       ".vcf",
	"application/json": ".json",
	"application/xml":  ".xml",
	"application/zip":  ".zip",
	"image/png":        ".png",
	"image/gif":        ".gif",
	"image/tiff":       ".tif",
	"image/webp":       ".webp",
	"audio/mpeg":       ".mp3",
	"audio/flac":       ".flac",
	"video/mp4":        ".mp4",
	"video/quicktime":  ".mov",
}

// ExtensionByType returns the usual extension of a content type, with its
// dot, such as .jpg for image/jpeg, or an empty string when it has none. It's
// the type's preferred extension, when it has one, or else the usual
// extension of a common type, or else the first of its extensions which the
// mime package knows, or else the first extension of a type of text (see
// AddTextExtensionType). The type's parameters, such as a
// charset, are ignored.
func ExtensionByType(contentType string) string {
	if mediatype, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediatype
	} else {
		return ""
	}

	// The type's preferred extension
	var preferred []string
	for ext, rule := range extensionContentTypes {
		if rule.ContentType == contentType && rule.Preferred {
			preferred = append(preferred, ext)
		}
	}
	if len(preferred) > 0 {
		slices.Sort(preferred)
		return preferred[0]
	}

	// The usual extension of a common type, whose first extension in the
	// mime package, which depends on the system, isn't
	if ext, exists := usualExtensions[contentType]; exists {
		return ext
	}

	// The first extension which the mime package knows, which it sorts
	if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		return exts[0]
	}

	// The first extension of a type of text
	textExtensionTypesMu.RLock()
	defer textExtensionTypesMu.RUnlock()
	var text []string
	for ext, textType := range textExtensionTypes {
		if textType == contentType {
			text = append(text, ext)
		}
	}
	if len(text) > 0 {
		slices.Sort(text)
		return text[0]
	}
	return ""
}

// AddTextExtensionType registers the type of text with the extension, such
// as text/x-typescript for .ts, which refines content sniffed as plain text,
// as mime.AddExtensionType's does, but is never the type of content by its
// extension alone, so content which isn't text, such as an MPEG transport
// stream, has the type it otherwise has. The extension starts with a dot,
// and is matched without case.
func AddTextExtensionType(ext, contentType string) error {
	if !strings.HasPrefix(ext, ".") || len(ext) < 2 {
		return ErrBadParameter.Withf("invalid extension %q", ext)
	} else if mediatype, _, err := mime.ParseMediaType(contentType); err != nil {
		return ErrBadParameter.Withf("invalid content type %q: %v", contentType, err)
	} else {
		contentType = mediatype
	}
	textExtensionTypesMu.Lock()
	defer textExtensionTypesMu.Unlock()
	textExtensionTypes[strings.ToLower(ext)] = contentType
	return nil
}
