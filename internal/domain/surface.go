package domain

import "strings"

// Surface identifies a request surface exposed by the broker.
type Surface string

const (
	SurfaceNative Surface = "native"
	SurfaceCompat Surface = "compat"
	SurfaceAll    Surface = "all"
)

func NormalizeSurface(value string) Surface {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(SurfaceNative):
		return SurfaceNative
	case string(SurfaceCompat):
		return SurfaceCompat
	case string(SurfaceAll):
		return SurfaceAll
	default:
		return ""
	}
}

func AllowsSurface(allowed, requested Surface) bool {
	if allowed == "" {
		allowed = SurfaceNative
	}
	switch allowed {
	case SurfaceAll:
		return true
	default:
		return requested != "" && allowed == requested
	}
}
