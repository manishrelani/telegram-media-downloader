package tdl

import (
	"regexp"
	"strconv"
	"strings"
)

// MinimumSafeManagedDownloadVersion is the first tdl release whose changelog includes
// the fix for concurrent map access while persisting interrupted download progress.
// We require this floor before exposing the app-managed download lifecycle because
// Pause/Cancel/Resume deliberately interrupt the tdl child process.
const MinimumSafeManagedDownloadVersion = "0.20.4"

var semanticVersionPattern = regexp.MustCompile(`(?i)(?:^|[^0-9])v?(\d+)\.(\d+)\.(\d+)(?:-([0-9a-z.-]+))?(?:\+[0-9a-z.-]+)?(?:$|[\s,;])`)

type semanticVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func parseSemanticVersion(output string) (semanticVersion, bool) {
	match := semanticVersionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(match) != 5 {
		return semanticVersion{}, false
	}
	major, errMajor := strconv.Atoi(match[1])
	minor, errMinor := strconv.Atoi(match[2])
	patch, errPatch := strconv.Atoi(match[3])
	if errMajor != nil || errMinor != nil || errPatch != nil {
		return semanticVersion{}, false
	}
	return semanticVersion{major: major, minor: minor, patch: patch, prerelease: match[4]}, true
}

// SupportsManagedDownloadVersion returns whether the installed tdl output identifies
// a stable release at or above the minimum version verified for managed interruption
// and continuation. Unknown/development version strings fail closed.
func SupportsManagedDownloadVersion(output string) bool {
	version, ok := parseSemanticVersion(output)
	if !ok || version.prerelease != "" {
		return false
	}
	minimum := semanticVersion{major: 0, minor: 20, patch: 4}
	if version.major != minimum.major {
		return version.major > minimum.major
	}
	if version.minor != minimum.minor {
		return version.minor > minimum.minor
	}
	if version.patch != minimum.patch {
		return version.patch > minimum.patch
	}
	return true
}
