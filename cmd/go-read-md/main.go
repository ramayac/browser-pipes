package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	md "github.com/JohannesKaufmann/html-to-markdown"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("go-read-md", flag.ContinueOnError)
	outputDir := fs.String("output", "", "Output directory for markdown files (required)")
	filenameOverride := fs.String("filename", "", "Explicit filename to use (optional)")
	inputHTML := fs.String("input", "", "Input HTML file (optional, if hyphen '-' reads from stdin)")
	sourceURL := fs.String("url", "", "Source URL for metadata (required if not a positional argument)")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go-read-md [flags] [url]\n\n")
		fmt.Fprintf(os.Stderr, "Processes HTML content from a URL, a file, or stdin, extracts the article\n")
		fmt.Fprintf(os.Stderr, "content using go-readability, and saves it as a Markdown file.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  go-read-md --output ./read http://example.com\n")
		fmt.Fprintf(os.Stderr, "  cat page.html | go-read-md --output ./read --url http://example.com\n")
		fmt.Fprintf(os.Stderr, "  go-read-md --output ./read --input page.html --url http://example.com\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *outputDir == "" {
		return fmt.Errorf("--output directory is required")
	}

	targetURL := *sourceURL
	if targetURL == "" && fs.NArg() > 0 {
		targetURL = fs.Arg(0)
	}

	if targetURL == "" {
		return fmt.Errorf("source URL is required (via --url or positional argument)")
	}

	// Validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid URL: %s", targetURL)
	}

	// Get HTML content
	var htmlReader io.Reader
	var closer io.Closer

	// Decide input source
	if *inputHTML != "" {
		if *inputHTML == "-" {
			if stdin == nil {
				return fmt.Errorf("stdin is required but not available")
			}
			htmlReader = stdin
		} else {
			f, err := os.Open(*inputHTML)
			if err != nil {
				return fmt.Errorf("failed to open input file: %w", err)
			}
			htmlReader = f
			closer = f
		}
	} else {
		// Check if we should read from stdin (auto-detection)
		isPipe := false
		if stdin != nil {
			if stdin == os.Stdin {
				stat, err := os.Stdin.Stat()
				if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
					isPipe = true
				}
			} else {
				// If it's not os.Stdin but provided (like in tests), treat as piped
				isPipe = true
			}
		}

		if isPipe {
			if *verbose {
				log.Println("📥 Reading from Stdin...")
			}
			htmlReader = stdin
		} else {
			// Fetch URL
			if *verbose {
				log.Printf("🔍 Fetching: %s", targetURL)
			}
			resp, err := http.Get(targetURL)
			if err != nil {
				return fmt.Errorf("failed to fetch URL: %w", err)
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return fmt.Errorf("HTTP error: %s", resp.Status)
			}
			htmlReader = resp.Body
			closer = resp.Body
		}
	}

	if closer != nil {
		defer closer.Close()
	}

	// Parse with go-readability
	article, err := readability.FromReader(htmlReader, parsedURL)
	if err != nil {
		return fmt.Errorf("failed to parse article: %w", err)
	}

	if *verbose {
		log.Printf("📄 Title: %s", article.Title())
		log.Printf("👤 Author: %s", article.Byline())
		pubTime, _ := article.PublishedTime()
		log.Printf("📅 Published: %s", pubTime.Format(time.RFC3339))
	}

	// Convert HTML to Markdown
	var htmlBuf strings.Builder
	if err := article.RenderHTML(&htmlBuf); err != nil {
		return fmt.Errorf("failed to render HTML: %w", err)
	}

	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(htmlBuf.String())
	if err != nil {
		return fmt.Errorf("failed to convert to markdown: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename
	var filename string
	if *filenameOverride != "" {
		filename = *filenameOverride
	} else {
		titleHash := hashString(targetURL)
		filename = sanitizeFilename(article.Title())
		if filename == "" {
			filename = fmt.Sprintf("article_%s", titleHash)
		} else {
			filename = fmt.Sprintf("%s_%s", filename, titleHash)
		}
	}

	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}

	outputPath := filepath.Join(*outputDir, filename)

	// Build the full markdown document
	var fullMarkdown strings.Builder
	fullMarkdown.WriteString(fmt.Sprintf("# %s\n\n", article.Title()))
	if article.Byline() != "" {
		fullMarkdown.WriteString(fmt.Sprintf("**Author:** %s\n\n", article.Byline()))
	}
	pubTime, err := article.PublishedTime()
	if err == nil && !pubTime.IsZero() {
		fullMarkdown.WriteString(fmt.Sprintf("**Published:** %s\n\n", pubTime.Format(time.RFC3339)))
	}
	fullMarkdown.WriteString(fmt.Sprintf("**Source:** [%s](%s)\n\n", targetURL, targetURL))
	fullMarkdown.WriteString(fmt.Sprintf("**Saved:** %s\n\n", time.Now().Format(time.RFC3339)))
	fullMarkdown.WriteString("---\n\n")
	fullMarkdown.WriteString(markdown)

	// Write to file
	if err := os.WriteFile(outputPath, []byte(fullMarkdown.String()), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Fprintf(stdout, "✅ Saved to: %s\n", outputPath)
	return nil
}

// sanitizeFilename creates a safe filename from a title
func sanitizeFilename(title string) string {
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	safe := reg.ReplaceAllString(title, "")
	safe = strings.ReplaceAll(safe, " ", "_")
	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_")
	safe = strings.TrimSpace(safe)
	safe = strings.Trim(safe, "_-")
	if len(safe) > 100 {
		safe = safe[:100]
	}
	return safe
}

func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))[:8]
}
