package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"gopkg.in/yaml.v3"
)

// --- Configuration Structures ---

type Config struct {
	Settings Settings          `yaml:"settings"`
	Browsers map[string]string `yaml:"browsers"`
	Toggles  map[string]string `yaml:"toggles"`
	Rules    []Rule            `yaml:"rules"`
	Actions  map[string]Action `yaml:"actions"`
}

type Settings struct {
	MarkdownFolder string `yaml:"markdown_folder"`
	MaxMessageSize int    `yaml:"max_message_size"`
}

type Rule struct {
	Match  string `yaml:"match"`
	Target string `yaml:"target"`
}

type Action struct {
	Cmd  string   `yaml:"cmd"`
	Args []string `yaml:"args"`
}

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
	// 0. Parse Flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// 1. Setup Logging (Stderr)
	log.SetOutput(os.Stderr)
	log.SetFlags(0) // Custom format

	log.Println("🔧 Plumber started...")

	// 2. Load Configuration
	if err := loadConfig(*configPath); err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	// 3. Start Native Messaging Loop
	startLoop()
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

	// Create default config if not exists (optional, but good for first run experience,
	// though not strictly requested. I will skip creation to strictly follow "Listener" role,
	// assuming user provides it or we fail. But for robustness, let's just try to read).

	f, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("could not open config file at %s: %w", configPath, err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return fmt.Errorf("could not decode config: %w", err)
	}

	// Set defaults
	if cfg.Settings.MaxMessageSize <= 0 {
		cfg.Settings.MaxMessageSize = 10 * 1024 * 1024 // Default 10MB
	}

	return nil
}

// startLoop listens on Stdin for Native Messaging messages
func startLoop() {
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
		if length > uint32(cfg.Settings.MaxMessageSize) {
			log.Printf("❌ Message too large: %d bytes (limit: %d)", length, cfg.Settings.MaxMessageSize)
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

	// Determine Target
	target := env.Target

	// Rule-based routing if target is empty or "toggle" isn't strictly enforced yet (but spec says Toggle is explicit)
	// Spec says: "If target is empty, the Plumber uses its routing rules."
	if target == "" {
		for _, rule := range cfg.Rules {
			matched, _ := regexp.MatchString(rule.Match, env.URL)
			if matched {
				target = rule.Target
				log.Printf("   Matched Rule: '%s' -> Target: '%s'", rule.Match, target)
				break
			}
		}
	}

	// Logic for "toggle"
	if target == "toggle" {
		if val, ok := cfg.Toggles[env.Origin]; ok {
			target = val
		} else {
			log.Printf("   ⚠️ No toggle defined for origin '%s'", env.Origin)
			return
		}
	}

	// Execution
	if target == "markdown" {
		if err := performMarkdown(env.URL); err != nil {
			log.Printf("   ❌ Markdown save failed: %v", err)
			sendResponse("error", fmt.Sprintf("Markdown save failed: %v", err))
		} else {
			sendResponse("success", "Page saved as Markdown")
		}
	} else if action, ok := cfg.Actions[target]; ok {
		// Custom Action Execution
		executeAction(target, action, env.URL)
	} else {
		// Assume target is a browser alias
		launchBrowser(target, env.URL)
	}
}

func executeAction(name string, action Action, targetURL string) {
	log.Printf("   🎬 Executing Action: %s", name)

	// Prepare args with substitution
	cmdArgs := make([]string, len(action.Args))
	for i, arg := range action.Args {
		// Simple substitution for now.
		// Security Note: In a real system we should be careful about shell injection if not using exec.Command directly (which we are below).
		// However, we are passing arguments to exec.Command, so it's safer than shell execution.
		cmdArgs[i] = strings.ReplaceAll(arg, "{url}", targetURL)
	}

	cmd := exec.Command(action.Cmd, cmdArgs...)

	// We might want to see output?
	// For now, let's just log if it starts.
	// Maybe piping stdout/stderr to log would be good for debugging actions like yt-dlp.

	if err := cmd.Start(); err != nil {
		log.Printf("   ❌ Action failed to start: %v", err)
		return
	}

	log.Printf("   ✅ Action started: %s (PID: %d)", action.Cmd, cmd.Process.Pid)

	// Fire and forget or wait?
	// For browsers we fire and forget. For downloads, maybe we want to know if it finished?
	// But NativeMessaging is request/response-ish or fire-ish.
	// We don't want to block the plumbers loop for a long download.
	// So async is correct.
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("   ⚠️ Action '%s' finished with error: %v", name, err)
		} else {
			log.Printf("   ✨ Action '%s' finished successfully", name)
		}
	}()
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

func performMarkdown(targetURL string) error {
	log.Printf("   📝 Saving Markdown: %s", targetURL)

	// 1. Fetch and Readability
	// Custom HTTP Client to set User-Agent (Wikipedia and others block empty/Go-http-client)
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to fetch URL, status: %d", resp.StatusCode)
	}

	// Use FromReader instead of FromURL
	article, err := readability.FromReader(resp.Body, parseURL(targetURL))
	if err != nil {
		return fmt.Errorf("failed to extract content: %w", err)
	}

	// 2. Prepare Output Path
	// Resolve ~ in path
	rawDir := cfg.Settings.MarkdownFolder
	saveDir := rawDir
	if strings.HasPrefix(saveDir, "~/") {
		home, _ := os.UserHomeDir()
		saveDir = filepath.Join(home, saveDir[2:])
	}

	if saveDir == "" {
		return fmt.Errorf("markdown_folder is empty in config (Home: %s)", func() string { h, _ := os.UserHomeDir(); return h }())
	}

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("failed to create markdown dir '%s' (raw: '%s'): %w", saveDir, rawDir, err)
	}

	timestamp := time.Now().Format("2006-01-02-1504")

	// Create a safe slug
	slug := sanitizeFilename(article.Title())
	if slug == "" {
		slug = "untitled"
	}
	baseFilename := fmt.Sprintf("%s-%s", timestamp, slug)

	// 3. Save Markdown
	path := filepath.Join(saveDir, baseFilename+".md")
	var buf bytes.Buffer
	if err := article.RenderText(&buf); err != nil {
		return fmt.Errorf("failed to render text: %w", err)
	}

	content := []byte(fmt.Sprintf("# %s\n\n%s", article.Title(), buf.String()))
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	log.Printf("   💾 Saved: %s", path)
	return nil
}

func launchBrowser(browserAlias, targetURL string) {
	cmdName, ok := cfg.Browsers[browserAlias]
	if !ok {
		log.Printf("   ❌ Unknown browser alias: '%s'", browserAlias)
		return
	}

	log.Printf("   🚀 Launching %s (%s)", browserAlias, cmdName)

	// Prepare command
	cmd := exec.Command(cmdName, targetURL)

	if err := cmd.Start(); err != nil {
		log.Printf("   ❌ Failed to launch browser: %v", err)
		sendResponse("error", fmt.Sprintf("Failed to launch %s: %v", browserAlias, err))
		return
	}

	sendResponse("success", fmt.Sprintf("Opened in %s", browserAlias))
}

func sanitizeFilename(name string) string {
	// Simple sanitize
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	return strings.ToLower(strings.Trim(reg.ReplaceAllString(name, "-"), "-"))
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
