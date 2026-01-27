package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "plumber-main-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	validConfigPath := filepath.Join(tmpDir, "valid.yaml")
	validConfig := `
version: "2"
jobs:
  default:
    steps:
      - run: "echo hello"
workflows:
  main:
    jobs:
      - default:
          match: ".*"
`
	os.WriteFile(validConfigPath, []byte(validConfig), 0644)

	t.Run("Command: schema", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		err := run([]string{"schema"}, nil, stdout, io.Discard)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !strings.Contains(stdout.String(), "\"$schema\"") {
			t.Errorf("expected JSON schema in output, got %q", stdout.String())
		}
	})

	t.Run("Command: validate success", func(t *testing.T) {
		stderr := &bytes.Buffer{}
		err := run([]string{"-config", validConfigPath, "validate"}, nil, io.Discard, stderr)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !strings.Contains(stderr.String(), "✅ Configuration is valid.") {
			t.Errorf("expected success message, got %q", stderr.String())
		}
	})

	t.Run("Command: validate failure (missing version)", func(t *testing.T) {
		invalidConfigPath := filepath.Join(tmpDir, "invalid.yaml")
		os.WriteFile(invalidConfigPath, []byte("jobs: {}"), 0644)

		err := run([]string{"-config", invalidConfigPath, "validate"}, nil, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "missing 'version'") {
			t.Errorf("expected validation error, got %v", err)
		}
	})

	t.Run("Native Messaging Loop", func(t *testing.T) {
		// Prepare a mock message
		msg := Envelope{
			URL:       "https://example.com?utm_source=test",
			Timestamp: 1679800000,
			Origin:    "test",
		}
		msgBytes, _ := json.Marshal(msg)

		var stdin bytes.Buffer
		binary.Write(&stdin, binary.LittleEndian, uint32(len(msgBytes)))
		stdin.Write(msgBytes)

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		// Run with the valid config
		// Note: This will execute the workflow. Since it's a mock test,
		// we just want to see it process one message and exit when stdin closes.
		err := run([]string{"-config", validConfigPath, "run"}, &stdin, stdout, stderr)
		if err != nil {
			t.Errorf("run failed: %v", err)
		}

		// Check if it cleaned the URL in the logs
		if !strings.Contains(stderr.String(), "Let's clean that up") {
			t.Errorf("expected URL cleaning log, got %q", stderr.String())
		}

		// Check if it sent a response
		if stdout.Len() < 4 {
			t.Fatal("no response sent to stdout")
		}
		var respLen uint32
		binary.Read(stdout, binary.LittleEndian, &respLen)
		respBytes := make([]byte, respLen)
		stdout.Read(respBytes)

		var resp Response
		json.Unmarshal(respBytes, &resp)
		if resp.Status != "success" {
			t.Errorf("expected success status, got %q (message: %q)", resp.Status, resp.Message)
		}
	})
}

func TestCleanURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://example.com", "https://example.com"},
		{"https://example.com?utm_source=news", "https://example.com"},
		{"https://example.com?fbclid=123&keep=me", "https://example.com?keep=me"},
		{"invalid-url", "invalid-url"},
	}

	for _, tt := range tests {
		actual := cleanURL(tt.input)
		if actual != tt.expected {
			t.Errorf("cleanURL(%q) = %q, want %q", tt.input, actual, tt.expected)
		}
	}
}
