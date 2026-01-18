package main

import (
	"encoding/binary"
	"io"
	"os"
)

func main() {
	// Read all input (JSON) from stdin
	input, _ := io.ReadAll(os.Stdin)

	// Write the length of the message (4 bytes, Little Endian)
	binary.Write(os.Stdout, binary.LittleEndian, uint32(len(input)))

	// Write the message itself
	os.Stdout.Write(input)
}
