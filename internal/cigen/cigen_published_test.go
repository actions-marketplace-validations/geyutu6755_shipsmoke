package cigen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geyutu6755/shipsmoke/internal/config"
	"gopkg.in/yaml.v3"
)

// Tests for the post-release gate template. The property that matters most is
// that a generated file is actually runnable and cannot pass vacuously: it must
// parse as YAML, parse as a shipsmoke config, download real asset names, refuse
// placeholders, and keep the repo's no-interpolation-in-run-blocks rule.

func TestGeneratePublishedFillsAssetNames(t *testing.T) {
	assets := PublishedAssets{
		Linux:   "mytool-linux-amd64.tar.gz",
		Darwin:  "mytool-darwin-arm64.tar.gz",
		Windows: "mytool-windows-amd64.zip",
	}
	cfg, wf, err := GeneratePublished("mytool", assets)
	if err != nil {
		t.Fatal(err)
	}
	if HasPlaceholder(assets.Resolve()) {
		t.Error("HasPlaceholder true for fully specified assets")
	}
	for _, want := range []string{assets.Linux, assets.Darwin, assets.Windows} {
		if !strings.Contains(cfg, want) || !strings.Contains(wf, want) {
			t.Errorf("generated output missing asset %q", want)
		}
	}
	// The explanatory note mentions the marker word, so the assertion belongs on
	// the artifact paths themselves, not on the whole file.
	for _, line := range strings.Split(cfg, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "path:") && strings.Contains(line, "SET-ME") {
			t.Errorf("a placeholder path survived a complete asset specification: %s", line)
		}
	}
	// The config must load under the strict schema, and name one artifact per OS.
	parsed, err := config.Parse([]byte(cfg))
	if err != nil {
		t.Fatalf("published config does not validate: %v", err)
	}
	if len(parsed.Artifacts) != 3 {
		t.Errorf("artifacts = %d, want 3", len(parsed.Artifacts))
	}
	names := map[string]bool{}
	for _, a := range parsed.Artifacts {
		names[a.Name] = true
		if a.Path == "" || a.Binary == "" || len(a.Checks) == 0 {
			t.Errorf("artifact %+v is incomplete", a)
		}
	}
	for _, want := range []string{"mytool-linux", "mytool-macos", "mytool-windows"} {
		if !names[want] {
			t.Errorf("missing artifact %q (have %v)", want, names)
		}
	}
	// Windows must target windows, not unix, or the platform gate skips it.
	for _, a := range parsed.Artifacts {
		if strings.HasSuffix(a.Name, "-windows") {
			if !strings.Contains(strings.Join(a.RunsOn, ","), "windows/") {
				t.Errorf("windows artifact runs_on = %v", a.RunsOn)
			}
			if !strings.HasSuffix(a.Binary, ".exe") {
				t.Errorf("windows binary = %q, want .exe", a.Binary)
			}
		}
	}
}

func TestGeneratePublishedPlaceholdersAreVisible(t *testing.T) {
	cfg, wf, err := GeneratePublished("mytool", PublishedAssets{})
	if err != nil {
		t.Fatal(err)
	}
	if !HasPlaceholder(PublishedAssets{}.Resolve()) {
		t.Error("HasPlaceholder(false) after resolving unspecified assets")
	}
	var placeholderPaths int
	for _, line := range strings.Split(cfg, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "path:") && strings.Contains(line, "SET-ME") {
			placeholderPaths++
		}
	}
	if placeholderPaths != 3 {
		t.Errorf("config placeholder paths = %d, want 3, got:\n%s", placeholderPaths, cfg)
	}

	// The workflow's matrix must carry the same placeholders so its guard can
	// actually see them, and the guard must run before any download.
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(wf), &doc); err != nil {
		t.Fatalf("published workflow is not valid YAML: %v", err)
	}
	jobs, _ := doc["jobs"].(map[string]any)
	job, _ := jobs["verify-published"].(map[string]any)
	strat, _ := job["strategy"].(map[string]any)
	mx, _ := strat["matrix"].(map[string]any)
	inc, _ := mx["include"].([]any)
	if len(inc) != 3 {
		t.Fatalf("matrix include = %d, want 3", len(inc))
	}
	for _, e := range inc {
		m, _ := e.(map[string]any)
		asset, _ := m["asset"].(string)
		if !strings.HasPrefix(asset, "SET-ME") {
			t.Errorf("matrix asset %q is not a visible placeholder", asset)
		}
	}
	guard := strings.Index(wf, "Refuse unfilled asset placeholders")
	download := strings.Index(wf, "Download the published asset")
	if guard < 0 || download < 0 {
		t.Fatal("expected both the guard step and the download step")
	}
	if guard > download {
		t.Error("placeholder guard must run before the download step")
	}
}

func TestGeneratePublishedInvalidName(t *testing.T) {
	if _, _, err := GeneratePublished("bad name", PublishedAssets{}); err == nil {
		t.Error("GeneratePublished should reject an invalid tool name")
	}
}

func TestPublishedWorkflowStructure(t *testing.T) {
	_, wf, err := GeneratePublished("mytool", PublishedAssets{
		Linux: "a.tar.gz", Darwin: "b.tar.gz", Windows: "c.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(wf), &doc); err != nil {
		t.Fatalf("published workflow is not valid YAML: %v", err)
	}
	on, ok := doc["on"].(map[string]any)
	if !ok {
		t.Fatalf("workflow has no triggers (top-level: %v)", doc)
	}
	if _, ok := on["release"]; !ok {
		t.Error("post-release gate must trigger on release")
	}
	types, _ := yaml.Marshal(on["release"])
	if !strings.Contains(string(types), "published") {
		t.Errorf("must trigger on release:published, got %s", types)
	}
	if _, ok := on["workflow_dispatch"]; !ok {
		t.Error("must be dispatchable so an existing release can be checked without publishing")
	}
	jobs, _ := doc["jobs"].(map[string]any)
	job, ok := jobs["verify-published"].(map[string]any)
	if !ok {
		t.Fatal("missing verify-published job")
	}
	if ro, _ := job["runs-on"].(string); ro != "${{ matrix.os }}" {
		t.Errorf("runs-on = %q", ro)
	}
	strat, _ := job["strategy"].(map[string]any)
	mx, _ := strat["matrix"].(map[string]any)
	inc, _ := mx["include"].([]any)
	if len(inc) != 3 {
		t.Fatalf("matrix include = %d entries, want 3", len(inc))
	}
	seenOS := map[string]bool{}
	seenArt := map[string]bool{}
	for _, e := range inc {
		m, _ := e.(map[string]any)
		seenOS[m["os"].(string)] = true
		seenArt[m["artifact"].(string)] = true
	}
	for _, want := range []string{"ubuntu-24.04", "macos-15", "windows-2025"} {
		if !seenOS[want] {
			t.Errorf("matrix missing runner %s", want)
		}
	}
	// Every artifact name in the workflow must exist in the config, or the
	// --artifact filter would silently select nothing.
	cfg, _, _ := GeneratePublished("mytool", PublishedAssets{Linux: "a.tar.gz", Darwin: "b.tar.gz", Windows: "c.zip"})
	parsed, err := config.Parse([]byte(cfg))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range parsed.Artifacts {
		if !seenArt[a.Name] {
			t.Errorf("config artifact %q is not in the workflow matrix", a.Name)
		}
	}
	if !strings.Contains(wf, "geyutu6755/shipsmoke@v1") {
		t.Error("must use the shipsmoke action")
	}
	if !strings.Contains(wf, "version: v1.0.4") {
		t.Error("must pin a released shipsmoke version")
	}
	// The shell text must survive Go's format engine: a bare %s inside the
	// template is a positional verb, and it silently substitutes an argument
	// (observed: printf '%s' "$TAG" rendered as the upload-artifact ref).
	if !strings.Contains(wf, `printf '%s' "$TAG" | grep -Eq`) {
		t.Errorf("tag grammar line was mangled by format substitution:\n%s", wf)
	}
	if strings.Contains(wf, `%!`) {
		t.Error("a format verb was left unresolved")
	}
	// The grammar itself must be anchored and single-line: a shell case pattern
	// like v[0-9]* accepts newlines and '../'.
	if strings.Contains(wf, "case \"$TAG\" in") {
		t.Error("tag validation regressed to a shell case pattern, which is not a validator")
	}
	for _, want := range []string{"^(v)?[0-9]+\\.[0-9]+", "grep -Eq"} {
		if !strings.Contains(wf, want) {
			t.Errorf("tag grammar missing %q", want)
		}
	}
}

// TestGeneratedWorkflowsNeverInterpolateInRunBlocks enforces the repo-wide
// script-injection rule on both templates: every run block must take values
// through env:, never through an expanded ${{ }} expression.
func TestGeneratedWorkflowsNeverInterpolateInRunBlocks(t *testing.T) {
	assets := PublishedAssets{Linux: "a.tar.gz", Darwin: "b.tar.gz", Windows: "c.zip"}
	_, publishedWF, err := GeneratePublished("mytool", assets)
	if err != nil {
		t.Fatal(err)
	}
	workflows := map[string]string{"published": publishedWF}
	for _, stack := range []Stack{StackGo, StackRust, StackC} {
		_, w, err := Generate(stack, "mytool")
		if err != nil {
			t.Fatal(err)
		}
		workflows[string(stack)] = w // checked separately: concatenated YAML would collide
	}
	for name, wf := range workflows {
		var doc map[string]any
		if err := yaml.Unmarshal([]byte(wf), &doc); err != nil {
			t.Fatalf("%s: invalid YAML: %v", name, err)
		}
		jobs, _ := doc["jobs"].(map[string]any)
		for jobName, jv := range jobs {
			job, _ := jv.(map[string]any)
			steps, _ := job["steps"].([]any)
			for _, sv := range steps {
				step, _ := sv.(map[string]any)
				run, ok := step["run"].(string)
				if !ok {
					continue
				}
				if strings.Contains(run, "${{") {
					t.Errorf("%s/%s: run block interpolates an expression (script injection): %s", name, jobName, run)
				}
				if strings.Contains(run, "<<") {
					t.Errorf("%s/%s: run block uses a heredoc (v1.0.1 fake-green pattern): %s", name, jobName, run)
				}
			}
		}
		// Third-party actions must stay SHA-pinned in generated output. Checked
		// per line with an anchored pattern so trailing text (a comment, the
		// literal shell text of a nearby step) cannot confuse the parse.
		shaForm := regexp.MustCompile(`actions/(?:checkout|setup-go|upload-artifact|download-artifact)@[0-9a-f]{40}(?:\s|$)`)
		movable := regexp.MustCompile(`actions/(?:checkout|setup-go|upload-artifact|download-artifact)@v[0-9]`)
		if movable.MatchString(wf) {
			t.Errorf("%s: a third-party action is pinned to a movable tag", name)
		}
		wanted := 1
		if name == "go" {
			wanted = 2 // checkout + setup-go
		}
		if got := len(shaForm.FindAllString(wf, -1)); got < wanted {
			t.Errorf("%s: found %d SHA-pinned action refs, expected at least %d", name, got, wanted)
		}
	}
}

// normalizeYAML drops comment lines and blanks so a doc that reproduces the
// generated file "with comments elided" can still be compared to it.
func normalizeYAML(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		kept = append(kept, strings.TrimRight(line, " \t"))
	}
	return strings.Join(kept, "\n")
}

// TestDocsConfigurationMatchesGenerator is the drift check its title claims:
// docs/CONFIGURATION.md reproduces the generated post-release files, so the
// page must contain the generator's actual YAML once comments are removed.
func TestDocsConfigurationMatchesGenerator(t *testing.T) {
	cfg, wf, err := GeneratePublished("mytool", PublishedAssets{
		Linux:   "mytool-linux-amd64.tar.gz",
		Darwin:  "mytool-darwin-arm64.tar.gz",
		Windows: "mytool-windows-amd64.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{cfg, wf} {
		if strings.Contains(s, "%!") || strings.Contains(s, "%[") {
			t.Errorf("unsubstituted format verb leaked into output:\n%s", s)
		}
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "CONFIGURATION.md"))
	if err != nil {
		t.Skipf("docs/CONFIGURATION.md not reachable from the test: %v", err)
	}
	doc := normalizeYAML(string(raw))
	for name, want := range map[string]string{".shipsmoke.yml": cfg, "workflow": wf} {
		if !strings.Contains(doc, normalizeYAML(want)) {
			t.Errorf("docs/CONFIGURATION.md no longer matches the generated %s; regenerate the page\n--- generated ---\n%s", name, want)
		}
	}
}

// TestPublishedRejectsHostileAssetNames covers the boundary the generator writes
// into two YAML files: a value with a newline could close the mapping and inject
// keys into a workflow that runs with the caller's token.
func TestPublishedRejectsHostileAssetNames(t *testing.T) {
	bad := []string{
		"mytool-linux.tar.gz\n    steps:\n      - run: echo pwned",
		"*leading-alias.tar.gz",
		"has space.tar.gz",
		"hash#comment.tar.gz",
		"quote\"name.tar.gz",
		"../escape.tar.gz",
		"back\\slash.tar.gz",
		"tab\tname.tar.gz",
		"", // empty means "use the placeholder", so it must stay acceptable
	}
	for _, s := range bad {
		if s == "" {
			if err := ValidateAsset(s); err != nil {
				t.Errorf("empty asset should be allowed (becomes a placeholder), got %v", err)
			}
			continue
		}
		if err := ValidateAsset(s); err == nil {
			t.Errorf("ValidateAsset(%q) = nil, want rejection", s)
		}
		if _, _, err := GeneratePublished("mytool", PublishedAssets{Linux: s}); err == nil {
			t.Errorf("GeneratePublished accepted hostile asset %q", s)
		}
	}
	for _, s := range []string{"mytool-linux-amd64.tar.gz", "tool_v1.2.3-x86_64.zip", "a.tgz", "tool-1.0.0darwin.tar.gz"} {
		if err := ValidateAsset(s); err != nil {
			t.Errorf("ValidateAsset(%q) = %v, want acceptance", s, err)
		}
	}
}
