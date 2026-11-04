package cigen

import (
	"strings"
	"testing"

	"github.com/geyutu6755/shipsmoke/internal/config"
	"gopkg.in/yaml.v3"
)

func TestParseStack(t *testing.T) {
	for _, s := range []string{"go", "rust", "c", "GO", " Rust "} {
		if _, err := ParseStack(s); err != nil {
			t.Errorf("ParseStack(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range []string{"", "python", "make", "auto"} {
		if _, err := ParseStack(s); err == nil {
			t.Errorf("ParseStack(%q) = nil, want error", s)
		}
	}
	if _, err := ParseStack(""); err == nil || !strings.Contains(err.Error(), "--stack is required") {
		t.Errorf("empty stack error = %v", err)
	}
}

func TestValidateName(t *testing.T) {
	for _, n := range []string{"mytool", "my-tool", "my_tool", "tool.exe", "v2"} {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v", n, err)
		}
	}
	for _, n := range []string{"", "my tool", "tool/x", "to:ol", strings.Repeat("x", 65)} {
		if err := ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", n)
		}
	}
}

func TestGenerate(t *testing.T) {
	for _, stack := range []Stack{StackGo, StackRust, StackC} {
		cfg, wf, err := Generate(stack, "mytool")
		if err != nil {
			t.Fatalf("Generate(%s): %v", stack, err)
		}
		// The config must parse under the strict shipsmoke schema.
		if _, err := config.Parse([]byte(cfg)); err != nil {
			t.Errorf("[%s] generated .shipsmoke.yml does not validate: %v", stack, err)
		}
		for _, want := range []string{
			"uses: geyutu6755/shipsmoke@v1",
			"version: v1.0.4",
			"ubuntu-24.04", "macos-15", "windows-2025",
			"actions/checkout@11d5960a", // SHA-pinned third-party actions
		} {
			if !strings.Contains(wf, want) {
				t.Errorf("[%s] workflow missing %q", stack, want)
			}
		}
		if !strings.Contains(wf, "${{ matrix.os }}") {
			t.Errorf("[%s] workflow missing matrix", stack)
		}
		// No heredoc anywhere: heredocs inside indented YAML block scalars
		// caused the v1.0.1 fake-green incident.
		if strings.Contains(wf, "<<") {
			t.Errorf("[%s] workflow contains a heredoc", stack)
		}
		// The generated workflow itself must be valid YAML.
		var parsed map[string]any
		if err := yaml.Unmarshal([]byte(wf), &parsed); err != nil {
			t.Errorf("[%s] generated workflow is not valid YAML: %v", stack, err)
		}
		switch stack {
		case StackGo:
			if !strings.Contains(wf, "go build") || !strings.Contains(wf, "setup-go") {
				t.Errorf("[%s] workflow lacks go build steps", stack)
			}
		case StackRust:
			if !strings.Contains(wf, "cargo build --release") {
				t.Errorf("[%s] workflow lacks cargo build", stack)
			}
		case StackC:
			if !strings.Contains(wf, "cc -O2") || !strings.Contains(wf, "gcc -O2") {
				t.Errorf("[%s] workflow lacks cc/gcc build", stack)
			}
		}
	}
}
