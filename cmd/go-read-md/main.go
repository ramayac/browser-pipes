package main

import (
	"flag"
	"fmt"
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
	outputDir = flag.String("output", "", "Output directory for markdown files (required)")
	verbose   = flag.Bool("verbose", false, "Enable verbose logging")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <url>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Fetches a URL, extracts the article content using go-readability,\n")
		fmt.Fprintf(os.Stderr, "and saves it as a Markdown file.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *outputDir == "" {
		log.Fatal("❌ Error: --output directory is required")
	}

	if flag.NArg() < 1 {
		log.Fatal("❌ Error: URL argument is required")
	}

	targetURL := flag.Arg(0)

	// Validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		log.Fatalf("❌ Invalid URL: %s", targetURL)
	}

	if *verbose {
		log.Printf("🔍 Fetching: %s", targetURL)
	}

	// Fetch the URL
	resp, err := http.Get(targetURL)
	if err != nil {
		log.Fatalf("❌ Failed to fetch URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("❌ HTTP error: %s", resp.Status)
	}

	// Parse with go-readability
	article, err := readability.FromReader(resp.Body, parsedURL)
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
	filename := sanitizeFilename(article.Title())
	if filename == "" {
		filename = fmt.Sprintf("article_%d", time.Now().Unix())
	}
	filename = filename + ".md"

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
