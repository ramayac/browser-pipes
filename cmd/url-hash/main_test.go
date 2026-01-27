package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("Success: Normal URL", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		err := run([]string{"http://example.com"}, stdout, stderr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expected := "f0e6a6a9"
		actual := strings.TrimSpace(stdout.String())
		if actual != expected {
			t.Errorf("expected hash %q, got %q", expected, actual)
		}
	})

	t.Run("Success: Stable Hash", func(t *testing.T) {
		stdout1 := &bytes.Buffer{}
		run([]string{"http://google.com"}, stdout1, &bytes.Buffer{})

		stdout2 := &bytes.Buffer{}
		run([]string{"http://google.com"}, stdout2, &bytes.Buffer{})

		if stdout1.String() != stdout2.String() {
			t.Errorf("hashes should be stable, got %q and %q", stdout1.String(), stdout2.String())
		}
	})

	t.Run("Error: Missing Argument", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		err := run([]string{}, stdout, stderr)
		if err == nil {
			t.Fatal("expected error for missing argument, got nil")
		}

		if !strings.Contains(stderr.String(), "Usage:") {
			t.Errorf("expected usage message in stderr, got %q", stderr.String())
		}
	})

	t.Run("Error: Invalid Flag", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		err := run([]string{"--invalid-flag"}, stdout, stderr)
		if err == nil {
			t.Fatal("expected error for invalid flag, got nil")
		}
	})
}
