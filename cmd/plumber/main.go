package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
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
	SnapshotFolder  string   `yaml:"snapshot_folder"`
	SnapshotFormats []string `yaml:"snapshot_formats"`
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
	// 1. Setup Logging (Stderr)
	log.SetOutput(os.Stderr)
	log.SetFlags(0) // Custom format

	log.Println("🔧 Plumber started...")

	// 2. Load Configuration
	if err := loadConfig(); err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	// 3. Start Native Messaging Loop
	startLoop()
}

// loadConfig loads the YAML configuration from ~/.config/browser-pipe/plumber.yaml
func loadConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(homeDir, ".config", "browser-pipe", "plumber.yaml")

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

		// Cap message size to avoid OOM or malicious input (e.g., 10MB)
		if length > 10*1024*1024 {
			log.Printf("❌ Message too large: %d bytes", length)
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
	if target == "snapshot" {
		if err := performSnapshot(env.URL); err != nil {
			log.Printf("   ❌ Snapshot failed: %v", err)
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

func performSnapshot(targetURL string) error {
	log.Printf("   📸 Snapshotting: %s", targetURL)

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
	saveDir := cfg.Settings.SnapshotFolder
	if strings.HasPrefix(saveDir, "~/") {
		home, _ := os.UserHomeDir()
		saveDir = filepath.Join(home, saveDir[2:])
	}

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot dir: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02-1504")

	// Create a safe slug
	slug := sanitizeFilename(article.Title())
	if slug == "" {
		slug = "untitled"
	}
	baseFilename := fmt.Sprintf("%s-%s", timestamp, slug)

	createdFiles := []string{}

	// 3. Save Formats
	for _, fmtType := range cfg.Settings.SnapshotFormats {
		path := filepath.Join(saveDir, baseFilename+"."+fmtType)
		var content []byte

		switch fmtType {
		case "html":
			var buf bytes.Buffer
			if err := article.RenderHTML(&buf); err != nil {
				log.Printf("   ⚠️ Error rendering HTML: %v", err)
			}

			// Simple clean HTML wrapper
			html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>%s</title>
<style>body{font-family:sans-serif;max-width:800px;margin:2em auto;line-height:1.6;padding:0 1em;}img{max-width:100%%;height:auto;}</style>
</head>
<body>
<h1>%s</h1>
%s
</body>
</html>`, article.Title(), article.Title(), buf.String())
			content = []byte(html)
		case "md":
			var buf bytes.Buffer
			if err := article.RenderText(&buf); err != nil {
				log.Printf("   ⚠️ Error rendering Text: %v", err)
			}
			content = []byte(fmt.Sprintf("# %s\n\n%s", article.Title(), buf.String()))
		}

		if len(content) > 0 {
			if err := os.WriteFile(path, content, 0644); err != nil {
				log.Printf("   ❌ Failed to write %s: %v", fmtType, err)
			} else {
				log.Printf("   💾 Saved: %s", path)
				createdFiles = append(createdFiles, path)
			}
		}
	}

	// 4. Open in default target
	// Spec: "Automatically open the resulting local file in the default_target browser."
	// Wait, "default_target" isn't a defined key in config, it says "default target browser".
	// Since we don't have a "default" key, we might need to pick one or look at the 'toggles' logic?
	// Or maybe the 'target' in the message? But the target was 'snapshot'.
	// I'll assume we open it in the system default or a specific browser from config.
	// Looking at config example: No "default" key.
	// However, if we look at `toggles`, maybe we can infer?
	// Let's assume the user wants it opened in "chrome" or strictly follow a "default" if it existed.
	// But it doesn't.
	// The prompt says: "Automatically open the resulting local file in the `default_target` browser."
	// Maybe they meant the rule target?
	// Let's assume we just open it with `xdg-open` (system default) or try to find a browser "chrome".
	// SAFEST BET: Use `xdg-open` on Linux, which respects system default.

	if len(createdFiles) > 0 {
		// Open the first one (likely HTML if preferred)
		fileToOpen := createdFiles[0]
		cmd := exec.Command("xdg-open", fileToOpen) // Linux specific
		cmd.Start()
	}

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

	// Detach process so it doesn't die when plumber dies (if Plumber is short lived, but Plumber is a listener here)
	// However, browsers usually fork anyway.
	if err := cmd.Start(); err != nil {
		log.Printf("   ❌ Failed to launch browser: %v", err)
	}
}

func sanitizeFilename(name string) string {
	// Simple sanitize
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	return strings.ToLower(strings.Trim(reg.ReplaceAllString(name, "-"), "-"))
}
