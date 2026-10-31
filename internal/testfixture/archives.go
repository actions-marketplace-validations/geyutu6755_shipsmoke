package testfixture

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"testing"
)

// ZipEntry describes one entry for BuildZip.
type ZipEntry struct {
	Name    string
	Data    []byte
	Mode    os.FileMode
	SetMode bool
}

// TarEntry describes one entry for BuildTarGz.
type TarEntry struct {
	Name     string
	Data     []byte
	Mode     os.FileMode
	Dir      bool
	Typeflag byte
	Link     string
}

// BuildZip writes a real zip archive at path with the given entries.
func BuildZip(t testing.TB, path string, entries []ZipEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.Name, Method: zip.Deflate}
		if e.SetMode {
			fh.SetMode(e.Mode)
		}
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.Data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// BuildTarGz writes a real gzipped tar at path with the given entries.
func BuildTarGz(t testing.TB, path string, entries []TarEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.Name, Mode: int64(e.Mode)}
		switch {
		case e.Typeflag != 0:
			hdr.Typeflag = e.Typeflag
			hdr.Linkname = e.Link
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
			continue
		case e.Dir:
			hdr.Typeflag = tar.TypeDir
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
			continue
		}
		hdr.Typeflag = tar.TypeReg
		hdr.Size = int64(len(e.Data))
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.Data); err != nil {
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
