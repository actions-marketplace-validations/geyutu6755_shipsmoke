package artifact

import (
	"archive/tar"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geyutu6755/shipsmoke/internal/testfixture"
)

func defaultLimits() Limits {
	return Limits{
		MaxFiles:            1000,
		MaxTotalSize:        64 << 20,
		MaxFileSize:         16 << 20,
		MaxCompressionRatio: 100,
	}
}

// fixtureBinName is the name used for the real binary inside test archives.
func fixtureBinName() string {
	n := "tool"
	if HostOS() == "windows" {
		n += ".exe"
	}
	return n
}

func TestExtractZipValid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fixture build in short mode")
	}
	dir := t.TempDir()
	bin, err := os.ReadFile(testfixture.HostBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "tool.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{
		{Name: "tool/", Data: nil},
		{Name: "tool/readme.txt", Data: []byte("hi")},
		{Name: "tool/" + fixtureBinName(), Data: bin},
	})
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := ExtractZip(archive, dest, defaultLimits())
	if err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	if res.Files != 2 {
		t.Errorf("files = %d, want 2", res.Files)
	}
	got, err := os.ReadFile(filepath.Join(dest, "tool", fixtureBinName()))
	if err != nil {
		t.Fatalf("extracted binary missing: %v", err)
	}
	if string(got) != string(bin) {
		t.Error("extracted binary content differs")
	}
}

func TestExtractTarGzValid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fixture build in short mode")
	}
	dir := t.TempDir()
	bin, err := os.ReadFile(testfixture.HostBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "tool.tar.gz")
	testfixture.BuildTarGz(t, archive, []testfixture.TarEntry{
		{Name: "tool", Mode: 0o755, Dir: true},
		{Name: "tool/readme.txt", Data: []byte("hi"), Mode: 0o644},
		{Name: "tool/" + fixtureBinName(), Data: bin, Mode: 0o755},
	})
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := ExtractTarGz(archive, dest, defaultLimits())
	if err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}
	if res.Files != 2 {
		t.Errorf("files = %d, want 2", res.Files)
	}
}

func TestExtractZipTraversal(t *testing.T) {
	for _, name := range []string{
		"../evil.txt",
		"a/../../evil.txt",
		// "....//evil.txt" is contained on Unix (a literal directory named
		// "....") and rejected on Windows (trailing dots) — not traversal.
		"....//../evil.txt",
		"sub/../../../evil.txt",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "out")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "evil.zip")
			testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: name, Data: []byte("pwned")}})
			_, err := ExtractZip(archive, dest, defaultLimits())
			if err == nil {
				t.Fatal("extraction should have failed")
			}
			if !errors.Is(err, ErrUnsafeEntry) {
				t.Fatalf("wrong error class: %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "evil.txt")); statErr == nil {
				t.Error("file escaped the extraction directory")
			}
			entries, _ := os.ReadDir(dest)
			if len(entries) != 0 {
				t.Errorf("dest not empty after refusal: %d entries", len(entries))
			}
		})
	}
}

func TestExtractZipAbsoluteAndDriveAndReserved(t *testing.T) {
	cases := []struct{ name, wantSub string }{
		{"/etc/evil.txt", "absolute"},
		{"C:/evil.txt", "drive letter"},
		{"CON", "Windows-reserved"},
		{"sub/CON.txt", "Windows-reserved"},
		{"..\\evil.exe", "path traversal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "out")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "evil.zip")
			testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: tc.name, Data: []byte("x")}})
			_, err := ExtractZip(archive, dest, defaultLimits())
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func TestExtractZipBackslashSeparatorNormalized(t *testing.T) {
	// PowerShell Compress-Archive writes entries with backslash separators
	// ("dir\file.txt"), against the zip spec. Windows Explorer reads them
	// as directories, so ShipSmoke normalizes and accepts them.
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "bs.zip")
	name := "a\\b.txt"
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: name, Data: []byte("content")}})
	_, err := ExtractZip(archive, dest, defaultLimits())
	if err != nil {
		t.Fatalf("backslash entry should extract as a/b.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a", "b.txt")); err != nil {
		t.Errorf("expected a/b.txt after normalization: %v", err)
	}
	// Backslash traversal must still be rejected.
	dest2 := filepath.Join(dir, "out2")
	if err := os.MkdirAll(dest2, 0o755); err != nil {
		t.Fatal(err)
	}
	archive2 := filepath.Join(dir, "bs-evil.zip")
	testfixture.BuildZip(t, archive2, []testfixture.ZipEntry{{Name: "..\\evil.txt", Data: []byte("x")}})
	_, err = ExtractZip(archive2, dest2, defaultLimits())
	if err == nil || !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("backslash traversal must be rejected, got %v", err)
	}
}

func TestExtractZipSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "evil.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{
		Name:    "link",
		Data:    []byte("/etc/passwd"),
		Mode:    0o777 | os.ModeSymlink,
		SetMode: true,
	}})
	_, err := ExtractZip(archive, dest, defaultLimits())
	if err == nil || !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("want unsafe-entry error, got %v", err)
	}
	if !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("error should mention symLink: %v", err)
	}
}

func TestExtractZipBombRatio(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "bomb.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: "zeros.bin", Data: make([]byte, 32<<20)}})
	_, err := ExtractZip(archive, dest, defaultLimits())
	if err == nil || !errors.Is(err, ErrLimitHit) {
		t.Fatalf("want limit error, got %v", err)
	}
}

func TestExtractZipPerFileLimit(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "big.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: "big.bin", Data: make([]byte, 1<<20)}})
	limits := defaultLimits()
	limits.MaxFileSize = 4096
	_, err := ExtractZip(archive, dest, limits)
	if err == nil || !errors.Is(err, ErrLimitHit) {
		t.Fatalf("want limit error, got %v", err)
	}
}

func TestExtractZipCorrupt(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "bad.zip")
	full := filepath.Join(dir, "full.zip")
	testfixture.BuildZip(t, full, []testfixture.ZipEntry{{Name: "a.txt", Data: []byte("aaa")}})
	data, _ := os.ReadFile(full)
	if err := os.WriteFile(archive, data[:len(data)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ExtractZip(archive, dest, defaultLimits())
	if err == nil {
		t.Fatal("want error for corrupt zip")
	}
}

func TestExtractZipTooManyEntries(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "many.zip")
	var entries []testfixture.ZipEntry
	for i := 0; i < 100; i++ {
		entries = append(entries, testfixture.ZipEntry{Name: fmt.Sprintf("f%03d.txt", i), Data: []byte("x")})
	}
	testfixture.BuildZip(t, archive, entries)
	limits := defaultLimits()
	limits.MaxFiles = 50
	_, err := ExtractZip(archive, dest, limits)
	if err == nil || !errors.Is(err, ErrLimitHit) {
		t.Fatalf("want limit error, got %v", err)
	}
}

func TestExtractTarGzTraversalAndLinks(t *testing.T) {
	cases := []struct {
		name    string
		entries []testfixture.TarEntry
		wantSub string
	}{
		{"parent traversal", []testfixture.TarEntry{{Name: "../evil.txt", Data: []byte("x")}}, ".."},
		{"nested traversal", []testfixture.TarEntry{{Name: "a/../../evil.txt", Data: []byte("x")}}, ".."},
		{"absolute", []testfixture.TarEntry{{Name: "/etc/evil", Data: []byte("x")}}, "absolute"},
		{"symlink", []testfixture.TarEntry{{Name: "link", Typeflag: tar.TypeSymlink, Link: "/etc/passwd"}}, "symbolic link"},
		{"hardlink", []testfixture.TarEntry{{Name: "hlink", Typeflag: tar.TypeLink, Link: "target"}}, "hard link"},
		{"fifo", []testfixture.TarEntry{{Name: "pipe", Typeflag: tar.TypeFifo}}, "FIFO"},
		{"chardev", []testfixture.TarEntry{{Name: "dev", Typeflag: tar.TypeChar}}, "device node"},
		{"backslash", []testfixture.TarEntry{{Name: "a\\b.txt", Data: []byte("x")}}, "backslash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "out")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "evil.tar.gz")
			testfixture.BuildTarGz(t, archive, tc.entries)
			_, err := ExtractTarGz(archive, dest, defaultLimits())
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantSub)
			}
			if tc.name == "parent traversal" || tc.name == "nested traversal" {
				if _, statErr := os.Stat(filepath.Join(dir, "evil.txt")); statErr == nil {
					t.Error("file escaped the extraction directory")
				}
			}
		})
	}
}

func TestExtractTarGzTruncated(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(dir, "full.tar.gz")
	testfixture.BuildTarGz(t, full, []testfixture.TarEntry{
		{Name: "a.txt", Data: []byte("hello world, this is a long enough payload")},
		{Name: "b.txt", Data: []byte("second file content, also long enough for tar blocks")},
	})
	data, _ := os.ReadFile(full)
	archive := filepath.Join(dir, "trunc.tar.gz")
	if err := os.WriteFile(archive, data[:len(data)/3], 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ExtractTarGz(archive, dest, defaultLimits())
	if err == nil {
		t.Fatal("want error for truncated tar.gz")
	}
}

func TestExtractTarGzBomb(t *testing.T) {
	if testing.Short() {
		t.Skip("slow in short mode")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "bomb.tar.gz")
	testfixture.BuildTarGz(t, archive, []testfixture.TarEntry{{Name: "zeros.bin", Data: make([]byte, 64<<20)}})
	_, err := ExtractTarGz(archive, dest, defaultLimits())
	if err == nil || !errors.Is(err, ErrLimitHit) {
		t.Fatalf("want limit error, got %v", err)
	}
}

func TestExtractTarGzNotGzip(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "notgz.tar.gz")
	if err := os.WriteFile(archive, []byte("definitely not gzip data"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ExtractTarGz(archive, dest, defaultLimits())
	if err == nil || !strings.Contains(err.Error(), "gzip") {
		t.Fatalf("want gzip error, got %v", err)
	}
}

func TestFindEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "tool"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := FindEntry(dir, "bin/tool", "", FormatZip)
	if err != nil {
		t.Fatalf("FindEntry: %v", err)
	}
	if filepath.Base(p) != "tool" {
		t.Errorf("got %q", p)
	}
	if _, err := FindEntry(dir, "../tool", "", FormatZip); err == nil {
		t.Error(".. in binary path must be rejected")
	}
	if _, err := FindEntry(dir, "/abs/tool", "", FormatZip); err == nil {
		t.Error("absolute binary path must be rejected")
	}
	_, err = FindEntry(dir, "missing/tool", "", FormatZip)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing entry error = %v", err)
	}
}

func TestValidateEntryNameEdge(t *testing.T) {
	ok := []string{"a/b/c.txt", "tool.exe", "dir/", "x.tar.gz"}
	for _, n := range ok {
		if err := validateEntryName(n); err != nil {
			t.Errorf("validateEntryName(%q) = %v, want nil", n, err)
		}
	}
	bad := []string{"", "/abs", "C:\\x", "a\\b", "../x", "a/../b", "NUL", "nul.txt", "a\x00b"}
	for _, n := range bad {
		if err := validateEntryName(n); err == nil {
			t.Errorf("validateEntryName(%q) = nil, want error", n)
		}
	}
}
