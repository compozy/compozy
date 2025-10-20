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
	parseFlags()
	printRunConfiguration()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result := newValidationResult()
	files := discoverDocumentationFiles(cancel)
	fmt.Printf("📄 Found %d MDX files\n\n", len(files))
	processDocumentation(ctx, files, result)
	generateReport(result, docsPath)
}

// parseFlags registers and parses CLI flags for the validator.
func parseFlags() {
	flag.StringVar(&docsPath, "path", "./docs/content/docs", "Path to the docs folder")
	flag.DurationVar(&timeout, "timeout", 10*time.Second, "Timeout for external link checks")
	flag.IntVar(&maxWorkers, "workers", 10, "Maximum concurrent workers for external link checking")
	flag.BoolVar(&checkExternal, "external", true, "Check external links")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.Parse()
}

// printRunConfiguration outputs the current execution settings to stdout.
func printRunConfiguration() {
	fmt.Println("🔍 Compozy Documentation Link Validator")
	fmt.Println("=====================================")
	fmt.Printf("📁 Docs path: %s\n", docsPath)
	fmt.Printf("⏱️  Timeout: %s\n", timeout)
	fmt.Printf("👷 Workers: %d\n", maxWorkers)
	fmt.Printf("🌐 Check external: %v\n\n", checkExternal)
}

// newValidationResult constructs a zero-initialized validation accumulator.
func newValidationResult() *ValidationResult {
	return &ValidationResult{
		LinksByFile:   make(map[string][]Link),
		BrokenByFile:  make(map[string][]Link),
		ExternalCache: make(map[string]bool),
	}
}

// discoverDocumentationFiles enumerates MDX files or exits on error.
func discoverDocumentationFiles(cancel context.CancelFunc) []string {
	files, err := findMDXFiles(docsPath)
	if err != nil {
		fmt.Printf("❌ Error finding MDX files: %v\n", err)
		cancel()
		os.Exit(1)
	}
	return files
}

// processDocumentation extracts and validates links across all MDX files.
func processDocumentation(ctx context.Context, files []string, result *ValidationResult) {
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

		if len(links) == 0 {
			continue
		}

		result.LinksByFile[file] = links
		result.TotalLinks += len(links)
		validateLinks(ctx, links, docsPath, result)
	}
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
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			continue
		}

		matches := markdownLinkRegex.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				link := createLink(match[2], match[1], filePath, lineNum+1)
				if link != nil && !shouldIgnoreLink(link.URL) {
					links = append(links, *link)
				}
			}
		}

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
	if !strings.HasPrefix(link.URL, "/docs/") {
		link.Valid = false
		link.Error = "internal link should start with /docs/"
		return
	}
	docPath := normalizeInternalDocPath(link.URL)
	targetPath, found := resolveInternalTarget(docsRoot, docPath)
	if !found {
		link.Valid = false
		link.Error = fmt.Sprintf("file not found for path: %s", docPath)
		return
	}
	if !validateAnchorIfPresent(link, targetPath) {
		return
	}
	link.Valid = true
}

// normalizeInternalDocPath trims prefixes and anchors from an internal link.
func normalizeInternalDocPath(url string) string {
	docPath := strings.TrimPrefix(url, "/docs/")
	docPath = strings.Split(docPath, "#")[0]
	if docPath == "" {
		docPath = "index"
	}
	return strings.TrimPrefix(docPath, "core/")
}

// resolveInternalTarget returns the first matching file path for an internal link.
func resolveInternalTarget(docsRoot, docPath string) (string, bool) {
	candidates := []string{
		filepath.Join(docsRoot, docPath+".mdx"),
		filepath.Join(docsRoot, docPath, "index.mdx"),
		filepath.Join(docsRoot, "core", docPath+".mdx"),
		filepath.Join(docsRoot, "core", docPath, "index.mdx"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

// validateAnchorIfPresent checks the anchor portion of a link when provided.
func validateAnchorIfPresent(link *Link, path string) bool {
	if !strings.Contains(link.URL, "#") {
		return true
	}
	parts := strings.Split(link.URL, "#")
	if len(parts) <= 1 || parts[1] == "" {
		return true
	}
	anchor := parts[1]
	if validateAnchorInFile(path, anchor) {
		return true
	}
	link.Valid = false
	link.Error = fmt.Sprintf("anchor '#%s' not found in file", anchor)
	return false
}

func validateExternalLink(ctx context.Context, link *Link, result *ValidationResult) {
	if cached, ok := lookupExternalCache(result, link); ok {
		applyCachedExternalResult(link, cached)
		return
	}
	client := newExternalHTTPClient()
	success, lastErr := probeExternalLink(ctx, client, link.URL)
	finalizeExternalValidation(result, link, success, lastErr)
}

// lookupExternalCache retrieves cached external validation outcomes.
func lookupExternalCache(result *ValidationResult, link *Link) (bool, bool) {
	result.mu.Lock()
	defer result.mu.Unlock()
	cached, ok := result.ExternalCache[link.URL]
	return cached, ok
}

// applyCachedExternalResult updates the link using cached validation state.
func applyCachedExternalResult(link *Link, valid bool) {
	link.Valid = valid
	if !valid {
		link.Error = "failed (cached)"
	}
}

// newExternalHTTPClient constructs an HTTP client respecting timeout and redirects.
func newExternalHTTPClient() *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

// probeExternalLink attempts to reach the URL using HEAD/GET semantics.
func probeExternalLink(ctx context.Context, client *http.Client, url string) (bool, error) {
	methods := []string{"HEAD", "GET"}
	var lastErr error
	for _, method := range methods {
		req, err := http.NewRequestWithContext(ctx, method, url, http.NoBody)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CompozyLinkValidator/1.0)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return true, nil
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return false, lastErr
}

// finalizeExternalValidation records the result and updates the shared cache.
func finalizeExternalValidation(result *ValidationResult, link *Link, success bool, lastErr error) {
	link.Valid = success
	if !success {
		link.Error = fmt.Sprintf("failed: %v", lastErr)
	}
	result.mu.Lock()
	result.ExternalCache[link.URL] = success
	result.mu.Unlock()
}

func validateAnchorLink(link *Link, docsRoot string) {
	if after, ok := strings.CutPrefix(link.URL, "#"); ok {
		if !validateAnchorInFile(link.SourceFile, after) {
			link.Valid = false
			link.Error = fmt.Sprintf("anchor '%s' not found in current file", link.URL)
		}
	} else if strings.Contains(link.URL, "#") {
		link.Type = LinkTypeInternal
		validateInternalLink(link, docsRoot)
	}
}

func validateFileLink(link *Link, _ string) {
	basePath := filepath.Dir(link.SourceFile)
	linkPath := link.URL
	linkPath = strings.Split(linkPath, "#")[0]
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
	headers := headerRegex.FindAllStringSubmatch(string(content), -1)
	for _, match := range headers {
		if len(match) > 1 {
			header := strings.TrimSpace(match[1])
			headerAnchor := strings.ToLower(header)
			headerAnchor = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(headerAnchor, "")
			headerAnchor = strings.ReplaceAll(headerAnchor, " ", "-")
			headerAnchor = regexp.MustCompile(`-+`).ReplaceAllString(headerAnchor, "-")

			if headerAnchor == anchor {
				return true
			}
		}
	}
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
	printReportOverview(result)
	printBrokenLinkDetails(result, docsRoot)
	printReportSummary(result)
}

// printReportOverview displays aggregate totals for the validation run.
func printReportOverview(result *ValidationResult) {
	fmt.Println("\n📊 Validation Report")
	fmt.Println("==================")
	fmt.Printf("Total links:    %d\n", result.TotalLinks)
	fmt.Printf("✅ Valid:       %d (%.1f%%)\n", result.ValidLinks, percentage(result.ValidLinks, result.TotalLinks))
	fmt.Printf("❌ Broken:      %d (%.1f%%)\n", result.BrokenLinks, percentage(result.BrokenLinks, result.TotalLinks))
	if result.SkippedLinks > 0 {
		fmt.Printf("⏭️  Skipped:     %d\n", result.SkippedLinks)
	}
}

// printBrokenLinkDetails lists broken links grouped by file when present.
func printBrokenLinkDetails(result *ValidationResult, docsRoot string) {
	if result.BrokenLinks == 0 {
		return
	}
	fmt.Println("\n❌ Broken Links by File")
	fmt.Println("======================")
	files := sortedBrokenFiles(result)
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

// sortedBrokenFiles returns a deterministic ordering of files with broken links.
func sortedBrokenFiles(result *ValidationResult) []string {
	files := make([]string, 0, len(result.BrokenByFile))
	for file := range result.BrokenByFile {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

// printReportSummary closes the report with actionable guidance.
func printReportSummary(result *ValidationResult) {
	fmt.Println("\n📈 Summary")
	fmt.Println("=========")
	if result.BrokenLinks == 0 {
		fmt.Println("✅ All links are valid! 🎉")
		return
	}
	fmt.Printf("❌ Found %d broken links that need to be fixed.\n", result.BrokenLinks)
	fmt.Println("\n💡 Common fixes:")
	fmt.Println("  - For internal links: ensure the target .mdx file exists")
	fmt.Println("  - For anchors: verify the heading exists and matches the anchor format")
	fmt.Println("  - For external links: check if the URL has changed or the site is down")
	fmt.Println("  - Remember: /docs/core/ paths map to content/docs/ directory")
}

// percentage safely computes percentages for reporting.
func percentage(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
