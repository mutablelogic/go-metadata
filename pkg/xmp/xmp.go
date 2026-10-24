package xmp

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Namespace is the namespace of the properties of the XMP basic schema,
// such as xmp:CreateDate
const Namespace = "xmp"

// The keys of the identifier of a document, and of its document ID, which
// is its identifier when it doesn't have one
const (
	identifierKey = "dc:identifier"
	documentIDKey = "xmpMM:DocumentID"
)

// defaultNamespaceURI is the start of the URI of a namespace prefix which
// isn't well known, such as image, and isn't given (see FromMetadata)
const defaultNamespaceURI = "urn:go-metadata:"

// Marshaler is implemented by the value of metadata which is written to an
// XMP document as properties of its own, rather than as its text, such as a
// position, which is exif:GPSLatitude and exif:GPSLongitude. MarshalXMP
// returns the properties, by "prefix:name", and their values, for the
// metadata with the key, in "prefix:name" form, or none when the value is
// written as its text.
type Marshaler interface {
	MarshalXMP(key string) map[string]string
}

// XMP holds a parsed XMP metadata document.
type XMP struct {
	about string  // rdf:about value (usually empty string)
	items []*Item // top-level properties
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns an empty XMP document.
func New() *XMP {
	return &XMP{}
}

// FromMetadata creates an XMP document from a list of metadata entries.
// The namespace of an entry is its namespace prefix, and its key is its
// local name. Entries without a namespace are added as unqualified XMP
// properties. An entry whose value is a Marshaler is the properties it
// returns, rather than its text.
//
// Optionally, callers can provide one namespace map of prefix -> URI to
// define extra namespaces (for example "audio" or "video").
func FromMetadata(items []metadata.Metadata, namespaces ...map[string]string) *XMP {
	x := &XMP{}
	var extra map[string]string
	if len(namespaces) > 0 {
		extra = namespaces[0]
	}

	type groupedItem struct {
		ns     string
		prefix string
		name   string
		values []string
	}

	order := make([]string, 0, len(items))
	grouped := make(map[string]*groupedItem, len(items))

	add := func(prefix, name, value string) {
		prefix, name = strings.TrimSpace(prefix), strings.TrimSpace(name)
		if name == "" {
			return
		}
		key, ns := name, ""
		if prefix != "" {
			key = prefix + string(metadata.Separator) + name
			ns = namespaceURIForPrefix(prefix, extra)
		}
		if _, exists := grouped[key]; !exists {
			order = append(order, key)
			grouped[key] = &groupedItem{ns: ns, prefix: prefix, name: name}
		}
		grouped[key].values = append(grouped[key].values, value)
	}

	for _, item := range items {
		if item == nil {
			continue
		}

		// A value which is properties of its own, in the order of their keys
		if m, ok := item.Any().(Marshaler); ok {
			key := item.NS() + string(metadata.Separator) + item.Key()
			if props := m.MarshalXMP(key); len(props) > 0 {
				for _, prop := range slices.Sorted(maps.Keys(props)) {
					prefix, name, ok := strings.Cut(prop, string(metadata.Separator))
					if !ok {
						prefix, name = "", prop
					}
					add(prefix, name, props[prop])
				}
				continue
			}
		}
		add(item.NS(), item.Key(), item.Value())
	}

	for _, key := range order {
		entry := grouped[key]
		if len(entry.values) <= 1 {
			x.Add(NewItem(entry.ns, entry.prefix, entry.name, entry.values[0]))
			continue
		}
		x.Add(NewSeq(entry.ns, entry.prefix, entry.name, entry.values...))
	}

	return x
}

// Parse parses an XMP document from a byte slice.
func Parse(data []byte) (*XMP, error) {
	return Read(bytes.NewReader(data))
}

// Read parses an XMP document from r.
func Read(r io.Reader) (*XMP, error) {
	return decode(r)
}

// Write encodes the XMP document as an XMP/RDF/XML packet to w, in the
// xpacket wrapper, to embed it in a file, such as a JPEG image.
func (x *XMP) Write(w io.Writer) error {
	return encode(w, x, true)
}

// WriteSidecar encodes the XMP document to w as a sidecar, a file of its
// own, such as photo.xmp, which starts with its x:xmpmeta element, without
// the xpacket wrapper.
func (x *XMP) WriteSidecar(w io.Writer) error {
	return encode(w, x, false)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// About returns the resource the document describes, its rdf:about, which
// is usually empty, for the file the document is embedded in
func (x *XMP) About() string {
	return x.about
}

// SetAbout sets the resource the document describes, its rdf:about, such as
// the path of a file, when the document isn't embedded in it
func (x *XMP) SetAbout(about string) {
	x.about = about
}

// Metadata returns the properties of the document as metadata, in the
// namespaces of their prefixes, such as dc:title (see Item.Metadata), and
// its xmpMM:DocumentID as dc:identifier too, when it doesn't have one, as
// it identifies the document, whichever version of it this is.
func (x *XMP) Metadata() []metadata.Metadata {
	result := make([]metadata.Metadata, 0, len(x.items)+1)
	for _, it := range x.items {
		if entry := it.Metadata(); entry != nil {
			result = append(result, entry)
		}
	}
	if x.First(identifierKey) == nil {
		if id := x.First(documentIDKey); id != nil {
			if entry := meta.New(dc.Namespace, dc.Identifier, id.Value()); entry != nil {
				result = append(result, entry)
			}
		}
	}
	return result
}

// Items returns all top-level metadata properties.
func (x *XMP) Items() []*Item {
	return x.items
}

// Get returns all items whose key equals key. The key may be "prefix:name" or
// just the local name.
func (x *XMP) Get(key string) []*Item {
	var out []*Item
	for _, it := range x.items {
		if it.Name() == key || it.name == key {
			out = append(out, it)
		}
	}
	return out
}

// Add appends items to the document.
func (x *XMP) Add(items ...*Item) {
	x.items = append(x.items, items...)
}

// First returns the first item with a non-empty value from the first matching
// key in the ordered list. This implements the priority-fallback pattern used
// when the same semantic field can appear under multiple namespace prefixes,
// e.g. x.First("photoshop:DateCreated", "exif:DateTimeOriginal", "xmp:CreateDate").
func (x *XMP) First(keys ...string) *Item {
	for _, key := range keys {
		for _, it := range x.items {
			if (it.Name() == key || it.name == key) && it.Value() != "" {
				return it
			}
		}
	}
	return nil
}

// Delete removes all items matching key and returns the count removed.
func (x *XMP) Delete(key string) int {
	kept := x.items[:0]
	n := 0
	for _, it := range x.items {
		if it.Name() == key || it.name == key {
			n++
		} else {
			kept = append(kept, it)
		}
	}
	x.items = kept
	return n
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// String returns the document encoded as an XMP/XML string.
func (x *XMP) String() string {
	var buf bytes.Buffer
	_ = x.Write(&buf)
	return buf.String()
}

// MarshalJSON implements json.Marshaler.
func (x *XMP) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		About string  `json:"about,omitempty"`
		Items []*Item `json:"items"`
	}{
		About: x.about,
		Items: x.items,
	})
}

func namespaceURIForPrefix(prefix string, extra map[string]string) string {
	if uri, ok := extra[prefix]; ok && uri != "" {
		return uri
	}
	for uri, known := range knownPrefixes {
		if known == prefix {
			return uri
		}
	}
	return defaultNamespaceURI + prefix
}
