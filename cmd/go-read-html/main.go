package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"crypto/sha256"

	readability "codeberg.org/readeck/go-readability/v2"
	md "github.com/JohannesKaufmann/html-to-markdown"
)

var (
	outputDir        = flag.String("output", "", "Output directory for markdown files (required)")
	inputHTML        = flag.String("input", "", "Input HTML file (if not provided, reads from stdin)")
	filenameOverride = flag.String("filename", "", "Explicit filename to use (optional)")
	sourceURL        = flag.String("url", "", "Source URL for metadata (required)")
	verbose          = flag.Bool("verbose", false, "Enable verbose logging")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Processes HTML content (from file or stdin), extracts article content using go-readability,\n")
		fmt.Fprintf(os.Stderr, "and saves it as a Markdown file.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *outputDir == "" {
		log.Fatal("❌ Error: --output directory is required")
	}

	if *sourceURL == "" {
		log.Fatal("❌ Error: --url is required for metadata")
	}

	// Validate URL
	parsedURL, err := url.Parse(*sourceURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		log.Fatalf("❌ Invalid URL: %s", *sourceURL)
	}

	if *verbose {
		log.Printf("🔍 Processing HTML for: %s", *sourceURL)
	}

	// Get HTML content
	var htmlReader io.Reader
	if *inputHTML != "" {
		// Read from file
		f, err := os.Open(*inputHTML)
		if err != nil {
			log.Fatalf("❌ Failed to open input file: %v", err)
		}
		defer f.Close()
		htmlReader = f
	} else {
		// Read from stdin
		htmlReader = os.Stdin
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
	// First render the article HTML
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

	// Generate filename from title or URL
	var filename string
	if *filenameOverride != "" {
		filename = *filenameOverride
	} else {
		titleHash := hashString(*sourceURL)
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

	// Build the full markdown document with metadata
	var fullMarkdown strings.Builder
	fullMarkdown.WriteString(fmt.Sprintf("# %s\n\n", article.Title()))
	if article.Byline() != "" {
		fullMarkdown.WriteString(fmt.Sprintf("**Author:** %s\n\n", article.Byline()))
	}
	pubTime, err := article.PublishedTime()
	if err == nil && !pubTime.IsZero() {
		fullMarkdown.WriteString(fmt.Sprintf("**Published:** %s\n\n", pubTime.Format(time.RFC3339)))
	}
	fullMarkdown.WriteString(fmt.Sprintf("**Source:** [%s](%s)\n\n", *sourceURL, *sourceURL))
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
