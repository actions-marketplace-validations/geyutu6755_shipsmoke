package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunHelpAndVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"--help"}, &out, &errOut, VersionInfo{Version: "test"}); code != 0 {
		t.Errorf("help exit = %d", code)
	}
	if !strings.Contains(out.String(), "test what users actually download") {
		t.Error("help text missing tagline")
	}
	if !strings.Contains(out.String(), "Security note") || !strings.Contains(out.String(), "sandbox") {
		t.Error("help must carry the security disclaimer")
	}
	out.Reset()
	if code := Run([]string{"version"}, &out, &errOut, VersionInfo{Version: "9.8.7", Commit: "abc"}); code != 0 {
		t.Errorf("version exit = %d", code)
	}
	if !strings.Contains(out.String(), "shipsmoke 9.8.7") || !strings.Contains(out.String(), "abc") {
		t.Errorf("version output = %q", out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"bogus"}, &out, &errOut, VersionInfo{}); code != 2 {
		t.Errorf("unknown command exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRunInitAndValidateAndCheck(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	// init
	if code := Run([]string{"init"}, &out, &errOut, VersionInfo{}); code != 0 {
		t.Fatalf("init exit = %d, stderr=%s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".shipsmoke.yml")); err != nil {
		t.Fatal("init did not write config")
	}
	// init again must refuse
	errOut.Reset()
	if code := Run([]string{"init"}, &out, &errOut, VersionInfo{}); code != 2 {
		t.Errorf("second init exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "already exists") {
		t.Errorf("second init stderr = %q", errOut.String())
	}
	// validate: example config references non-existent dist files but
	// validation does not touch the filesystem.
	out.Reset()
	if code := Run([]string{"validate"}, &out, &errOut, VersionInfo{}); code != 0 {
		t.Errorf("validate exit = %d, stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "OK") {
		t.Errorf("validate output = %q", out.String())
	}
	// check with the example config: its only artifact targets linux/amd64.
	// On linux the missing file is an ERROR (exit 1); elsewhere everything
	// is skipped, which must fail closed (exit 3).
	{
		code := Run([]string{"check"}, &out, &errOut, VersionInfo{Version: "t"})
		want := 3
		if runtime.GOOS == "linux" {
			want = 1
		}
		if code != want {
			t.Errorf("check (example config) exit = %d, want %d", code, want)
		}
	}
	// check with broken config → exit 2
	os.WriteFile(".shipsmoke.yml", []byte("version: 1\nartifacts:\n  - path: x\n    typo_field: 1\n"), 0o644)
	errOut.Reset()
	if code := Run([]string{"check"}, &out, &errOut, VersionInfo{Version: "t"}); code != 2 {
		t.Errorf("check (broken config) exit = %d, want 2, stderr=%s", code, errOut.String())
	}
	// check without config → exit 2
	os.Remove(".shipsmoke.yml")
	errOut.Reset()
	if code := Run([]string{"check"}, &out, &errOut, VersionInfo{Version: "t"}); code != 2 {
		t.Errorf("check (no config) exit = %d, want 2", code)
	}
}

func TestSanitizeVersionFallback(t *testing.T) {
	if sanitizeVersion("") != "0.0.0-dev" {
		t.Error("empty version should fall back to dev")
	}
	if sanitizeVersion("v1") != "v1" {
		t.Error("non-empty version must be kept")
	}
}
