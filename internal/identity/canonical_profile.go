package identity

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// CanonicalProfile is a relay-owned synthetic presentation state derived
// deterministically from accountID + seed. It is NOT provider identity metadata.
type CanonicalProfile struct {
	Platform   string // "darwin", "linux"
	Shell      string // "zsh", "bash"
	OSVersion  string // "Darwin 24.4.0", "Linux 6.5.0-44-generic"
	WorkingDir string // "/Users/user/project"
	HomePrefix string // "/Users/user/"

	// Stainless-compatible values
	StainlessOS   string // "Mac OS X", "Linux"
	StainlessArch string // "arm64", "x64"
}

var profilePresets = []CanonicalProfile{
	{
		Platform: "darwin", Shell: "zsh",
		OSVersion: "Darwin 24.4.0", WorkingDir: "/Users/user/project",
		HomePrefix: "/Users/user/", StainlessOS: "Mac OS X", StainlessArch: "arm64",
	},
	{
		Platform: "darwin", Shell: "bash",
		OSVersion: "Darwin 23.6.0", WorkingDir: "/Users/user/project",
		HomePrefix: "/Users/user/", StainlessOS: "Mac OS X", StainlessArch: "arm64",
	},
	{
		Platform: "darwin", Shell: "zsh",
		OSVersion: "Darwin 24.1.0", WorkingDir: "/Users/user/project",
		HomePrefix: "/Users/user/", StainlessOS: "Mac OS X", StainlessArch: "x64",
	},
	{
		Platform: "linux", Shell: "bash",
		OSVersion: "Linux 6.5.0-44-generic", WorkingDir: "/home/user/project",
		HomePrefix: "/home/user/", StainlessOS: "Linux", StainlessArch: "x64",
	},
	{
		Platform: "linux", Shell: "zsh",
		OSVersion: "Linux 6.8.0-45-generic", WorkingDir: "/home/user/project",
		HomePrefix: "/home/user/", StainlessOS: "Linux", StainlessArch: "x64",
	},
	{
		Platform: "linux", Shell: "bash",
		OSVersion: "Linux 6.1.0-25-generic", WorkingDir: "/home/user/project",
		HomePrefix: "/home/user/", StainlessOS: "Linux", StainlessArch: "arm64",
	},
}

// DeriveCanonicalProfile deterministically generates a CanonicalProfile from
// accountID and a server-wide seed. No database storage required.
func DeriveCanonicalProfile(accountID, seed string) CanonicalProfile {
	h := sha256.Sum256([]byte(fmt.Sprintf("canonical:%s:%s", seed, accountID)))
	idx := binary.BigEndian.Uint64(h[:8]) % uint64(len(profilePresets))
	return profilePresets[idx]
}
