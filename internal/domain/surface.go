package domain

import "strings"

// Surface identifies a request surface exposed by the broker.
type Surface string

const (
	SurfaceNative Surface = "native"
)

func NormalizeSurface(value string) Surface {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(SurfaceNative), "all", "compat":
		return SurfaceNative
	default:
		return ""
	}
}

func AllowsSurface(allowed, requested Surface) bool {
	return true
}
