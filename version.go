package ensign

import "fmt"

// Version component constants for the current build.
const (
	VersionMajor         = 0
	VersionMinor         = 8
	VersionPatch         = 0
	VersionReleaseLevel  = "beta"
	VersionReleaseNumber = 7
)

// Version returns the semantic version for the current build.
func Version() string {
	versionCore := fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionPatch)

	if VersionReleaseLevel != "" {
		if VersionReleaseNumber > 0 {
			versionCore = fmt.Sprintf("%s-%s.%d", versionCore, VersionReleaseLevel, VersionReleaseNumber)
		} else {
			versionCore = fmt.Sprintf("%s-%s", versionCore, VersionReleaseLevel)
		}
	}

	return versionCore
}
