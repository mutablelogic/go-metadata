// Package errs is the errors of the packages of pkg, which are of the
// standard library alone, such as pkg/geo, which are go-metadata's, of their
// codes, without importing it
package errs

import (
	"fmt"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// codeBadParameter is the code of go-metadata's ErrBadParameter, which is
// HTTP's 400 Bad Request
const codeBadParameter = 400

////////////////////////////////////////////////////////////////////////////////
// TYPES

// badParameter is an error of a parameter which isn't valid, such as a position,
// which is go-metadata's ErrBadParameter, of its code, without importing
// it, so the packages which return it, such as pkg/geo, are of the standard
// library alone, for those which only need them
type badParameter struct {
	message string
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// BadParameter returns an error of a parameter which isn't valid
func BadParameter(format string, args ...any) error {
	return badParameter{message: fmt.Sprintf(format, args...)}
}

////////////////////////////////////////////////////////////////////////////////
// ERROR

func (e badParameter) Error() string {
	return "bad parameter: " + e.message
}

// Code returns the error's code, which is codeBadParameter, as
// go-metadata's ErrBadParameter's is
func (e badParameter) Code() uint {
	return codeBadParameter
}

// Is returns true of an error which is of the same code, such as
// go-metadata's ErrBadParameter, so errors.Is(err, metadata.ErrBadParameter)
// is true of it
func (e badParameter) Is(target error) bool {
	coded, ok := target.(interface{ Code() uint })
	return ok && coded.Code() == codeBadParameter
}
