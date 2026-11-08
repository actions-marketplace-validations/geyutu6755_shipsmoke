package cigen

import (
	"os"
	"strings"
	"testing"
)

// TestActionDescriptionLength ensures the action.yml description stays
// within the 125-char limit required by the project's Marketplace listing
// quality standard (docs/MARKETPLACE_CHECKLIST.md).
func TestActionDescriptionLength(t *testing.T) {
	data, err := os.ReadFile("../../action.yml")
	if err != nil {
		t.Skipf("action.yml not reachable from test: %v", err)
	}
	s := string(data)
	idx := strings.Index(s, "description:")
	if idx < 0 {
		t.Fatal("description not found in action.yml")
	}
	rest := s[idx:]
	end := strings.Index(rest, "\nauthor:")
	if end < 0 {
		t.Fatal("author field not found after description")
	}
	block := rest[:end]
	// Extract the folded scalar lines and join them
	var parts []string
	for _, line := range strings.Split(block, "\n")[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == ">" || trimmed == ">-" {
			continue
		}
		parts = append(parts, trimmed)
	}
	desc := strings.Join(parts, " ")
	if len(desc) > 125 {
		t.Errorf("action.yml description is %d chars, must be <= 125: %q", len(desc), desc)
	}
	if len(desc) < 20 {
		t.Errorf("action.yml description is only %d chars, too short to be useful", len(desc))
	}
}
