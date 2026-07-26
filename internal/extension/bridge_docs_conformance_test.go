package extensionpkg_test

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionpkg "github.com/compozy/agh/internal/extension"
)

type documentedProviderSlots struct {
	Required []string
	Optional []string
}

type bridgeDocProvider struct {
	Name     string
	Manifest *extensionpkg.Manifest
}

func TestBridgeProviderDocsConformance(t *testing.T) {
	t.Parallel()

	repoRoot := bridgeDocsRepoRoot(t)
	providers := bridgeDocProviders(t, filepath.Join(repoRoot, "extensions", "bridges"))
	indexPath := filepath.Join(repoRoot, "packages", "site", "content", "runtime", "core", "bridges", "index.mdx")
	setupPath := filepath.Join(repoRoot, "packages", "site", "content", "runtime", "core", "bridges", "setup.mdx")
	slackSetupPath := filepath.Join(
		repoRoot,
		"packages",
		"site",
		"content",
		"runtime",
		"core",
		"bridges",
		"setup-slack.mdx",
	)
	index := readBridgeDoc(t, indexPath)
	slackSetup := readBridgeDoc(t, slackSetupPath)

	t.Run("Should cover every discovered provider in the slots table", func(t *testing.T) {
		t.Parallel()
		rows := parseProviderSlotsTable(index)
		for _, provider := range providers {
			row, ok := rows[provider.Manifest.Bridge.DisplayName]
			if !ok {
				t.Errorf(
					"provider %q is missing from packages/site/content/runtime/core/bridges/index.mdx supported slots table",
					provider.Name,
				)
				continue
			}
			wantRequired, wantOptional := manifestSlotSets(provider.Manifest)
			if !sameSortedStrings(row.Required, wantRequired) || !sameSortedStrings(row.Optional, wantOptional) {
				t.Errorf(
					"provider %q documented slots required=%v optional=%v, want required=%v optional=%v",
					provider.Name,
					row.Required,
					row.Optional,
					wantRequired,
					wantOptional,
				)
			}
		}
	})

	t.Run("Should keep the documented Slack scopes equal to the runtime contract", func(t *testing.T) {
		t.Parallel()
		got := parseBacktickBulletListAfter(slackSetup, "It requests these bot scopes")
		want := bridgepkg.SlackBotScopes()
		if !sameSortedStrings(got, want) {
			t.Fatalf("documented Slack scopes = %v, want runtime scopes %v", got, want)
		}
	})

	t.Run("Should provide a dedicated setup guide for every discovered provider", func(t *testing.T) {
		t.Parallel()
		docsDir := filepath.Dir(setupPath)
		for _, provider := range providers {
			providerPage := filepath.Join(docsDir, "setup-"+provider.Name+".mdx")
			if _, err := os.Stat(providerPage); err == nil {
				continue
			}
			t.Errorf(
				"provider %q is missing its dedicated setup guide under packages/site/content/runtime/core/bridges",
				provider.Name,
			)
		}
	})
}

func bridgeDocProviders(t *testing.T, root string) []bridgeDocProvider {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("os.ReadDir(provider root) error = %v", err)
	}
	providers := make([]bridgeDocProvider, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "extension.toml")); err != nil {
			continue
		}
		manifest, err := extensionpkg.LoadManifest(dir)
		if err != nil {
			t.Fatalf("LoadManifest(%q) error = %v", entry.Name(), err)
		}
		providers = append(providers, bridgeDocProvider{Name: entry.Name(), Manifest: manifest})
	}
	sort.Slice(providers, func(left int, right int) bool { return providers[left].Name < providers[right].Name })
	return providers
}

func readBridgeDoc(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return string(content)
}

func parseProviderSlotsTable(document string) map[string]documentedProviderSlots {
	rows := make(map[string]documentedProviderSlots)
	inSection := false
	scanner := bufio.NewScanner(strings.NewReader(document))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "## Supported provider slots" {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "## ") {
			break
		}
		if !inSection || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := markdownTableCells(line)
		if len(cells) < 3 || cells[0] == "Provider" || strings.Trim(cells[0], "-: ") == "" {
			continue
		}
		rows[cells[0]] = documentedProviderSlots{
			Required: parseBacktickValues(cells[1]),
			Optional: parseBacktickValues(cells[2]),
		}
	}
	return rows
}

func markdownTableCells(line string) []string {
	trimmed := strings.Trim(strings.TrimSpace(line), "|")
	parts := strings.Split(trimmed, "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func parseBacktickValues(value string) []string {
	if strings.EqualFold(strings.TrimSpace(value), "none") {
		return nil
	}
	parts := strings.Split(value, "`")
	values := make([]string, 0, len(parts)/2)
	for index := 1; index < len(parts); index += 2 {
		if slot := strings.TrimSpace(parts[index]); slot != "" {
			values = append(values, slot)
		}
	}
	sort.Strings(values)
	return values
}

func manifestSlotSets(manifest *extensionpkg.Manifest) ([]string, []string) {
	required := make([]string, 0, len(manifest.Bridge.SecretSlots))
	optional := make([]string, 0, len(manifest.Bridge.SecretSlots))
	for _, slot := range manifest.Bridge.SecretSlots {
		if slot.Required {
			required = append(required, strings.TrimSpace(slot.Name))
		} else {
			optional = append(optional, strings.TrimSpace(slot.Name))
		}
	}
	sort.Strings(required)
	sort.Strings(optional)
	return required, optional
}

func parseBacktickBulletListAfter(document string, marker string) []string {
	_, afterMarker, found := strings.Cut(document, marker)
	if !found {
		return nil
	}
	values := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(afterMarker))
	started := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "- `") && strings.HasSuffix(line, "`") {
			started = true
			values = append(values, strings.Trim(line[2:], "` "))
			continue
		}
		if started && line != "" {
			break
		}
	}
	sort.Strings(values)
	return values
}

func sameSortedStrings(left []string, right []string) bool {
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func bridgeDocsRepoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}
