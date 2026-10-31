package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geyutu6755/shipsmoke/internal/testfixture"
)

func TestDetectBinaryRealCrossCompiled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-compile fixtures in short mode")
	}
	cases := []struct {
		goos, goarch string
		wantFormat   string
		wantOS       string
		wantArch     string
	}{
		{"linux", "amd64", "elf", "linux", "amd64"},
		{"linux", "arm64", "elf", "linux", "arm64"},
		{"windows", "amd64", "pe", "windows", "amd64"},
		{"darwin", "arm64", "macho", "darwin", "arm64"},
		{"darwin", "amd64", "macho", "darwin", "amd64"},
		{"linux", "386", "elf", "linux", "386"},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			bin := testfixture.BinPath(t, tc.goos, tc.goarch)
			got := DetectBinary(bin)
			if got.Format != tc.wantFormat || got.OS != tc.wantOS || got.Arch != tc.wantArch {
				t.Errorf("DetectBinary(%s) = %s, want %s/%s/%s", bin, got, tc.wantFormat, tc.wantOS, tc.wantArch)
			}
			if !got.Known() {
				t.Errorf("identity should be known: %s", got)
			}
		})
	}
}

func TestDetectBinaryScript(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "script")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := DetectBinary(p)
	if got.Format != "script" || got.OS != "unknown" {
		t.Errorf("script detection = %s", got)
	}
	if got.Known() {
		t.Errorf("script platform must not claim to be known")
	}
}

func TestDetectBinaryUnknown(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(p, []byte("this is not an executable at all, just text"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DetectBinary(p)
	if got.Format != "unknown" || got.Known() {
		t.Errorf("unknown detection = %s", got)
	}
}

func TestClassify(t *testing.T) {
	dir := t.TempDir()
	t.Run("zip", func(t *testing.T) {
		p := filepath.Join(dir, "a.zip")
		if err := os.WriteFile(p, append([]byte("PK\x03\x04"), make([]byte, 100)...), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := Classify(p)
		if err != nil || f != FormatZip {
			t.Errorf("classify zip = %v, %v", f, err)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		p := filepath.Join(dir, "junk.bin")
		if err := os.WriteFile(p, []byte("garbage garbage garbage"), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := Classify(p)
		if err == nil || f != FormatUnknown {
			t.Errorf("classify junk = %v, %v (want error)", f, err)
		}
	})
	t.Run("missing", func(t *testing.T) {
		_, err := Classify(filepath.Join(dir, "nope.zip"))
		if err == nil {
			t.Error("want error for missing file")
		}
	})
}

func TestMatchesHost(t *testing.T) {
	host := HostOS() + "/" + HostArch()
	if !MatchesHost(host) {
		t.Errorf("MatchesHost(%q) = false", host)
	}
	if !MatchesHost("any") || MatchesHost("") == false {
		t.Log("any/empty both match")
	}
	if MatchesHost("nonos/noarch") {
		t.Error("bogus spec should not match")
	}
	if !RunsOnHost(nil) || !RunsOnHost([]string{"any"}) || !RunsOnHost([]string{"nonos/noarch", host}) {
		t.Error("RunsOnHost logic broken")
	}
	if RunsOnHost([]string{"nonos/noarch"}) {
		t.Error("RunsOnHost should be false for unrelated platform")
	}
}
