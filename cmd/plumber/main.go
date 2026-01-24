package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// --- Message Structures ---

type Envelope struct {
	ID        string `json:"id"`
	Origin    string `json:"origin"`
	URL       string `json:"url"`
	Target    string `json:"target"`
	Timestamp int64  `json:"timestamp"`
}

// --- Global Config ---
var cfg Config

func main() {
	// 0. Parse Flags & Subcommands
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Default command is "run"
	cmd := "run"
	if len(flag.Args()) > 0 {
		cmd = flag.Arg(0)
	}

	// 1. Setup Logging (Stderr)
	log.SetOutput(os.Stderr)
	log.SetFlags(0) // Custom format

	if cmd == "schema" {
		fmt.Println(GenerateJSONSchema())
		return
	}

	log.Println("🔧 Plumber started...")

	// 2. Load Configuration (required for run and validate)
	if err := loadConfig(*configPath); err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	if cmd == "validate" {
		if err := cfg.Validate(); err != nil {
			log.Fatalf("❌ Configuration is invalid: %v", err)
		}
		log.Println("✅ Configuration is valid.")
		return
	}

	if cmd == "run" {
		if err := cfg.Validate(); err != nil {
			log.Fatalf("❌ Configuration is invalid: %v", err)
		}
		// 3. Start Native Messaging Loop
		startLoop()
	} else {
		log.Fatalf("❌ Unknown command: %s. usage: plumber [run|validate|schema]", cmd)
	}
}

// loadConfig loads the YAML configuration
func loadConfig(explicitPath string) error {
	var configPath string
	if explicitPath != "" {
		configPath = explicitPath
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		configPath = filepath.Join(homeDir, ".config", "browser-pipes", "plumber.yaml")
	}

	log.Printf("📂 Loading config from: %s", configPath)

	f, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("could not open config file at %s: %w", configPath, err)
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return fmt.Errorf("could not decode config: %w", err)
	}

	if cfg.Version == "" {
		// Enforce V2
		return fmt.Errorf("invalid config: missing 'version' (must be '2')")
	}

	return nil
}

// startLoop listens on Stdin for Native Messaging messages
func startLoop() {
	// Default size limit (10MB)
	maxSize := uint32(10 * 1024 * 1024)

	for {
		// Native Messaging Protocol:
		// 1. First 4 bytes: length of message (UInt32, Little Endian)
		// 2. N bytes: The JSON message

		var length uint32
		err := binary.Read(os.Stdin, binary.LittleEndian, &length)
		if err == io.EOF {
			log.Println("🔌 Stdin closed, exiting.")
			return
		}
		if err != nil {
			log.Printf("❌ Error reading header: %v", err)
			return
		}

		// Cap message size to avoid OOM or malicious input
		if length > maxSize {
			log.Printf("❌ Message too large: %d bytes (limit: %d)", length, maxSize)
			// Skip or exit? Exiting is safer for Native Messaging.
			return
		}

		msgBuf := make([]byte, length)
		_, err = io.ReadFull(os.Stdin, msgBuf)
		if err != nil {
			log.Printf("❌ Error reading message body: %v", err)
			return
		}

		var env Envelope
		if err := json.Unmarshal(msgBuf, &env); err != nil {
			log.Printf("❌ Error decoding JSON: %v", err)
			continue
		}

		// Handle the message
		handleMessage(env)
	}
}

func handleMessage(env Envelope) {
	// Structured Log
	log.Printf("[%s] [%s] -> [%s] : [%s]",
		time.Unix(env.Timestamp, 0).Format(time.RFC3339),
		env.Origin,
		env.Target,
		env.URL,
	)

	// Clean URL
	cleanedURL := cleanURL(env.URL)
	if cleanedURL != env.URL {
		log.Printf("   Let's clean that up: %s -> %s", env.URL, cleanedURL)
	}
	env.URL = cleanedURL

	if err := ExecuteWorkflowV2(&cfg, env.URL); err != nil {
		log.Printf("   ❌ Workflow Execution Failed: %v", err)
		sendResponse("error", fmt.Sprintf("Workflow failed: %v", err))
	} else {
		sendResponse("success", "Workflow executed")
	}
}

func cleanURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // Return parsing failed, return original
	}

	q := u.Query()
	paramsToDelete := []string{
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"fbclid", "gclid", "ref",
	}

	for _, p := range paramsToDelete {
		q.Del(p)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// --- Response Handling ---

type Response struct {
	Status  string `json:"status"` // "success" or "error"
	Message string `json:"message"`
}

func sendResponse(status, message string) {
	resp := Response{
		Status:  status,
		Message: message,
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("❌ Failed to marshal response: %v", err)
		return
	}

	// Native Messaging requires length prefix (uint32 little endian)
	if err := binary.Write(os.Stdout, binary.LittleEndian, uint32(len(bytes))); err != nil {
		log.Printf("❌ Failed to write response length: %v", err)
		return
	}

	if _, err := os.Stdout.Write(bytes); err != nil {
		log.Printf("❌ Failed to write response body: %v", err)
		return
	}
}
