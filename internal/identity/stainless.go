package identity

import (
	"net/http"
	"strings"
)

// RemoveAllStainless strips all x-stainless-* headers from a header set.
func RemoveAllStainless(h http.Header) {
	for key := range h {
		if strings.HasPrefix(strings.ToLower(key), StainlessPrefix) {
			h.Del(key)
		}
	}
}
