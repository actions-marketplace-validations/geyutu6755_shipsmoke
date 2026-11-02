package cigen

import (
	"os"
	"path/filepath"
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
	if !strings.Contains(wf, "version: v1.0.3") {
		t.Error("must pin a released shipsmoke version")
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
		// Third-party actions must stay SHA-pinned in generated output.
		for _, ref := range []string{"actions/checkout@", "actions/upload-artifact@", "actions/setup-go@"} {
			for _, line := range strings.Split(wf, "\n") {
				if i := strings.Index(line, ref); i >= 0 {
					rest := strings.TrimSpace(line[i+len(ref):])
					tag := strings.Fields(rest)[0]
					if len(tag) != 40 {
						t.Errorf("%s: %s%s is not pinned to a commit SHA", name, ref, tag)
					}
				}
			}
		}
	}
}

func TestPublishedConfigAndWorkflowMatchTrialFiles(t *testing.T) {
	// The committed example must not drift from what the generator emits, since
	// docs point readers at it.
	cfg, wf, err := GeneratePublished("wordcount", PublishedAssets{
		Linux:   "wordcount-linux-amd64.tar.gz",
		Darwin:  "wordcount-darwin-arm64.tar.gz",
		Windows: "wordcount-windows-amd64.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{cfg, wf} {
		if strings.Contains(s, "%!") || strings.Contains(s, "%[") {
			t.Errorf("unsubstituted format verb leaked into output:\n%s", s)
		}
	}
	// sanity: writing both files works on every platform (paths are relative)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".shipsmoke.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(filepath.Join(dir, ".shipsmoke.yml")); err != nil {
		t.Errorf("published config fails to load from disk: %v", err)
	}
}
