// Package input gives each handler of an input its own section of it, which
// it can read independently and concurrently, and seek in, copying an input
// which can't be read at an offset, such as a pipe, to a temporary file
package input

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Input gives content type detection, and each handler, its own section of
// the input, which it can read independently and concurrently, and seek in.
// When the input can't be read at an offset and seeked, such as a pipe, it's
// copied to a temporary file first, which is removed when the input is
// closed, so it's never read into memory as a whole.
type Input struct {
	name    string
	section func() *io.SectionReader
	temp    *os.File
}

// named is a reader with a name, such as a file
type named interface {
	Name() string
}

// namedReader wraps a section of the input with a Name()
type namedReader struct {
	*io.SectionReader
	name string
}

func (r namedReader) Name() string { return r.name }

// ctxReader is a reader which returns an error once its context is done
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r ctxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ErrTooLarge is returned when the input is larger than its maximum size
var ErrTooLarge = errors.New("input is too large")

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the input of r, from its current position, which is at
// most maxSize bytes, when maxSize is greater than zero. If r carries a
// name, it's preserved on each section, since detection uses its extension,
// and a handler may need the filename itself (e.g. to look up TMDB metadata)
// rather than the file's contents. The input must be closed.
func New(ctx context.Context, r io.Reader, maxSize int64) (*Input, error) {
	in := new(Input)
	if named, ok := r.(named); ok {
		in.name = named.Name()
	}

	// Read sections of r itself, when it can be read at an offset and seeked
	if ra, ok := r.(interface {
		io.ReaderAt
		io.Seeker
	}); ok {
		if start, size, err := remaining(ra); err == nil {
			if maxSize > 0 && size > maxSize {
				return nil, fmt.Errorf("%w: larger than %d bytes", ErrTooLarge, maxSize)
			}
			in.section = func() *io.SectionReader {
				return io.NewSectionReader(ra, start, size)
			}
			return in, nil
		}
	}

	// Otherwise copy r to a temporary file, and read sections of that
	temp, err := os.CreateTemp("", "metadata-*")
	if err != nil {
		return nil, err
	}
	in.temp = temp
	src := io.Reader(ctxReader{ctx, r})
	if maxSize > 0 {
		src = io.LimitReader(src, maxSize+1)
	}
	size, err := io.Copy(temp, src)
	if err == nil && maxSize > 0 && size > maxSize {
		err = fmt.Errorf("%w: larger than %d bytes", ErrTooLarge, maxSize)
	}
	if err != nil {
		return nil, errors.Join(err, in.Close())
	}
	in.section = func() *io.SectionReader {
		return io.NewSectionReader(temp, 0, size)
	}
	return in, nil
}

// Close removes the temporary file of the input, if there is one
func (in *Input) Close() error {
	if in.temp == nil {
		return nil
	}
	return errors.Join(in.temp.Close(), os.Remove(in.temp.Name()))
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Reader returns a new section of the input, from its start, with the name
// of the input, when it has one
func (in *Input) Reader() io.Reader {
	if in.name == "" {
		return in.section()
	}
	return namedReader{in.section(), in.name}
}

// remaining returns the current position of r, and the number of bytes from
// there to its end, leaving it at its current position
func remaining(r io.Seeker) (int64, int64, error) {
	start, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, 0, err
	}
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, 0, err
	}
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return 0, 0, err
	}
	return start, end - start, nil
}
