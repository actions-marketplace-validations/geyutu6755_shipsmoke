package report

import (
	"os"
	"regexp"
)

// Secret-name policy: variables whose names match this pattern never appear
// as values in reports (values are masked wherever they occur). This mirrors
// the runner's child-env policy; the two are intentionally identical.
var secretNameRe = regexp.MustCompile(`(?i)(token|secret|password|passwd|credential|apikey|api_key|private_?key|_key$|_key_)`)

func isSecretVar(name string) bool {
	return secretNameRe.MatchString(name)
}

func environ() []string { return os.Environ() }
