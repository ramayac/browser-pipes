package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <url>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Outputs an 8-character SHA-256 hash of the given URL.\n")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		os.Exit(1)
	}

	url := flag.Arg(0)
	h := sha256.New()
	h.Write([]byte(url))
	hash := fmt.Sprintf("%x", h.Sum(nil))
	fmt.Println(hash[:8])
}
