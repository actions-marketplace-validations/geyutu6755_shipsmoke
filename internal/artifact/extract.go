package artifact

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Limits bound what an archive may do to disk. They are enforced while
// streaming (before and during each write), never after the fact.
type Limits struct {
	MaxFiles            int
	MaxTotalSize        int64
	MaxFileSize         int64
	MaxCompressionRatio int64
}

// Result reports what was extracted.
type Result struct {
	Files      int
	Dirs       int
	TotalBytes int64
}

// Errors that mark the artifact (not the host) as the problem.
var (
	ErrUnsafeEntry = errors.New("unsafe archive entry")
	ErrLimitHit    = errors.New("archive exceeds safety limits")
)

// errUnsafe wraps a reason so callers can classify extraction failures.
type unsafeError struct{ reason string }

func (e *unsafeError) Error() string { return e.reason }
func (e *unsafeError) Unwrap() error { return ErrUnsafeEntry }

type limitError struct{ reason string }

func (e *limitError) Error() string { return e.reason }
func (e *limitError) Unwrap() error { return ErrLimitHit }

func unsafef(format string, a ...any) error {
	return &unsafeError{reason: fmt.Sprintf(format, a...)}
}

func limitf(format string, a ...any) error {
	return &limitError{reason: fmt.Sprintf(format, a...)}
}

// ExtractZip extracts the zip archive at archivePath into destDir, enforcing
// limits and rejecting unsafe entries before anything is written outside
// destDir. destDir must already exist.
func ExtractZip(archivePath, destDir string, limits Limits) (*Result, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open zip: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() == 0 {
		return nil, unsafef("zip archive is empty (0 bytes)")
	}
	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return nil, fmt.Errorf("not a readable zip archive: %w", err)
	}
	if len(zr.File) == 0 {
		return nil, unsafef("zip archive contains no entries")
	}
	if len(zr.File) > limits.MaxFiles {
		return nil, limitf("archive has %d entries, limit is %d", len(zr.File), limits.MaxFiles)
	}

	res := &Result{}
	counted := int64(0)
	for _, zf := range zr.File {
		// Some Windows packers (notably PowerShell Compress-Archive) write
		// entries with backslash separators, against the zip spec. Windows
		// Explorer reads them as separators, so we normalize before
		// validating — traversal like "..\evil" is still rejected below.
		name := strings.ReplaceAll(zf.Name, "\\", "/")
		if err := validateEntryName(name); err != nil {
			return nil, err
		}
		mode := zf.Mode()
		if mode&os.ModeSymlink != 0 {
			return nil, unsafef("zip entry %q is a symbolic link: not supported, refusing to create links", name)
		}
		if mode&os.ModeType != 0 && !mode.IsDir() {
			return nil, unsafef("zip entry %q has unsupported type %s: only regular files and directories are allowed", name, mode.Type())
		}

		size := int64(zf.UncompressedSize64)
		// A header may lie; the actual copy is also bounded below.
		if size > limits.MaxFileSize {
			return nil, limitf("zip entry %q is %d bytes, per-file limit is %d", name, size, limits.MaxFileSize)
		}
		if counted+size > limits.MaxTotalSize {
			return nil, limitf("extracting entry %q would exceed the total size limit %d (already at %d)", name, limits.MaxTotalSize, counted)
		}
		compSize := int64(zf.CompressedSize64)
		if compSize > 0 && size/compSize > limits.MaxCompressionRatio && size > 1<<20 {
			return nil, limitf("zip entry %q expands %d× (from %d to %d bytes): refusing probable decompression bomb", name, size/compSize, compSize, size)
		}

		dest := filepath.Join(destDir, filepath.FromSlash(name))
		if err := ensureInside(destDir, dest); err != nil {
			return nil, err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return nil, fmt.Errorf("cannot create directory %q: %w", name, err)
			}
			res.Dirs++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fmt.Errorf("cannot create parent directory for %q: %w", name, err)
		}
		rc, err := zf.Open()
		if err != nil {
			return nil, fmt.Errorf("cannot open zip entry %q: %w", name, err)
		}
		n, err := copyCapped(rc, dest, limits.MaxFileSize)
		rc.Close()
		if err != nil {
			return nil, err
		}
		counted += n
		res.Files++
		res.TotalBytes += n
		if err := normalizeFileMode(dest, mode); err != nil {
			return nil, err
		}
	}
	if res.Files == 0 && res.Dirs == 0 {
		return nil, unsafef("zip archive produced no files")
	}
	return res, nil
}

// ExtractTarGz extracts the gzipped tar at archivePath into destDir with the
// same safety rules as ExtractZip. Symbolic links, hard links, devices and
// FIFOs are rejected.
func ExtractTarGz(archivePath, destDir string, limits Limits) (*Result, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open tar.gz: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() == 0 {
		return nil, unsafef("tar.gz archive is empty (0 bytes)")
	}
	gz, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		return nil, fmt.Errorf("not a readable gzip stream: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	res := &Result{}
	counted := int64(0)
	entries := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar archive is truncated or corrupt after %d entries: %w", entries, err)
		}
		entries++
		if entries > limits.MaxFiles {
			return nil, limitf("archive has more than %d entries", limits.MaxFiles)
		}
		name := hdr.Name
		if err := validateEntryName(name); err != nil {
			return nil, err
		}
		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			// regular file
		case tar.TypeDir:
			dest := filepath.Join(destDir, filepath.FromSlash(name))
			if err := ensureInside(destDir, dest); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return nil, fmt.Errorf("cannot create directory %q: %w", name, err)
			}
			res.Dirs++
			continue
		case tar.TypeSymlink:
			return nil, unsafef("tar entry %q is a symbolic link to %q: not supported, refusing to create links", name, hdr.Linkname)
		case tar.TypeLink:
			return nil, unsafef("tar entry %q is a hard link to %q: not supported, refusing to create links", name, hdr.Linkname)
		case tar.TypeChar, tar.TypeBlock:
			return nil, unsafef("tar entry %q is a device node: refusing", name)
		case tar.TypeFifo:
			return nil, unsafef("tar entry %q is a FIFO: refusing", name)
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue // PAX metadata, not a filesystem entry
		default:
			return nil, unsafef("tar entry %q has unsupported type %q: refusing", name, string(hdr.Typeflag))
		}

		if hdr.Size > limits.MaxFileSize {
			return nil, limitf("tar entry %q is %d bytes, per-file limit is %d", name, hdr.Size, limits.MaxFileSize)
		}
		if counted+hdr.Size > limits.MaxTotalSize {
			return nil, limitf("extracting entry %q would exceed the total size limit %d (already at %d)", name, limits.MaxTotalSize, counted)
		}

		dest := filepath.Join(destDir, filepath.FromSlash(name))
		if err := ensureInside(destDir, dest); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fmt.Errorf("cannot create parent directory for %q: %w", name, err)
		}
		n, err := copyCapped(tr, dest, limits.MaxFileSize)
		if err != nil {
			return nil, err
		}
		counted += n
		res.Files++
		res.TotalBytes += n
		if err := normalizeFileMode(dest, os.FileMode(hdr.Mode)&0o777); err != nil {
			return nil, err
		}
	}
	if entries == 0 {
		return nil, unsafef("tar archive contains no entries")
	}
	// Compression-ratio check for the whole stream.
	if fi.Size() > 0 && counted/fi.Size() > limits.MaxCompressionRatio && counted > 8<<20 {
		return nil, limitf("archive expands %d× (from %d to %d bytes): refusing probable decompression bomb",
			counted/fi.Size(), fi.Size(), counted)
	}
	return res, nil
}

// copyCapped copies the reader into a new file at dest, enforcing maxPerFile
// during the copy (archive headers can lie about sizes).
func copyCapped(r io.Reader, dest string, maxPerFile int64) (int64, error) {
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, fmt.Errorf("cannot create file %s: %w", filepath.Base(dest), err)
	}
	n, err := io.Copy(out, io.LimitReader(r, maxPerFile+1))
	closeErr := out.Close()
	if err != nil {
		os.Remove(dest)
		return n, fmt.Errorf("error while extracting %s: %w", filepath.Base(dest), err)
	}
	if closeErr != nil {
		return n, closeErr
	}
	if n > maxPerFile {
		os.Remove(dest)
		return n, limitf("file %s exceeds the per-file limit of %d bytes during extraction", filepath.Base(dest), maxPerFile)
	}
	return n, nil
}

// normalizeFileMode makes extracted files usable: on Unix apply the archive
// mode restricted to a sane band (no setuid/setgid/sticky); on Windows clear
// the read-only attribute so the entry binary can be overwritten/removed.
func normalizeFileMode(path string, mode os.FileMode) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	sane := mode & 0o755
	if sane&0o400 == 0 {
		sane |= 0o400 // must remain readable by owner
	}
	if sane&0o100 != 0 { // preserve owner-executable only if archive said so
		sane |= 0o100
	} else {
		sane &^= 0o111
	}
	return os.Chmod(path, sane)
}

// ensureInside is the final Zip Slip guard: dest must be strictly within
// root after cleaning. Called after validateEntryName so both layers must
// agree.
func ensureInside(root, dest string) error {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	cleanDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cleanRoot, cleanDest)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return unsafef("entry %q escapes the extraction directory: refusing (path traversal)", filepath.Base(dest))
	}
	return nil
}

// windowsReserved names cannot be created on Windows; creating them on other
// platforms would produce archives that break on Windows, so they are always
// rejected.
var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// validateEntryName rejects entry names that are absolute, traverse outside
// the root, use backslashes, target Windows reserved devices, or contain
// NUL. It is the first line of defense; ensureInside is the second.
func validateEntryName(name string) error {
	if name == "" {
		return unsafef("archive contains an entry with an empty name")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return unsafef("archive entry name %q contains a control character: refusing", name)
		}
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return unsafef("archive entry %q has an absolute path: refusing", name)
	}
	if len(name) >= 2 && name[1] == ':' {
		return unsafef("archive entry %q contains a drive letter: refusing", name)
	}
	if strings.Contains(name, "\\") {
		return unsafef("archive entry %q contains a backslash: refusing (archive entry names must use forward slashes)", name)
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." {
			return unsafef("archive entry %q contains \"..\": refusing (path traversal)", name)
		}
		base := strings.ToLower(seg)
		if i := strings.IndexByte(base, '.'); i >= 0 {
			base = base[:i]
		}
		if windowsReserved[base] {
			return unsafef("archive entry %q uses a Windows-reserved device name (%s): refusing", name, strings.ToUpper(base))
		}
		trimmed := strings.TrimRight(seg, " .")
		if trimmed != seg && runtime.GOOS == "windows" {
			return unsafef("archive entry %q has a trailing dot or space, which Windows would strip: refusing", name)
		}
	}
	return nil
}

// ListTop returns the sorted top-level entry names of an archive for error
// diagnostics ("binary not found; archive contains: …").
func ListTop(archivePath string, format Format) []string {
	names, err := listTopNames(archivePath, format)
	if err != nil {
		return nil
	}
	return names
}

func listTopNames(archivePath string, format Format) ([]string, error) {
	seen := map[string]bool{}
	add := func(n string) {
		if i := strings.IndexByte(n, '/'); i >= 0 {
			n = n[:i]
		}
		if n != "" {
			seen[n] = true
		}
	}
	switch format {
	case FormatZip:
		f, err := os.Open(archivePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return nil, err
		}
		zr, err := zip.NewReader(f, fi.Size())
		if err != nil {
			return nil, err
		}
		for _, zf := range zr.File {
			add(zf.Name)
		}
	case FormatTarGz:
		f, err := os.Open(archivePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		gz, err := gzip.NewReader(bufio.NewReader(f))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err != nil {
				break
			}
			add(hdr.Name)
		}
	default:
		return nil, fmt.Errorf("listing not supported for format %s", format)
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sortStrings(out)
	return out, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
