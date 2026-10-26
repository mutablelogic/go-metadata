package office_test

import (
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

// hintOf returns the hints of the metadata, by key, or nothing
func hintOf(meta map[string]metadata.Metadata) string {
	if m, exists := meta[metadata.MetadataNamespace+string(metadata.Separator)+metadata.HintKey]; exists {
		return m.Value()
	}
	return ""
}

func TestHints(t *testing.T) {
	t.Run("Office", func(t *testing.T) {
		// A document with a body has Markdown, and with a thumbnail, artwork,
		// without either, it has neither
		meta, err := entries(t, document(t, map[string]string{"docProps/core.xml": coreXML, "word/document.xml": "<w:document/>", "docProps/thumbnail.jpeg": string(coverPNG(t))}))
		require.NoError(t, err)
		assert.Equal(t, "artwork,markdown", hintOf(meta))
		meta, err = entries(t, document(t, map[string]string{"docProps/core.xml": coreXML, "word/document.xml": "<w:document/>"}))
		require.NoError(t, err)
		assert.Equal(t, "markdown", hintOf(meta))
		meta, err = entries(t, document(t, map[string]string{"docProps/core.xml": coreXML}))
		require.NoError(t, err)
		assert.Equal(t, "", hintOf(meta))
	})

	t.Run("OpenDocument", func(t *testing.T) {
		// A document with content has Markdown, and with a thumbnail, artwork
		meta, err := odfEntries(t, "/docs/report.odt", document(t, map[string]string{"mimetype": odtType, "meta.xml": odfMetaXML, "content.xml": "<office:document-content/>", "Thumbnails/thumbnail.png": string(coverPNG(t))}))
		require.NoError(t, err)
		assert.Equal(t, "artwork,markdown", hintOf(meta))
		meta, err = odfEntries(t, "/docs/report.odt", document(t, map[string]string{"mimetype": odtType, "meta.xml": odfMetaXML, "content.xml": "<office:document-content/>"}))
		require.NoError(t, err)
		assert.Equal(t, "markdown", hintOf(meta))
	})
}
