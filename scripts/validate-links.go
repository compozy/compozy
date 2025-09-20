package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

type LinkType string

const (
	LinkTypeInternal LinkType = "internal"
	LinkTypeExternal LinkType = "external"
	LinkTypeAnchor   LinkType = "anchor"
	LinkTypeFile     LinkType = "file"
)

type Link struct {
	URL        string
	Text       string
	Type       LinkType
	SourceFile string
	LineNumber int
	Valid      bool
	Error      string
}

type ValidationResult struct {
	TotalLinks    int
	ValidLinks    int
	BrokenLinks   int
	SkippedLinks  int
	LinksByFile   map[string][]Link
	BrokenByFile  map[string][]Link
	ExternalCache map[string]bool
	mu            sync.Mutex
}

var (
	docsPath      string
	timeout       time.Duration
	maxWorkers    int
	checkExternal bool
	verbose       bool

	// Regular expressions for link extraction
	markdownLinkRegex = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	hrefPropRegex     = regexp.MustCompile(`href=["']([^"']+)["']`)

	// Headers in MDX files that can be linked to
	headerRegex = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)

	// Patterns to ignore
	ignorePatterns = []string{
		"mailto:",
		"javascript:",
		"#",
		"{",
		"$",
	}
)

func main() {
	flag.StringVar(&docsPath, "path", "./docs/content/docs", "Path to the docs folder")
	flag.DurationVar(&timeout, "timeout", 10*time.Second, "Timeout for external link checks")
	flag.IntVar(&maxWorkers, "workers", 10, "Maximum concurrent workers for external link checking")
	flag.BoolVar(&checkExternal, "external", true, "Check external links")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.Parse()

	fmt.Println("🔍 Compozy Documentation Link Validator")
	fmt.Println("=====================================")
	fmt.Printf("📁 Docs path: %s\n", docsPath)
	fmt.Printf("⏱️  Timeout: %s\n", timeout)
	fmt.Printf("👷 Workers: %d\n", maxWorkers)
	fmt.Printf("🌐 Check external: %v\n\n", checkExternal)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	result := &ValidationResult{
		LinksByFile:   make(map[string][]Link),
		BrokenByFile:  make(map[string][]Link),
		ExternalCache: make(map[string]bool),
	}

	// Find all MDX files
	files, err := findMDXFiles(docsPath)
	if err != nil {
		fmt.Printf("❌ Error finding MDX files: %v\n", err)
		cancel()
		os.Exit(1)
	}

	fmt.Printf("📄 Found %d MDX files\n\n", len(files))

	// Extract and validate links from each file
	for i, file := range files {
		relPath, err := filepath.Rel(docsPath, file)
		if err != nil {
			relPath = file
		}
		fmt.Printf("[%d/%d] Processing %s\n", i+1, len(files), relPath)

		links, err := extractLinks(file, docsPath)
		if err != nil {
			fmt.Printf("  ⚠️  Error processing file: %v\n", err)
			continue
		}

		if len(links) > 0 {
			result.LinksByFile[file] = links
			result.TotalLinks += len(links)

			// Validate links
			validateLinks(ctx, links, docsPath, result)
		}
	}

	// Generate report
	generateReport(result, docsPath)
}

func findMDXFiles(root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".mdx") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func extractLinks(filePath, _ string) ([]Link, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var links []Link
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		// Skip lines that are in code blocks
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			continue
		}

		// Extract markdown links [text](url)
		matches := markdownLinkRegex.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				link := createLink(match[2], match[1], filePath, lineNum+1)
				if link != nil && !shouldIgnoreLink(link.URL) {
					links = append(links, *link)
				}
			}
		}

		// Extract href props href="url"
		hrefMatches := hrefPropRegex.FindAllStringSubmatch(line, -1)
		for _, match := range hrefMatches {
			if len(match) >= 2 {
				link := createLink(match[1], "", filePath, lineNum+1)
				if link != nil && !shouldIgnoreLink(link.URL) {
					links = append(links, *link)
				}
			}
		}
	}

	return links, nil
}

func createLink(url, text, sourceFile string, lineNumber int) *Link {
	link := &Link{
		URL:        strings.TrimSpace(url),
		Text:       text,
		SourceFile: sourceFile,
		LineNumber: lineNumber,
		Valid:      true,
	}

	// Determine link type
	switch {
	case strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://"):
		link.Type = LinkTypeExternal
	case strings.HasPrefix(url, "/"):
		link.Type = LinkTypeInternal
	case strings.Contains(url, "#"):
		link.Type = LinkTypeAnchor
	default:
		link.Type = LinkTypeFile
	}

	return link
}

func shouldIgnoreLink(url string) bool {
	for _, pattern := range ignorePatterns {
		if strings.HasPrefix(url, pattern) {
			return true
		}
	}
	return false
}

func validateLinks(ctx context.Context, links []Link, docsRoot string, result *ValidationResult) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxWorkers)

	for i := range links {
		link := &links[i]

		switch link.Type {
		case LinkTypeInternal:
			validateInternalLink(link, docsRoot)
		case LinkTypeExternal:
			if checkExternal {
				wg.Add(1)
				go func(l *Link) {
					defer wg.Done()
					semaphore <- struct{}{}
					defer func() { <-semaphore }()

					validateExternalLink(ctx, l, result)
				}(link)
			} else {
				link.Valid = true
				link.Error = "skipped"
			}
		case LinkTypeAnchor:
			validateAnchorLink(link, docsRoot)
		case LinkTypeFile:
			validateFileLink(link, docsRoot)
		}

		// Update counters
		result.mu.Lock()
		if link.Valid {
			result.ValidLinks++
		} else {
			result.BrokenLinks++
			if result.BrokenByFile[link.SourceFile] == nil {
				result.BrokenByFile[link.SourceFile] = []Link{}
			}
			result.BrokenByFile[link.SourceFile] = append(result.BrokenByFile[link.SourceFile], *link)
		}
		if link.Error == "skipped" {
			result.SkippedLinks++
		}
		result.mu.Unlock()
	}

	wg.Wait()
}

func validateInternalLink(link *Link, docsRoot string) {
	// Internal links should start with /docs/
	if !strings.HasPrefix(link.URL, "/docs/") {
		link.Valid = false
		link.Error = "internal link should start with /docs/"
		return
	}

	// Remove /docs/ prefix and any anchors
	docPath := strings.TrimPrefix(link.URL, "/docs/")
	docPath = strings.Split(docPath, "#")[0]

	// Special case for /docs/ root
	if docPath == "" {
		docPath = "index"
	}

	// Try to find corresponding MDX file
	// Remove /core prefix as it maps to the root content/docs directory
	docPath = strings.TrimPrefix(docPath, "core/")

	possiblePaths := []string{
		filepath.Join(docsRoot, docPath+".mdx"),
		filepath.Join(docsRoot, docPath, "index.mdx"),
		filepath.Join(docsRoot, "core", docPath+".mdx"),
		filepath.Join(docsRoot, "core", docPath, "index.mdx"),
	}

	found := false
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			found = true

			// If there's an anchor, validate it
			if strings.Contains(link.URL, "#") {
				parts := strings.Split(link.URL, "#")
				if len(parts) > 1 {
					anchor := parts[1]
					if !validateAnchorInFile(path, anchor) {
						link.Valid = false
						link.Error = fmt.Sprintf("anchor '#%s' not found in file", anchor)
						return
					}
				}
			}
			break
		}
	}

	if !found {
		link.Valid = false
		link.Error = fmt.Sprintf("file not found for path: %s", docPath)
	}
}

func validateExternalLink(ctx context.Context, link *Link, result *ValidationResult) {
	// Check cache first
	result.mu.Lock()
	if cached, ok := result.ExternalCache[link.URL]; ok {
		link.Valid = cached
		if !cached {
			link.Error = "failed (cached)"
		}
		result.mu.Unlock()
		return
	}
	result.mu.Unlock()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Try HEAD request first, fall back to GET
	methods := []string{"HEAD", "GET"}
	var lastErr error

	for _, method := range methods {
		req, err := http.NewRequestWithContext(ctx, method, link.URL, http.NoBody)
		if err != nil {
			lastErr = err
			continue
		}

		// Add user agent to avoid being blocked
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CompozyLinkValidator/1.0)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			link.Valid = true

			// Cache the result
			result.mu.Lock()
			result.ExternalCache[link.URL] = true
			result.mu.Unlock()

			return
		}

		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	link.Valid = false
	link.Error = fmt.Sprintf("failed: %v", lastErr)

	// Cache the negative result
	result.mu.Lock()
	result.ExternalCache[link.URL] = false
	result.mu.Unlock()
}

func validateAnchorLink(link *Link, docsRoot string) {
	// Handle pure anchors (#section) and paths with anchors
	if after, ok := strings.CutPrefix(link.URL, "#"); ok {
		// Pure anchor - check in the same file
		if !validateAnchorInFile(link.SourceFile, after) {
			link.Valid = false
			link.Error = fmt.Sprintf("anchor '%s' not found in current file", link.URL)
		}
	} else if strings.Contains(link.URL, "#") {
		// Path with anchor - validate as internal link
		link.Type = LinkTypeInternal
		validateInternalLink(link, docsRoot)
	}
}

func validateFileLink(link *Link, _ string) {
	// For relative file links
	basePath := filepath.Dir(link.SourceFile)
	linkPath := link.URL

	// Remove any anchors
	linkPath = strings.Split(linkPath, "#")[0]

	// Handle relative paths - try with .mdx extension
	possiblePaths := []string{
		filepath.Join(basePath, linkPath),
		filepath.Join(basePath, linkPath+".mdx"),
		filepath.Join(basePath, linkPath, "index.mdx"),
	}

	found := false
	var foundPath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			found = true
			foundPath = path

			// If there's an anchor, validate it
			if strings.Contains(link.URL, "#") {
				parts := strings.Split(link.URL, "#")
				if len(parts) > 1 {
					anchor := parts[1]
					if !validateAnchorInFile(foundPath, anchor) {
						link.Valid = false
						link.Error = fmt.Sprintf("anchor '#%s' not found in file", anchor)
						return
					}
				}
			}
			break
		}
	}

	if !found {
		link.Valid = false
		link.Error = fmt.Sprintf("file not found: %s", link.URL)
	}
}

func validateAnchorInFile(filePath, anchor string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	// Anchors in markdown are typically lowercase with hyphens

	// Find all headers
	headers := headerRegex.FindAllStringSubmatch(string(content), -1)
	for _, match := range headers {
		if len(match) > 1 {
			header := strings.TrimSpace(match[1])
			// Convert header to anchor format
			headerAnchor := strings.ToLower(header)
			headerAnchor = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(headerAnchor, "")
			headerAnchor = strings.ReplaceAll(headerAnchor, " ", "-")
			headerAnchor = regexp.MustCompile(`-+`).ReplaceAllString(headerAnchor, "-")

			if headerAnchor == anchor {
				return true
			}
		}
	}

	// Also check for explicit anchor definitions {#anchor-name}
	anchorDefRegex := regexp.MustCompile(`\{#([^}]+)\}`)
	anchorDefs := anchorDefRegex.FindAllStringSubmatch(string(content), -1)
	for _, match := range anchorDefs {
		if len(match) > 1 && match[1] == anchor {
			return true
		}
	}

	return false
}

func generateReport(result *ValidationResult, docsRoot string) {
	fmt.Println("\n📊 Validation Report")
	fmt.Println("==================")
	fmt.Printf("Total links:    %d\n", result.TotalLinks)
	fmt.Printf(
		"✅ Valid:       %d (%.1f%%)\n",
		result.ValidLinks,
		float64(result.ValidLinks)/float64(result.TotalLinks)*100,
	)
	fmt.Printf(
		"❌ Broken:      %d (%.1f%%)\n",
		result.BrokenLinks,
		float64(result.BrokenLinks)/float64(result.TotalLinks)*100,
	)
	if result.SkippedLinks > 0 {
		fmt.Printf("⏭️  Skipped:     %d\n", result.SkippedLinks)
	}

	if result.BrokenLinks > 0 {
		fmt.Println("\n❌ Broken Links by File")
		fmt.Println("======================")

		// Sort files for consistent output
		var files []string
		for file := range result.BrokenByFile {
			files = append(files, file)
		}
		sort.Strings(files)

		for _, file := range files {
			links := result.BrokenByFile[file]
			relPath, err := filepath.Rel(docsRoot, file)
			if err != nil {
				relPath = file
			}
			fmt.Printf("\n📄 %s (%d broken links)\n", relPath, len(links))

			for _, link := range links {
				fmt.Printf("  Line %d: %s\n", link.LineNumber, link.URL)
				if link.Text != "" {
					fmt.Printf("    Text: \"%s\"\n", link.Text)
				}
				fmt.Printf("    Type: %s\n", link.Type)
				fmt.Printf("    Error: %s\n", link.Error)
			}
		}
	}

	// Summary
	fmt.Println("\n📈 Summary")
	fmt.Println("=========")
	if result.BrokenLinks == 0 {
		fmt.Println("✅ All links are valid! 🎉")
	} else {
		fmt.Printf("❌ Found %d broken links that need to be fixed.\n", result.BrokenLinks)

		// Provide suggestions
		fmt.Println("\n💡 Common fixes:")
		fmt.Println("  - For internal links: ensure the target .mdx file exists")
		fmt.Println("  - For anchors: verify the heading exists and matches the anchor format")
		fmt.Println("  - For external links: check if the URL has changed or the site is down")
		fmt.Println("  - Remember: /docs/core/ paths map to content/docs/ directory")
	}
}
