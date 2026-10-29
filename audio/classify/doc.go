// Package classify registers a handler of audio which classifies it, when
// it's asked for with metadata.WithClassifyAudio, as mostly speech, music,
// silence or other sound, from up to its first
// MaxDuration, with go-media's segmenter, which detects speech with libfvad,
// the WebRTC voice activity detector. It returns the class as audio:class,
// and the fractions of the audio which are speech, music and silence, from
// 0 to 1, as audio:speech, audio:music and audio:silence. Audio which is
// protected with DRM can't be decoded, so isn't classified. It decodes the
// audio, which takes about a second for each 8 minutes of it, which is why
// it's only done when it's asked for.
//
// It's only built with cgo and the libfvad build tag, which the Makefile
// sets when it finds libfvad.
package classify
