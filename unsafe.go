package zen

import (
	"unsafe"
)

// StringToBytes converts a string to a byte slice without allocations.
// The returned bytes MUST NOT be modified.
func StringToBytes(s string) []byte {
	if s == "" {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BytesToString converts a byte slice to a string without allocations.
func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}
