package identity

import "regexp"

var (
	rePlatform   = regexp.MustCompile(`(Platform:\s*)\S+`)
	reShell      = regexp.MustCompile(`(Shell:\s*)\S+`)
	reOSVersion  = regexp.MustCompile(`(OS Version:\s*)[^\n<]+`)
	reWorkingDir = regexp.MustCompile(`((?:Primary )?[Ww]orking directory:\s*)/\S+`)
	reHomePath   = regexp.MustCompile(`/(?:Users|home)/[^/\s]+/`)
	reEnvBlock   = regexp.MustCompile(`(?s)(<env>)(.*?)(</env>)`)
)

// SanitizePromptEnv replaces environment-identifying text ONLY within <env>
// blocks injected by Claude Code. Text outside <env> blocks is untouched,
// preserving user-authored content that may legitimately contain paths or
// platform references.
func SanitizePromptEnv(text string, profile CanonicalProfile) string {
	return reEnvBlock.ReplaceAllStringFunc(text, func(match string) string {
		parts := reEnvBlock.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		inner := parts[2]
		inner = rePlatform.ReplaceAllString(inner, "${1}"+profile.Platform)
		inner = reShell.ReplaceAllString(inner, "${1}"+profile.Shell)
		inner = reOSVersion.ReplaceAllString(inner, "${1}"+profile.OSVersion)
		inner = reWorkingDir.ReplaceAllString(inner, "${1}"+profile.WorkingDir)
		inner = reHomePath.ReplaceAllString(inner, profile.HomePrefix)
		return parts[1] + inner + parts[3]
	})
}
