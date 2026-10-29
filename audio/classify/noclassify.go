//go:build !(cgo && libfvad)

package classify

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Enabled is false, as audio is only classified when it's built with cgo and
// the libfvad tag
const Enabled = false
