package artifact

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/geyutu6755/shipsmoke/internal/testfixture"
)

// corruptByte flips one byte at the given offset in a file on disk.
func corruptByte(t *testing.T, path string, off int64) {
	t.Helper()
	if err := corruptByteErr(path, off); err != nil {
		t.Fatal(err)
	}
}

func corruptByteErr(path string, off int64) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if off < 0 || off >= int64(len(data)) {
		return fmt.Errorf("offset %d beyond file size %d", off, len(data))
	}
	data[off] ^= 0xff
	return os.WriteFile(path, data, 0o644)
}

// buildSingleFileTarGz creates a tar.gz containing one executable payload —
// the canonical release shape used by every corruption test below.
func buildSingleFileTarGz(t *testing.T, dir string) string {
	t.Helper()
	bin, err := os.ReadFile(testfixture.HostBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "tool.tar.gz")
	testfixture.BuildTarGz(t, path, []testfixture.TarEntry{
		{Name: "tool", Mode: 0o755, Dir: true},
		{Name: "tool/" + fixtureBinName(), Data: bin, Mode: 0o755},
	})
	return path
}

func extractForCorruption(t *testing.T, archive string) (*Result, error) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	return ExtractTarGz(archive, dest, defaultLimits())
}

//  1. CRC error: flipping a byte in the gzip trailer (last 8 bytes: CRC32 +
//     ISIZE) must fail the extraction. Before v1.0.3 the tar reader stopped at
//     the zero blocks and never reached the trailer, so this passed.
func TestExtractTarGzCRCCorruptionFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fixture build in short mode")
	}
	dir := t.TempDir()
	archive := buildSingleFileTarGz(t, dir)
	fi, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	corruptByte(t, archive, fi.Size()-3) // inside the CRC32 field
	res, err := extractForCorruption(t, archive)
	if err == nil {
		t.Fatalf("CRC corruption must fail extraction (extracted %d files)", res.Files)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("integrity check")) &&
		!bytes.Contains([]byte(err.Error()), []byte("checksum")) {
		t.Logf("note: error message = %v", err)
	}
}

//  2. Truncated trailer: cutting the last bytes (part of the gzip CRC/ISIZE
//     trailer) must fail — a real "broken upload" shape.
func TestExtractTarGzTruncatedTrailerFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fixture build in short mode")
	}
	dir := t.TempDir()
	archive := buildSingleFileTarGz(t, dir)
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, data[:len(data)-4], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := extractForCorruption(t, archive); err == nil {
		t.Fatal("truncated gzip trailer must fail extraction")
	}
}

//  3. Mid-stream deflate corruption: a flipped byte inside the compressed
//     data either breaks inflate or yields different bytes whose CRC no
//     longer matches — both must fail extraction now that the member is
//     drained and verified.
func TestExtractTarGzMidStreamCorruptionFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fixture build in short mode")
	}
	dir := t.TempDir()
	archive := buildSingleFileTarGz(t, dir)
	fi, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	corruptByte(t, archive, fi.Size()/2)
	if _, err := extractForCorruption(t, archive); err == nil {
		t.Fatal("mid-stream deflate corruption must fail extraction (CRC verified)")
	}
}

//  4. Trailing garbage AFTER a complete gzip member stays accepted: the tar
//     payload is structurally complete and the member CRC verifies. This is
//     the documented leniency; download checksums cover this gap.
func TestExtractTarGzTrailingGarbageAccepted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fixture build in short mode")
	}
	dir := t.TempDir()
	archive := buildSingleFileTarGz(t, dir)
	f, err := os.OpenFile(archive, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("some-junk-appended-after-the-archive"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	res, err := extractForCorruption(t, archive)
	if err != nil {
		t.Fatalf("complete payload + trailing junk must still extract: %v", err)
	}
	if res.Files != 1 {
		t.Errorf("files = %d, want 1", res.Files)
	}
}

//  5. Content corruption of a NON-CRITICAL file inside the member: with the
//     CRC now verified, any flipped byte inside the compressed data changes
//     the decompressed bytes and must fail — the tar structure alone cannot
//     vouch for content anymore. (Before v1.0.3 this passed silently.)
func TestExtractTarGzNonCriticalFileCorruptionFails(t *testing.T) {
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
		{Name: "tool/" + fixtureBinName(), Data: bin, Mode: 0o755},
		{Name: "tool/README.md", Data: []byte("a non-critical companion file with enough content to fill a block or two — padded padded padded padded padded padded"), Mode: 0o644},
	})
	fi, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	corruptByte(t, archive, fi.Size()/2)
	if _, err := extractForCorruption(t, archive); err == nil {
		t.Fatal("content corruption inside the gzip member must fail (CRC verified)")
	}
}

// Guard: the corruption helpers only flip in-bounds bytes.
func TestCorruptByteBounds(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	os.WriteFile(p, []byte("abc"), 0o644)
	corruptByte(t, p, 1)
	data, _ := os.ReadFile(p)
	if bytes.Equal(data, []byte("abc")) {
		t.Fatal("byte was not flipped")
	}
	if err := corruptByteErr(p, 999); err == nil {
		t.Fatal("out-of-bounds offset should have been rejected")
	}
}
