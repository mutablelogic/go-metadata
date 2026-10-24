// Package testassert checks the conditions of tests, as testify's assert
// and require do, so tests which use them can be ported without testify
package testassert

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Checker checks a condition of a test, and fails the test when it's false,
// which stops the test when it's fatal, as testify's require does, and
// otherwise carries on, as testify's assert does
type Checker struct {
	fatal bool
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// Assert fails a test when a condition is false, and carries on
	Assert = Checker{fatal: false}

	// Require fails a test when a condition is false, and stops it
	Require = Checker{fatal: true}
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// fail fails the test with the message
func (c Checker) fail(t *testing.T, message string, msgAndArgs ...any) bool {
	t.Helper()
	if len(msgAndArgs) > 0 {
		message += ": " + fmt.Sprint(msgAndArgs...)
	}
	if c.fatal {
		t.Fatal(message)
	} else {
		t.Error(message)
	}
	return false
}

func (c Checker) Equal(t *testing.T, want, got any, msgAndArgs ...any) bool {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		return c.fail(t, fmt.Sprintf("expected %#v, got %#v", want, got), msgAndArgs...)
	}
	return true
}

func (c Checker) NoError(t *testing.T, err error, msgAndArgs ...any) bool {
	t.Helper()
	if err != nil {
		return c.fail(t, fmt.Sprintf("unexpected error: %v", err), msgAndArgs...)
	}
	return true
}

func (c Checker) ErrorIs(t *testing.T, err, target error, msgAndArgs ...any) bool {
	t.Helper()
	if !errors.Is(err, target) {
		return c.fail(t, fmt.Sprintf("expected %v, got %v", target, err), msgAndArgs...)
	}
	return true
}

func (c Checker) True(t *testing.T, value bool, msgAndArgs ...any) bool {
	t.Helper()
	if !value {
		return c.fail(t, "expected true", msgAndArgs...)
	}
	return true
}

func (c Checker) Len(t *testing.T, object any, n int, msgAndArgs ...any) bool {
	t.Helper()
	if got := reflect.ValueOf(object).Len(); got != n {
		return c.fail(t, fmt.Sprintf("expected a length of %d, got %d: %#v", n, got, object), msgAndArgs...)
	}
	return true
}

func (c Checker) Empty(t *testing.T, object any, msgAndArgs ...any) bool {
	t.Helper()
	if !isEmpty(object) {
		return c.fail(t, fmt.Sprintf("expected empty, got %#v", object), msgAndArgs...)
	}
	return true
}

func (c Checker) NotEmpty(t *testing.T, object any, msgAndArgs ...any) bool {
	t.Helper()
	if isEmpty(object) {
		return c.fail(t, "expected not empty", msgAndArgs...)
	}
	return true
}

func (c Checker) Contains(t *testing.T, container, element any, msgAndArgs ...any) bool {
	t.Helper()
	if !contains(container, element) {
		return c.fail(t, fmt.Sprintf("expected %#v to contain %#v", container, element), msgAndArgs...)
	}
	return true
}

func (c Checker) NotContains(t *testing.T, container, element any, msgAndArgs ...any) bool {
	t.Helper()
	if contains(container, element) {
		return c.fail(t, fmt.Sprintf("expected %#v not to contain %#v", container, element), msgAndArgs...)
	}
	return true
}

func (c Checker) Less(t *testing.T, a, b any, msgAndArgs ...any) bool {
	t.Helper()
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	var less bool
	switch {
	case va.CanInt() && vb.CanInt():
		less = va.Int() < vb.Int()
	case va.CanUint() && vb.CanUint():
		less = va.Uint() < vb.Uint()
	case va.CanFloat() && vb.CanFloat():
		less = va.Float() < vb.Float()
	default:
		return c.fail(t, fmt.Sprintf("can't compare %#v and %#v", a, b), msgAndArgs...)
	}
	if !less {
		return c.fail(t, fmt.Sprintf("expected %v to be less than %v", a, b), msgAndArgs...)
	}
	return true
}

// isEmpty returns true when the object is nil, its type's zero value, or
// has a length of zero
func isEmpty(object any) bool {
	if object == nil {
		return true
	}
	v := reflect.ValueOf(object)
	switch v.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	}
	return v.IsZero()
}

// contains returns true when the container, which is a string, map or slice,
// contains the element: as a substring, a key, or an element
func contains(container, element any) bool {
	v := reflect.ValueOf(container)
	switch v.Kind() {
	case reflect.String:
		s, ok := element.(string)
		return ok && strings.Contains(v.String(), s)
	case reflect.Map:
		key := reflect.ValueOf(element)
		return key.Type().AssignableTo(v.Type().Key()) && v.MapIndex(key).IsValid()
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if reflect.DeepEqual(v.Index(i).Interface(), element) {
				return true
			}
		}
	}
	return false
}
