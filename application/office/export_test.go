package office

// SetMaxDocumentSize sets the largest document which is read into memory,
// and returns a function which restores it
func SetMaxDocumentSize(size int64) func() {
	previous := maxDocumentSize
	maxDocumentSize = size
	return func() { maxDocumentSize = previous }
}
