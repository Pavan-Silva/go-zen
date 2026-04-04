package zen

import "errors"

// ErrInvalidBindTarget is returned by BindForm when dest is not a pointer to a struct.
// This is a programming error (e.g., passing a bare struct instead of &struct).
// Using a sentinel error avoids allocation and allows exact comparison with errors.Is.
//
// Example:
//
//	var person Person  // Wrong: bare struct
//	if err := c.BindForm(person); err != nil {  // Will be ErrInvalidBindTarget
//	    ...
//	}
//
//	var person Person
//	if err := c.BindForm(&person); err != nil {  // Correct
//	    ...
//	}
var ErrInvalidBindTarget = errors.New("zen: BindForm dest must be a pointer to a struct")

// FormError is returned by BindForm when a form value cannot be parsed into
// the target field's type (e.g., "abc" into an int field).
//
// Unlike generic errors, FormError provides structured access to the field name
// and underlying cause, allowing callers to generate precise error messages
// without string parsing.
//
// Example:
//
//	err := c.BindForm(&data)
//	if err != nil {
//	    var fe *zen.FormError
//	    if errors.As(err, &fe) {
//	        // Structured error: fe.Field and fe.Err
//	        c.JSON(http.StatusBadRequest, map[string]string{
//	            "field": fe.Field,
//	            "error": fe.Err.Error(),  // e.g., "value out of range"
//	        })
//	        return
//	    }
//	    // Handle other errors (validation, parsing, etc)
//	}
type FormError struct {
	// Field is the form key that failed (resolved from "json" tag or field name).
	Field string

	// Err is the underlying conversion error from strconv (e.g., strconv.ErrRange).
	Err error
}

// Error implements the error interface.
func (e *FormError) Error() string {
	return "zen: BindForm field \"" + e.Field + "\": " + e.Err.Error()
}

// Unwrap returns the underlying cause error, enabling errors.Is/As to traverse
// the error chain. This allows callers to match specific strconv errors:
//
//	if errors.Is(err, strconv.ErrRange) {
//	    // Field value was out of range for its type
//	}
func (e *FormError) Unwrap() error { return e.Err }
