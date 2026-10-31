// Package artifact classifies release artifact formats, identifies binary
// metadata from real file headers (ELF/PE/Mach-O), and extracts archives
// safely.
package artifact

import (
	"bufio"
	"compress/gzip"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"io"
	"os"
	"strings"
)

// Format classifies the container format of an artifact.
type Format string

const (
	FormatZip     Format = "zip"
	FormatTarGz   Format = "tar.gz"
	FormatBinary  Format = "binary"
	FormatTar     Format = "tar"
	FormatUnknown Format = "unknown"
)

// BinaryInfo is the platform identity of an executable, derived from its
// real header — never from a filename.
type BinaryInfo struct {
	Format string // elf | pe | macho | script | unknown
	OS     string // linux | windows | darwin | unknown
	Arch   string // amd64 | arm64 | 386 | arm | riscv64 | ppc64 | s390x | unknown
	Detail string // human-readable extra detail (e.g. reason when unknown)
}

// Known reports whether both OS and Arch were identified from the header.
func (b BinaryInfo) Known() bool {
	return b.OS != "unknown" && b.Arch != "unknown"
}

// String renders "elf/linux/amd64" style identity.
func (b BinaryInfo) String() string {
	s := b.Format + "/" + b.OS + "/" + b.Arch
	if b.Detail != "" {
		s += " (" + b.Detail + ")"
	}
	return s
}

// readHeader reads the first 512 bytes for magic inspection.
func readHeader(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := io.ReadFull(bufio.NewReader(f), buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}

// Classify determines the container format of the artifact at path by magic
// bytes. Extensions are only used to phrase the error message, never to
// decide.
func Classify(path string) (Format, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FormatUnknown, fmt.Errorf("cannot access artifact: %w", err)
	}
	if info.IsDir() {
		return FormatUnknown, fmt.Errorf("artifact is a directory, not a file: %s", path)
	}
	hdr, err := readHeader(path)
	if err != nil {
		return FormatUnknown, fmt.Errorf("cannot read artifact header: %w", err)
	}
	switch {
	case len(hdr) >= 4 && string(hdr[:4]) == "PK\x03\x04":
		return FormatZip, nil
	case len(hdr) >= 2 && hdr[0] == 0x1f && hdr[1] == 0x8b:
		// gzip-compressed: is the payload a tar?
		if f, err := os.Open(path); err == nil {
			zr, zerr := gzip.NewReader(bufio.NewReader(f))
			if zerr == nil {
				tbuf := make([]byte, 512)
				n, _ := io.ReadFull(zr, tbuf)
				f.Close()
				if n >= 512 && isTarHeader(tbuf) {
					return FormatTarGz, nil
				}
				return FormatUnknown, fmt.Errorf("gzip file does not contain a tar archive (zip a directory into .tar.gz, or use format: binary for gzipped executables is not supported)")
			}
		}
		return FormatUnknown, fmt.Errorf("gzip stream is not readable")
	case isTarHeader(hdr):
		return FormatTar, nil
	default:
		if bi := DetectBinary(path); bi.Format == "elf" || bi.Format == "pe" || bi.Format == "macho" {
			return FormatBinary, nil
		}
		if len(hdr) >= 2 && hdr[0] == '#' && hdr[1] == '!' {
			return FormatBinary, nil // scripts are runnable standalone artifacts
		}
		magic := hdr
		if len(magic) > 8 {
			magic = magic[:8]
		}
		return FormatUnknown, fmt.Errorf("unrecognized artifact format (starts with %q); supported: zip, tar.gz, standalone executable", hexQuote(magic))
	}
}

func isTarHeader(b []byte) bool {
	if len(b) < 512 {
		return false
	}
	// POSIX/GNU "ustar" magic at offset 257.
	if string(b[257:262]) == "ustar" {
		return true
	}
	// Ancient V7 tar: checksum field validation as fallback.
	return looksLikeV7Tar(b)
}

func looksLikeV7Tar(b []byte) bool {
	// Name field printable, mode/uid/gid octal.
	for _, c := range b[0:100] {
		if c == 0 {
			break
		}
		if c < 32 || c > 126 {
			return false
		}
	}
	return isOctalField(b[100:108]) && isOctalField(b[108:116])
}

func isOctalField(b []byte) bool {
	sawDigit := false
	for _, c := range b {
		if c == 0 || c == ' ' {
			continue
		}
		if c < '0' || c > '7' {
			return false
		}
		sawDigit = true
	}
	return sawDigit
}

func hexQuote(b []byte) string {
	var sb strings.Builder
	sb.WriteString("0x")
	for _, c := range b {
		fmt.Fprintf(&sb, "%02x", c)
	}
	return sb.String()
}

// DetectBinary identifies the executable format and platform of the file at
// path by parsing its header. Unknown formats are reported honestly — never
// guessed.
func DetectBinary(path string) BinaryInfo {
	f, err := os.Open(path)
	if err != nil {
		return BinaryInfo{Format: "unknown", OS: "unknown", Arch: "unknown", Detail: "cannot open file"}
	}
	defer f.Close()

	hdr, err := readHeader(path)
	if err != nil {
		return BinaryInfo{Format: "unknown", OS: "unknown", Arch: "unknown", Detail: "cannot read header"}
	}

	if len(hdr) >= 2 && hdr[0] == '#' && hdr[1] == '!' {
		line := hdr
		if i := indexByte(line, '\n'); i >= 0 {
			line = line[:i]
		}
		return BinaryInfo{Format: "script", OS: "unknown", Arch: "unknown",
			Detail: "shebang script; platform not determinable from content"}
	}

	// ELF
	if len(hdr) >= 4 && string(hdr[:4]) == "\x7fELF" {
		if ef, err := elf.NewFile(f); err == nil {
			defer ef.Close()
			return BinaryInfo{Format: "elf", OS: "linux", Arch: elfArch(ef.Machine)}
		}
		return BinaryInfo{Format: "elf", OS: "linux", Arch: "unknown", Detail: "ELF header present but not parseable"}
	}

	// Mach-O (32/64, both endians)
	if len(hdr) >= 4 && (string(hdr[:4]) == "\xfe\xed\xfa\xce" || string(hdr[:4]) == "\xfe\xed\xfa\xcf" ||
		string(hdr[:4]) == "\xce\xfa\xed\xfe" || string(hdr[:4]) == "\xcf\xfa\xed\xfe") {
		if mf, err := macho.NewFile(f); err == nil {
			defer mf.Close()
			return BinaryInfo{Format: "macho", OS: "darwin", Arch: machoArch(mf.Cpu)}
		}
		return BinaryInfo{Format: "macho", OS: "darwin", Arch: "unknown", Detail: "Mach-O header present but not parseable"}
	}

	// PE (MZ)
	if len(hdr) >= 2 && hdr[0] == 'M' && hdr[1] == 'Z' {
		if pf, err := pe.NewFile(f); err == nil {
			defer pf.Close()
			return BinaryInfo{Format: "pe", OS: "windows", Arch: peArch(pf.Machine)}
		}
		return BinaryInfo{Format: "pe", OS: "windows", Arch: "unknown", Detail: "MZ header present but PE not parseable (is this a text file starting with MZ?)"}
	}

	// WebAssembly / other runnable formats are not supported: report unknown.
	return BinaryInfo{Format: "unknown", OS: "unknown", Arch: "unknown",
		Detail: "no recognizable executable header (ELF, PE, Mach-O, shebang)"}
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

func elfArch(m elf.Machine) string {
	switch m {
	case elf.EM_X86_64:
		return "amd64"
	case elf.EM_386:
		return "386"
	case elf.EM_AARCH64:
		return "arm64"
	case elf.EM_ARM:
		return "arm"
	case elf.EM_RISCV:
		return "riscv64"
	case elf.EM_PPC64:
		return "ppc64"
	case elf.EM_S390:
		return "s390x"
	case elf.EM_LOONGARCH:
		return "loong64"
	case elf.EM_MIPS:
		return "mips"
	default:
		return "unknown"
	}
}

func machoArch(c macho.Cpu) string {
	switch c {
	case macho.CpuAmd64:
		return "amd64"
	case macho.CpuArm64:
		return "arm64"
	case macho.Cpu386:
		return "386"
	case macho.CpuArm:
		return "arm"
	default:
		return "unknown"
	}
}

func peArch(m uint16) string {
	const (
		imageFileMachineI386  = 0x014c
		imageFileMachineArm   = 0x01c0
		imageFileMachineArmNT = 0x01c4
		imageFileMachineAmd64 = 0x8664
		imageFileMachineArm64 = 0xaa64
	)
	switch m {
	case imageFileMachineAmd64:
		return "amd64"
	case imageFileMachineI386:
		return "386"
	case imageFileMachineArm64:
		return "arm64"
	case imageFileMachineArm, imageFileMachineArmNT:
		return "arm"
	default:
		return "unknown"
	}
}
