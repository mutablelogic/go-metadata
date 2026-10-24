// Package google registers a handler of the files which Google Drive for
// desktop creates for Google documents, such as .gdoc files, which return
// the type of the document, its identifier, and its URL
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// googleType is a type of Google document, its content type, and the URL of
// a document of the type, with its identifier, or "" when it's not known
type googleType struct {
	name, contentType, url string
}

// googleFile is the contents of a file which Google Drive for desktop
// creates for a Google document, such as a .gdoc file
type googleFile struct {
	ID          string `json:"doc_id"`
	ResourceKey string `json:"resource_key"`
	Email       string `json:"email"`
	URL         string `json:"url"`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of a Google document
	Namespace = "google"

	// maxFileSize is the largest Google document file which is read
	maxFileSize = 64 << 10
)

var (
	reContentType = regexp.MustCompile(`^application/vnd\.google-apps\.`)
)

// googleKinds are the kinds of Google documents, by their content type, of
// which a map has none
var googleKinds = map[string]string{
	"application/vnd.google-apps.document":     dc.KindDocument,
	"application/vnd.google-apps.spreadsheet":  dc.KindSpreadsheet,
	"application/vnd.google-apps.presentation": dc.KindPresentation,
	"application/vnd.google-apps.drawing":      dc.KindImage,
	"application/vnd.google-apps.form":         dc.KindForm,
}

// googleTypes are the types of Google documents, by their extension
var googleTypes = map[string]googleType{
	".gdoc":    {"document", "application/vnd.google-apps.document", "https://docs.google.com/document/d/%s"},
	".gsheet":  {"spreadsheet", "application/vnd.google-apps.spreadsheet", "https://docs.google.com/spreadsheets/d/%s"},
	".gslides": {"presentation", "application/vnd.google-apps.presentation", "https://docs.google.com/presentation/d/%s"},
	".gdraw":   {"drawing", "application/vnd.google-apps.drawing", "https://docs.google.com/drawings/d/%s"},
	".gform":   {"form", "application/vnd.google-apps.form", "https://docs.google.com/forms/d/%s"},
	".gmap":    {"map", "application/vnd.google-apps.map", "https://www.google.com/maps/d/viewer?mid=%s"},
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The files are JSON, which is detected as plain text, and refined by
	// their extension
	for ext, docType := range googleTypes {
		if err := mime.AddExtensionType(ext, docType.contentType); err != nil {
			panic(err)
		}
	}

	metadata.AddHandler(reContentType, Namespace, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		return append(googleMetadata(r), metadata.KindMetadata(googleKinds[o.ContentType()])...), nil
	}, dc.Namespace, Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// googleMetadata returns the metadata of a file which Google Drive for
// desktop creates for a Google document, from its extension and contents,
// or nil when it's not one, or its contents can't be read. The metadata is
// the type of the document, its identifier, resource key and the account's
// email, in the google namespace, and its URL as dc:identifier.
func googleMetadata(r io.Reader) []metadata.Metadata {
	named, ok := r.(metadata.NamedReader)
	if !ok {
		return nil
	}
	docType, ok := googleTypes[strings.ToLower(filepath.Ext(named.Name()))]
	if !ok {
		return nil
	}

	// Read the file, which is JSON
	data, err := io.ReadAll(io.LimitReader(r, maxFileSize))
	if err != nil {
		return nil
	}
	var file googleFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil
	}

	// The type, identifier, resource key and email
	result := []metadata.Metadata{meta.New(Namespace, "type", docType.name)}
	for _, field := range []struct{ key, value string }{
		{"id", file.ID},
		{"resource_key", file.ResourceKey},
		{"email", file.Email},
	} {
		if value := strings.TrimSpace(field.value); value != "" {
			result = append(result, meta.New(Namespace, field.key, value))
		}
	}

	// The URL, which is in the file, or from the type and identifier, with
	// the resource key, which is needed for some shared documents
	if u := strings.TrimSpace(file.URL); u != "" {
		result = append(result, meta.New(dc.Namespace, dc.Identifier, u))
	} else if id := strings.TrimSpace(file.ID); id != "" && docType.url != "" {
		u := fmt.Sprintf(docType.url, url.PathEscape(id))
		if key := strings.TrimSpace(file.ResourceKey); key != "" {
			separator := "?"
			if strings.Contains(u, "?") {
				separator = "&"
			}
			u += separator + "resourcekey=" + url.QueryEscape(key)
		}
		result = append(result, meta.New(dc.Namespace, dc.Identifier, u))
	}
	return result
}
