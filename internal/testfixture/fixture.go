// Package testfixture builds a small real CLI program (the "hello tool") for
// use by tests across platforms: it cross-compiles with the local Go
// toolchain, so tests exercise genuine ELF/PE/Mach-O binaries.
package testfixture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// Source of the fixture tool. Flags:
//
//	--version        prints "tool version 1.2.3"
//	--help           prints usage text
//	sleep N          sleeps N seconds (default 60)
//	spawn-sleep N    spawns itself as a child sleeping N seconds, then sleeps
//	exit N           exits with code N
//	stderr MSG       prints MSG to stderr, exit 0
//	echo A B C...    prints args joined by spaces
//	flood            prints millions of lines (output flood test)
//	crash            panics (non-zero exit via runtime error)
//	stdin            reads a line from stdin then prints it
const helloSrc = `package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("usage: hello [--version|--help|sleep N|spawn-sleep N|exit N|stderr MSG|echo ...|flood|crash|stdin]")
		os.Exit(2)
	}
	switch args[0] {
	case "--version":
		fmt.Println("tool version 1.2.3")
	case "--help":
		fmt.Println("hello — fixture tool")
		fmt.Println("flags: --version --help")
	case "sleep":
		n := 60
		if len(args) > 1 {
			n, _ = strconv.Atoi(args[1])
		}
		time.Sleep(time.Duration(n) * time.Second)
	case "spawn-sleep":
		n := 30
		if len(args) > 1 {
			n, _ = strconv.Atoi(args[1])
		}
		exe, _ := os.Executable()
		cmd := exec.Command(exe, "sleep", strconv.Itoa(n))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "spawn failed:", err)
			os.Exit(3)
		}
		fmt.Printf("child pid %d\n", cmd.Process.Pid)
		time.Sleep(time.Duration(n) * time.Second)
	case "exit":
		code := 0
		if len(args) > 1 {
			code, _ = strconv.Atoi(args[1])
		}
		os.Exit(code)
	case "stderr":
		if len(args) > 1 {
			fmt.Fprintln(os.Stderr, args[1])
		}
	case "echo":
		fmt.Println(strings.Join(args[1:], " "))
	case "flood":
		for i := 0; i < 5000000; i++ {
			fmt.Printf("line-%d-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n", i)
		}
	case "crash":
		var xs []int
		_ = xs[42]
	case "stdin":
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		fmt.Print("got: " + line)
	default:
		fmt.Fprintln(os.Stderr, "unknown arg", args[0])
		os.Exit(2)
	}
}
`

const goMod = "module hello\n\ngo 1.24\n"

var (
	once    sync.Once
	rootDir string
	rootErr error
)

func ensureRoot() (string, error) {
	once.Do(func() {
		rootDir, rootErr = os.MkdirTemp("", "shipsmoke-fixtures")
		if rootErr != nil {
			return
		}
		src := filepath.Join(rootDir, "src")
		if err := os.MkdirAll(src, 0o755); err != nil {
			rootErr = err
			return
		}
		if err := os.WriteFile(filepath.Join(src, "main.go"), []byte(helloSrc), 0o644); err != nil {
			rootErr = err
			return
		}
		if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte(goMod), 0o644); err != nil {
			rootErr = err
			return
		}
	})
	return rootDir, rootErr
}

// BinPath returns the cached binary path for the target platform, building
// it on first use.
func BinPath(t testing.TB, goos, goarch string) string {
	t.Helper()
	root, err := ensureRoot()
	if err != nil {
		t.Fatalf("fixture root: %v", err)
	}
	name := fmt.Sprintf("hello-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	out := filepath.Join(root, name)
	if _, err := os.Stat(out); err == nil {
		return out
	}
	src := filepath.Join(root, "src")
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go"+ext())
	cmd := exec.Command(goBin, "build", "-o", out, ".")
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building fixture for %s/%s: %v\n%s", goos, goarch, err, outb)
	}
	return out
}

// HostBinary returns the fixture binary for the current platform.
func HostBinary(t testing.TB) string {
	return BinPath(t, runtime.GOOS, runtime.GOARCH)
}

func ext() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
