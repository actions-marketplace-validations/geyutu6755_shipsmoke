package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geyutu6755/shipsmoke/internal/testfixture"
)

// Boundary tests for hostile/edge-case file names and destinations, from the
// final-audit checklist: Unicode, spaces, very long paths, deep nesting.

func TestExtractZipUnicodeAndSpaceNames(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "uni.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{
		{Name: "文件 目录/工具版本说明.txt", Data: []byte("unicode content")},
		{Name: "with spaces/plain name.txt", Data: []byte("spaces are legal")},
		{Name: "日本語/русский/العربية/nested.txt", Data: []byte("multi-script")},
	})
	res, err := ExtractZip(archive, dest, defaultLimits())
	if err != nil {
		t.Fatalf("unicode/space entries must extract: %v", err)
	}
	if res.Files != 3 {
		t.Errorf("files = %d, want 3", res.Files)
	}
	for _, p := range []string{
		filepath.Join(dest, "文件 目录", "工具版本说明.txt"),
		filepath.Join(dest, "with spaces", "plain name.txt"),
		filepath.Join(dest, "日本語", "русский", "العربية", "nested.txt"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("extracted file missing: %s (%v)", p, err)
		}
	}
}

func TestExtractZipDeepNesting(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// 40 levels deep, still far below any per-component OS limit.
	deep := strings.Repeat("d/", 40) + "leaf.txt"
	archive := filepath.Join(dir, "deep.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: deep, Data: []byte("deep leaf")}})
	if _, err := ExtractZip(archive, dest, defaultLimits()); err != nil {
		t.Fatalf("deep nesting must extract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, strings.ReplaceAll(deep, "/", string(os.PathSeparator)))); err != nil {
		t.Errorf("deep leaf missing: %v", err)
	}
}

func TestExtractZipLongNameComponent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// 300-char single component: exceeds every filesystem's 255-byte
	// per-component cap, so the OS create must fail with a wrapped error —
	// not a panic and not a silent partial write.
	longName := strings.Repeat("长", 300) + ".txt"
	archive := filepath.Join(dir, "long.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: longName, Data: []byte("x")}})
	_, err := ExtractZip(archive, dest, defaultLimits())
	if err == nil {
		t.Fatal("300-char component should fail at OS level")
	}
	// Windows rejects the create itself ("cannot create file …"); Unix fails
	// during the copy ("error while extracting …"). Both are the wrapped,
	// non-panicking paths the extractor guarantees.
	if !strings.Contains(err.Error(), "cannot create file") &&
		!strings.Contains(err.Error(), "error while extracting") {
		t.Errorf("error should be a wrapped extraction failure, got: %v", err)
	}
}
