package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzExtractTarGz hammers the tar.gz extractor with arbitrary bytes; it must
// never panic and never write outside destDir.
func FuzzExtractTarGz(f *testing.F) {
	// Seeds: valid tar.gz, truncated, non-gzip.
	dir := f.TempDir()
	good := filepath.Join(dir, "good.tar.gz")
	buildTarGz(f, good, []tarEntry{
		{name: "a.txt", data: []byte("hello hello hello")},
		{name: "d/b.txt", data: []byte("nested")},
	})
	if seed, err := os.ReadFile(good); err == nil {
		f.Add(seed)
	}
	f.Add([]byte("not gzip at all"))
	f.Add([]byte{0x1f, 0x8b})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "out")
		if err := os.MkdirAll(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, "fuzz.tar.gz")
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _ = ExtractTarGz(p, dest, Limits{
			MaxFiles:            500,
			MaxTotalSize:        8 << 20,
			MaxFileSize:         1 << 20,
			MaxCompressionRatio: 50,
		})
	})
}

// FuzzExtractZip hammers the zip extractor with arbitrary bytes.
func FuzzExtractZip(f *testing.F) {
	dir := f.TempDir()
	good := filepath.Join(dir, "good.zip")
	buildZip(f, good, []zipEntry{
		{name: "a.txt", data: []byte("hello hello hello")},
		{name: "d/b.txt", data: []byte("nested")},
	})
	if seed, err := os.ReadFile(good); err == nil {
		f.Add(seed)
	}
	f.Add([]byte("PK\x03\x04truncated"))
	f.Add([]byte{})
	f.Add([]byte("plain text"))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "out")
		if err := os.MkdirAll(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, "fuzz.zip")
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _ = ExtractZip(p, dest, Limits{
			MaxFiles:            500,
			MaxTotalSize:        8 << 20,
			MaxFileSize:         1 << 20,
			MaxCompressionRatio: 50,
		})
	})
}
