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
	HTML      string `json:"html,omitempty"` // Optional HTML content for paywalled articles
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("plumber", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cmd := "run"
	if fs.NArg() > 0 {
		cmd = fs.Arg(0)
	}

	log.SetOutput(stderr)
	log.SetFlags(0)

	if cmd == "schema" {
		fmt.Fprintln(stdout, GenerateJSONSchema())
		return nil
	}

	log.Println("🔧 Plumber started...")

	var cfg Config
	if err := loadConfig(*configPath, &cfg, stderr); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cmd == "validate" {
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("configuration is invalid: %w", err)
		}
		log.Println("✅ Configuration is valid.")
		return nil
	}

	if cmd == "run" {
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("configuration is invalid: %w", err)
		}
		startLoop(stdin, stdout, &cfg)
		return nil
	}

	return fmt.Errorf("unknown command: %s. usage: plumber [run|validate|schema]", cmd)
}

func loadConfig(explicitPath string, cfg *Config, stderr io.Writer) error {
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

	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("could not decode config: %w", err)
	}

	if cfg.Version == "" {
		return fmt.Errorf("invalid config: missing 'version' (must be '2')")
	}

	return nil
}

func startLoop(stdin io.Reader, stdout io.Writer, cfg *Config) {
	maxSize := uint32(10 * 1024 * 1024)

	for {
		var length uint32
		err := binary.Read(stdin, binary.LittleEndian, &length)
		if err == io.EOF {
			log.Println("🔌 Stdin closed, exiting.")
			return
		}
		if err != nil {
			log.Printf("❌ Error reading header: %v", err)
			return
		}

		if length > maxSize {
			log.Printf("❌ Message too large: %d bytes (limit: %d)", length, maxSize)
			return
		}

		msgBuf := make([]byte, length)
		_, err = io.ReadFull(stdin, msgBuf)
		if err != nil {
			log.Printf("❌ Error reading message body: %v", err)
			return
		}

		var env Envelope
		if err := json.Unmarshal(msgBuf, &env); err != nil {
			log.Printf("❌ Error decoding JSON: %v", err)
			continue
		}

		handleMessage(env, stdout, cfg)
	}
}

func handleMessage(env Envelope, stdout io.Writer, cfg *Config) {
	log.Printf("[%s] [%s] -> [%s] : [%s]",
		time.Unix(env.Timestamp, 0).Format(time.RFC3339),
		env.Origin,
		env.Target,
		env.URL,
	)

	cleanedURL := cleanURL(env.URL)
	if cleanedURL != env.URL {
		log.Printf("   Let's clean that up: %s -> %s", env.URL, cleanedURL)
	}
	env.URL = cleanedURL

	if err := ExecuteWorkflowV2(cfg, env.URL, env.HTML); err != nil {
		log.Printf("   ❌ Workflow Execution Failed: %v", err)
		sendResponse("error", fmt.Sprintf("Workflow failed: %v", err), stdout)
	} else {
		sendResponse("success", "Workflow executed", stdout)
	}
}

func cleanURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
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

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func sendResponse(status, message string, stdout io.Writer) {
	resp := Response{
		Status:  status,
		Message: message,
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("❌ Failed to marshal response: %v", err)
		return
	}

	if err := binary.Write(stdout, binary.LittleEndian, uint32(len(bytes))); err != nil {
		log.Printf("❌ Failed to write response length: %v", err)
		return
	}

	if _, err := stdout.Write(bytes); err != nil {
		log.Printf("❌ Failed to write response body: %v", err)
		return
	}
}
