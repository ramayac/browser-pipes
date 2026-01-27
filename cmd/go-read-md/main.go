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

var (
	outputDir        = flag.String("output", "", "Output directory for markdown files (required)")
	filenameOverride = flag.String("filename", "", "Explicit filename to use (optional)")
	inputHTML        = flag.String("input", "", "Input HTML file (optional, if hyphen '-' reads from stdin)")
	sourceURL        = flag.String("url", "", "Source URL for metadata (required if not a positional argument)")
	verbose          = flag.Bool("verbose", false, "Enable verbose logging")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [url]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Processes HTML content from a URL, a file, or stdin, extracts the article\n")
		fmt.Fprintf(os.Stderr, "content using go-readability, and saves it as a Markdown file.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s --output ./read http://example.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  cat page.html | %s --output ./read --url http://example.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --output ./read --input page.html --url http://example.com\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *outputDir == "" {
		log.Fatal("❌ Error: --output directory is required")
	}

	targetURL := *sourceURL
	if targetURL == "" && flag.NArg() > 0 {
		targetURL = flag.Arg(0)
	}

	if targetURL == "" {
		log.Fatal("❌ Error: Source URL is required (via --url or positional argument)")
	}

	// Validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		log.Fatalf("❌ Invalid URL: %s", targetURL)
	}

	// Get HTML content
	var htmlReader io.Reader
	var closer io.Closer

	// Decide input source
	// 1. Explicit file or stdin via --input
	if *inputHTML != "" {
		if *inputHTML == "-" {
			htmlReader = os.Stdin
		} else {
			f, err := os.Open(*inputHTML)
			if err != nil {
				log.Fatalf("❌ Failed to open input file: %v", err)
			}
			htmlReader = f
			closer = f
		}
	} else {
		// 2. Check if Stdin is a pipe
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			if *verbose {
				log.Println("📥 Reading from Stdin...")
			}
			htmlReader = os.Stdin
		} else {
			// 3. Fallback: Fetch URL
			if *verbose {
				log.Printf("🔍 Fetching: %s", targetURL)
			}
			resp, err := http.Get(targetURL)
			if err != nil {
				log.Fatalf("❌ Failed to fetch URL: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				log.Fatalf("❌ HTTP error: %s", resp.Status)
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
		log.Fatalf("❌ Failed to parse article: %v", err)
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
		log.Fatalf("❌ Failed to render HTML: %v", err)
	}

	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(htmlBuf.String())
	if err != nil {
		log.Fatalf("❌ Failed to convert to markdown: %v", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create output directory: %v", err)
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
		log.Fatalf("❌ Failed to write file: %v", err)
	}

	fmt.Printf("✅ Saved to: %s\n", outputPath)
}

// sanitizeFilename creates a safe filename from a title
func sanitizeFilename(title string) string {
	// Remove or replace invalid characters
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	safe := reg.ReplaceAllString(title, "")

	// Replace spaces and multiple dashes
	safe = strings.ReplaceAll(safe, " ", "_")
	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_")

	// Trim and limit length
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
