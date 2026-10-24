package metadata

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
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

	// By extension second
	if extType != "" {
		return mime.ParseMediaType(extType)
	}

	// Unknown type
	return types.ContentTypeBinary, nil, nil
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
