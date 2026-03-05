package zen

import "errors"

// ErrInvalidBindTarget is returned by BindForm when dest is not a pointer to
// a struct. This is a programming error — the value never changes based on
// input, so a package-level sentinel avoids allocating a new error string on
// every call and allows callers to check it exactly with [errors.Is].
var ErrInvalidBindTarget = errors.New("BindForm dest must be a pointer to a struct")

// FormError is returned by BindForm when a form value cannot be converted to
// the target field's type (e.g. "abc" into an int field). It carries both the
// field name and the underlying cause so callers can inspect them independently
// without resorting to string parsing.
//
//	var fe *zen.FormError
//	if errors.As(err, &fe) {
//	    c.JSON(400, map[string]string{"field": fe.Field, "error": fe.Err.Error()})
//	}
type FormError struct {
	// Field is the form key that failed — resolved from the "json" struct tag
	// or the exported field name when no tag is present.
	Field string

	// Err is the underlying conversion error returned by the strconv package.
	Err error
}

// Error implements the error interface.
func (e *FormError) Error() string {
	return "BindForm field \"" + e.Field + "\": " + e.Err.Error()
}

// Unwrap lets [errors.Is] and [errors.As] traverse the cause chain, so
// callers can match against specific strconv sentinel errors if needed.
func (e *FormError) Unwrap() error { return e.Err }
