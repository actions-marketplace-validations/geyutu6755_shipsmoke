// Package main is the Go example tool verified by shipsmoke.
package main

import (
	"fmt"
	"os"
	"strconv"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: gokit [--version|--help|exit N]")
		return
	}
	switch os.Args[1] {
	case "--version":
		fmt.Printf("gokit version %s\n", version)
	case "--help":
		fmt.Println("gokit — Go example tool for shipsmoke")
	case "exit":
		if len(os.Args) > 2 {
			n, _ := strconv.Atoi(os.Args[2])
			os.Exit(n)
		}
	default:
		fmt.Fprintf(os.Stderr, "gokit: unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}
