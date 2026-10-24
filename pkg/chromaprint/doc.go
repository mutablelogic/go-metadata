// Package chromaprint makes the Chromaprint fingerprint of audio, which
// AcoustID looks up (see the acoustid package). It uses the Chromaprint
// library, and FFmpeg to decode the audio, so it's only built with cgo, and
// the chromaprint build tag, which the Makefile sets when it finds
// Chromaprint.
package chromaprint
