package version

import "fmt"

// These values are intended to be populated via ldflags at build time.
var (
	// Version is the application version (semantic version or git tag).
	Version = "dev"
	// GitCommit is the git commit hash of the build.
	GitCommit = "unknown"
	// BuildDate is the build timestamp in RFC3339 format.
	BuildDate = "unknown"
)

// Info captures build metadata for exposure via diagnostics endpoints.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
}

// Get returns the current build info.
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}
}

// String renders a human readable summary.
func String() string {
	info := Get()
	return fmt.Sprintf("%s (%s) built %s", info.Version, info.GitCommit, info.BuildDate)
}
