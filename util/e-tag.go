package util

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// GenerateETag generates an ETag (Entity Tag) hash for the given content.
// ETags are used for HTTP caching and conditional requests to determine if a resource has changed.
//
// The function supports multiple input types:
//   - []byte: Raw byte data
//   - string: String content
//   - any other type: JSON marshaled representation
//
// The ETag is generated using SHA-1 hash of the content and returned as a hexadecimal string.
func GenerateETag(content any) string {
	var data []byte
	var err error

	switch v := content.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		// For other types, try to marshal to JSON
		data, err = json.Marshal(content)
		if err != nil {
			// Fallback to string representation using fmt.Appendf
			data = fmt.Appendf(nil, "%v", content)
		}
	}

	hash := sha1.Sum(data)
	return hex.EncodeToString(hash[:])
}
