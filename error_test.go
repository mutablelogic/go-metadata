package metadata_test

import (
	"errors"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
	rational "github.com/mutablelogic/go-metadata/pkg/rational"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

func Test_HTTPErr(t *testing.T) {
	// The errors of the packages which don't import this one, such as
	// pkg/geo's, are of their codes, as this package's are, when they're
	// wrapped too
	_, geoErr := geo.Parse("not a position")
	_, ratErr := rational.Parse("not a number")
	for name, err := range map[string]error{
		"metadata": metadata.ErrBadParameter.With("bad"),
		"geo":      geoErr,
		"rational": ratErr,
		"wrapped":  errors.Join(errors.New("reading"), geoErr),
	} {
		var httpErr httpresponse.Err
		if got := metadata.HTTPErr(err); !errors.As(got, &httpErr) || httpErr != httpresponse.ErrBadRequest {
			t.Errorf("%s: got %v, want a bad request", name, got)
		}
	}

	// Another error is internal
	var httpErr httpresponse.Err
	if got := metadata.HTTPErr(errors.New("other")); !errors.As(got, &httpErr) || httpErr != httpresponse.ErrInternalError {
		t.Errorf("other: got %v, want an internal error", got)
	}
}
