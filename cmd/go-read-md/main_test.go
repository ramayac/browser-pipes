package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	// Create a temporary directory for all tests
	baseTmpDir, err := os.MkdirTemp("", "go-read-md-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseTmpDir)

	t.Run("Success: URL Fetch", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "<html><body><h1>Example Article</h1><article><p>This is the main content of the article.</p></article></body></html>")
		}))
		defer ts.Close()

		outputDir := filepath.Join(baseTmpDir, "url-fetch")
		stdout := &bytes.Buffer{}
		err := run([]string{"--output", outputDir, ts.URL}, nil, stdout)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !strings.Contains(stdout.String(), "✅ Saved to:") {
			t.Errorf("expected success message, got %q", stdout.String())
		}

		// Verify file existence
		files, _ := os.ReadDir(outputDir)
		if len(files) != 1 {
			t.Errorf("expected 1 file in output directory, got %d", len(files))
		}
	})

	t.Run("Success: File Input", func(t *testing.T) {
		htmlFile := filepath.Join(baseTmpDir, "test.html")
		err := os.WriteFile(htmlFile, []byte("<html><body><h1>File Title</h1><p>File content here.</p></body></html>"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		outputDir := filepath.Join(baseTmpDir, "file-input")
		stdout := &bytes.Buffer{}
		err = run([]string{"--output", outputDir, "--url", "http://test.com", "--input", htmlFile}, nil, stdout)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !strings.Contains(stdout.String(), "✅ Saved to:") {
			t.Errorf("expected success message, got %q", stdout.String())
		}
	})

	t.Run("Success: Stdin Input", func(t *testing.T) {
		stdin := strings.NewReader("<html><body><h1>Stdin Title</h1><p>Stdin content here.</p></body></html>")
		outputDir := filepath.Join(baseTmpDir, "stdin-input")
		stdout := &bytes.Buffer{}
		err := run([]string{"--output", outputDir, "--url", "http://test.com", "--input", "-"}, stdin, stdout)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !strings.Contains(stdout.String(), "✅ Saved to:") {
			t.Errorf("expected success message, got %q", stdout.String())
		}
	})

	t.Run("Error: Missing Output Dir", func(t *testing.T) {
		err := run([]string{"http://example.com"}, nil, ioDiscard())
		if err == nil || !strings.Contains(err.Error(), "--output directory is required") {
			t.Errorf("expected missing output error, got %v", err)
		}
	})

	t.Run("Error: Missing URL", func(t *testing.T) {
		err := run([]string{"--output", "/tmp"}, nil, ioDiscard())
		if err == nil || !strings.Contains(err.Error(), "source URL is required") {
			t.Errorf("expected missing URL error, got %v", err)
		}
	})

	t.Run("Error: HTTP 404", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		err := run([]string{"--output", baseTmpDir, ts.URL}, nil, ioDiscard())
		if err == nil || !strings.Contains(err.Error(), "HTTP error: 404 Not Found") {
			t.Errorf("expected 404 error, got %v", err)
		}
	})

	t.Run("Error: Invalid File path", func(t *testing.T) {
		err := run([]string{"--output", baseTmpDir, "--url", "http://test.com", "--input", "/non/existent/path"}, nil, ioDiscard())
		if err == nil || !strings.Contains(err.Error(), "failed to open input file") {
			t.Errorf("expected file open error, got %v", err)
		}
	})
}

// Helper to silence stdout in tests
func ioDiscard() *bytes.Buffer {
	return &bytes.Buffer{}
}
