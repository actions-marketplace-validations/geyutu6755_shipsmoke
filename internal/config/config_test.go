package config

import (
	"strings"
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	yml := `
version: 1
report:
  json: out/report.json
artifacts:
  - name: tool-linux
    path: dist/tool-linux.tar.gz
    binary: tool
    runs_on: [linux/amd64]
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
          stdout_contains: ["tool version"]
        timeout: 15s
  - path: dist/tool.exe
    binary: tool.exe
    checks:
      - args: ["--help"]
`
	c, err := Parse([]byte(yml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Version != 1 {
		t.Errorf("version = %d", c.Version)
	}
	if c.Report.JSON != "out/report.json" {
		t.Errorf("report.json = %q", c.Report.JSON)
	}
	if len(c.Artifacts) != 2 {
		t.Fatalf("artifacts = %d", len(c.Artifacts))
	}
	a := c.Artifacts[0]
	if a.Format != "auto" || a.Env != "minimal" {
		t.Errorf("defaults not applied: format=%q env=%q", a.Format, a.Env)
	}
	if a.Checks[0].Timeout.Duration != 15*time.Second {
		t.Errorf("timeout = %v", a.Checks[0].Timeout.Duration)
	}
	a2 := c.Artifacts[1]
	if a2.Name != "tool" {
		t.Errorf("derived name = %q, want tool", a2.Name)
	}
	if a2.Checks[0].Timeout.Duration != DefaultTimeout {
		t.Errorf("default timeout = %v", a2.Checks[0].Timeout.Duration)
	}
}

func TestParseUnknownKeyIsError(t *testing.T) {
	yml := `
version: 1
artifacts:
  - path: dist/tool.tar.gz
    binary: tool
    run_on: [linux/amd64]
    checks:
      - args: ["--version"]
`
	_, err := Parse([]byte(yml))
	if err == nil || !strings.Contains(err.Error(), "run_on") {
		t.Fatalf("expected unknown-field error mentioning run_on, got: %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name    string
		yml     string
		wantSub string
	}{
		{"wrong version", "version: 2\nartifacts:\n  - path: a\n    binary: b\n    checks:\n      - args: [x]\n", "version"},
		{"no artifacts", "version: 1\nartifacts: []\n", "no artifacts"},
		{"missing path", "version: 1\nartifacts:\n  - binary: b\n    checks: [{args: [x]}]\n", "\"path\" is required"},
		{"missing binary", "version: 1\nartifacts:\n  - path: a\n    checks: [{args: [x]}]\n", "\"binary\" is required"},
		{"bad format", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    format: rar\n    checks: [{args: [x]}]\n", "unknown format"},
		{"bad env mode", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    env: full\n    checks: [{args: [x]}]\n", "unknown env mode"},
		{"bad runs_on", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    runs_on: [linux]\n    checks: [{args: [x]}]\n", "invalid runs_on"},
		{"no checks", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    checks: []\n", "no checks"},
		{"empty args", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    checks:\n      - args: []\n", "args"},
		{"negative exit", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    checks:\n      - args: [x]\n        expect: {exit_code: -3}\n", "exit_code"},
		{"bad regex", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    checks:\n      - args: [x]\n        expect: {stdout_regex: \"([\"}\n", "regular expression"},
		{"bad duration", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    checks:\n      - args: [x]\n        timeout: soon\n", "invalid duration"},
		{"huge duration", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    checks:\n      - args: [x]\n        timeout: 2h\n", "maximum allowed"},
		{"bad env key", "version: 1\nartifacts:\n  - path: a\n    binary: b\n    env_extra: {\"bad key\": v}\n    checks: [{args: [x]}]\n", "env_extra"},
		{"dup names", "version: 1\nartifacts:\n  - name: t\n    path: a\n    binary: b\n    checks: [{args: [x]}]\n  - name: t\n    path: c\n    binary: d\n    checks: [{args: [x]}]\n", "duplicate artifact name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yml))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestDurationMarshal(t *testing.T) {
	d := Duration{Duration: 90 * time.Second}
	out, err := d.MarshalYAML()
	if err != nil {
		t.Fatal(err)
	}
	if out != "1m30s" {
		t.Errorf("marshalled = %v", out)
	}
}

func TestBaseName(t *testing.T) {
	cases := map[string]string{
		"dist/mytool-linux-amd64.tar.gz": "mytool-linux-amd64",
		"tool.exe":                       "tool",
		"dist/tool":                      "tool",
	}
	for in, want := range cases {
		if got := baseName(in); got != want {
			t.Errorf("baseName(%q) = %q, want %q", in, got, want)
		}
	}
}
