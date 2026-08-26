package bytesconv

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
