package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// HostOS / HostArch are the real platform shipsmoke is running on.
func HostOS() string   { return runtime.GOOS }
func HostArch() string { return runtime.GOARCH }

// MatchesHost reports whether the platform spec "os/arch" (or "any")
// describes the current host.
func MatchesHost(spec string) bool {
	if spec == "any" || spec == "" {
		return true
	}
	osArch := strings.SplitN(spec, "/", 2)
	if len(osArch) != 2 {
		return false
	}
	return osArch[0] == runtime.GOOS && osArch[1] == runtime.GOARCH
}

// RunsOnHost reports whether any spec in the list matches the host.
func RunsOnHost(specs []string) bool {
	if len(specs) == 0 {
		return true // empty means any
	}
	for _, s := range specs {
		if MatchesHost(s) {
			return true
		}
	}
	return false
}

// FormatRunsOn renders a runs_on list for messages.
func FormatRunsOn(specs []string) string {
	if len(specs) == 0 {
		return "any"
	}
	return strings.Join(specs, ", ")
}

// FindEntry locates the entry binary inside the extraction directory.
// relPath is a relative path using forward slashes. The returned path is
// absolute. A missing binary produces an error that includes the top-level
// contents of the directory for diagnosis.
func FindEntry(dir, relPath string, archivePath string, format Format) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("entry binary path is empty")
	}
	if strings.Contains(relPath, "\\") {
		return "", fmt.Errorf("binary path %q uses backslashes: use forward slashes, e.g. \"bin/tool\"", relPath)
	}
	if filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "/") {
		return "", fmt.Errorf("binary path %q must be relative to the artifact root", relPath)
	}
	segs := strings.Split(relPath, "/")
	for _, s := range segs {
		if s == ".." {
			return "", fmt.Errorf("binary path %q must not contain \"..\"", relPath)
		}
	}
	dest := filepath.Join(dir, filepath.FromSlash(relPath))
	st, err := os.Stat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			top := ListTop(archivePath, format)
			if len(top) > 0 {
				return "", fmt.Errorf("entry binary %q not found in the artifact; top-level contents: %s", relPath, strings.Join(top, ", "))
			}
			return "", fmt.Errorf("entry binary %q not found in the artifact", relPath)
		}
		return "", fmt.Errorf("cannot access entry binary %q: %w", relPath, err)
	}
	if st.IsDir() {
		return "", fmt.Errorf("entry binary %q is a directory, not a file", relPath)
	}
	return dest, nil
}

// HostDescription renders the host platform for reports.
func HostDescription() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
