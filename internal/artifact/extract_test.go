package artifact

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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

// --- archive builders (all archives are constructed in-test) ---

type zipEntry struct {
	name     string
	data     []byte
	mode     os.FileMode
	modified bool
}

func buildZip(t testing.TB, path string, entries []zipEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.modified {
			fh.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

type tarEntry struct {
	name     string
	data     []byte
	mode     os.FileMode
	dir      bool
	typeflag byte
	link     string
}

func buildTarGz(t testing.TB, path string, entries []tarEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: int64(e.mode)}
		switch {
		case e.typeflag != 0: // symlink/hardlink/fifo/device as requested
			hdr.Typeflag = e.typeflag
			hdr.Linkname = e.link
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
			continue
		case e.dir:
			hdr.Typeflag = tar.TypeDir
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
			continue
		}
		hdr.Typeflag = tar.TypeReg
		hdr.Size = int64(len(e.data))
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
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
	buildZip(t, archive, []zipEntry{
		{name: "tool/", data: nil},
		{name: "tool/readme.txt", data: []byte("hi")},
		{name: "tool/" + fixtureBinName(), data: bin},
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
	buildTarGz(t, archive, []tarEntry{
		{name: "tool", mode: 0o755, dir: true},
		{name: "tool/readme.txt", data: []byte("hi"), mode: 0o644},
		{name: "tool/" + fixtureBinName(), data: bin, mode: 0o755},
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
		"....//evil.txt",
		"sub/../../../evil.txt",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "out")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "evil.zip")
			buildZip(t, archive, []zipEntry{{name: name, data: []byte("pwned")}})
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
		{"a\\b.txt", "backslash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "out")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "evil.zip")
			buildZip(t, archive, []zipEntry{{name: tc.name, data: []byte("x")}})
			_, err := ExtractZip(archive, dest, defaultLimits())
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func TestExtractZipSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "evil.zip")
	buildZip(t, archive, []zipEntry{{
		name:     "link",
		data:     []byte("/etc/passwd"),
		mode:     0o777 | os.ModeSymlink,
		modified: true,
	}})
	_, err := ExtractZip(archive, dest, defaultLimits())
	if err == nil || !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("want unsafe-entry error, got %v", err)
	}
	if !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("error should mention symlink: %v", err)
	}
}

func TestExtractZipBombRatio(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "bomb.zip")
	buildZip(t, archive, []zipEntry{{name: "zeros.bin", data: make([]byte, 32<<20)}})
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
	buildZip(t, archive, []zipEntry{{name: "big.bin", data: make([]byte, 1<<20)}})
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
	buildZip(t, full, []zipEntry{{name: "a.txt", data: []byte("aaa")}})
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
	var entries []zipEntry
	for i := 0; i < 100; i++ {
		entries = append(entries, zipEntry{name: fmt.Sprintf("f%03d.txt", i), data: []byte("x")})
	}
	buildZip(t, archive, entries)
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
		entries []tarEntry
		wantSub string
	}{
		{"parent traversal", []tarEntry{{name: "../evil.txt", data: []byte("x")}}, ".."},
		{"nested traversal", []tarEntry{{name: "a/../../evil.txt", data: []byte("x")}}, ".."},
		{"absolute", []tarEntry{{name: "/etc/evil", data: []byte("x")}}, "absolute"},
		{"symlink", []tarEntry{{name: "link", typeflag: tar.TypeSymlink, link: "/etc/passwd"}}, "symbolic link"},
		{"hardlink", []tarEntry{{name: "hlink", typeflag: tar.TypeLink, link: "target"}}, "hard link"},
		{"fifo", []tarEntry{{name: "pipe", typeflag: tar.TypeFifo}}, "FIFO"},
		{"chardev", []tarEntry{{name: "dev", typeflag: tar.TypeChar}}, "device node"},
		{"backslash", []tarEntry{{name: "a\\b.txt", data: []byte("x")}}, "backslash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "out")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "evil.tar.gz")
			buildTarGz(t, archive, tc.entries)
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
	buildTarGz(t, full, []tarEntry{
		{name: "a.txt", data: []byte("hello world, this is a long enough payload")},
		{name: "b.txt", data: []byte("second file content, also long enough for tar blocks")},
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
	buildTarGz(t, archive, []tarEntry{{name: "zeros.bin", data: make([]byte, 64<<20)}})
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
