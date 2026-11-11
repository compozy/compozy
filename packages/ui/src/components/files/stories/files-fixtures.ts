// Story fixtures for the Pierre-backed file primitives. Data boundaries only:
// listings, file text, and patches stand in for the daemon's file routes.

export const SAMPLE_GO = `package projectfiles

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// ErrOutsideRoot reports a path that escapes the browsable root.
var ErrOutsideRoot = errors.New("projectfiles: path is outside the root")

// Entry is one row of a directory listing.
type Entry struct {
	Name      string
	Path      string
	Kind      string
	Size      int64
	Ignored   bool
	Sensitive bool
}

// Reader lists and reads files confined to a root.
type Reader struct {
	root  string
	limit int
}

// NewReader returns a Reader for root that lists at most limit entries.
func NewReader(root string, limit int) *Reader {
	return &Reader{root: filepath.Clean(root), limit: limit}
}

// List returns the entries of dir, directories first.
func (r *Reader) List(ctx context.Context, dir string) ([]Entry, bool, error) {
	abs, err := r.resolve(dir)
	if err != nil {
		return nil, false, err
	}
	var entries []Entry
	walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == abs {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		entries = append(entries, Entry{Name: d.Name(), Path: path})
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	truncated := len(entries) > r.limit
	if truncated {
		entries = entries[:r.limit]
	}
	return entries, truncated, walkErr
}

func (r *Reader) resolve(rel string) (string, error) {
	abs := filepath.Join(r.root, rel)
	if abs != r.root && !strings.HasPrefix(abs, r.root+string(filepath.Separator)) {
		return "", ErrOutsideRoot
	}
	return abs, nil
}
`;

/** A large file: `text` repeated, big enough to highlight in the worker pool. */
export function longText(text: string, copies: number): string {
  return Array.from({ length: copies }, () => text).join("\n");
}

export const ROOT_LISTING = [
  "cmd/",
  "internal/",
  "web/",
  "node_modules/",
  ".env",
  "go.mod",
  "go.sum",
  "README.md",
];

export const CHILD_LISTINGS: Record<string, string[]> = {
  "cmd/": ["cmd/compozy/"],
  "cmd/compozy/": ["cmd/compozy/main.go"],
  "internal/": ["internal/projectfiles/", "internal/api/"],
  "internal/projectfiles/": [
    "internal/projectfiles/reader.go",
    "internal/projectfiles/roots.go",
    "internal/projectfiles/status.go",
  ],
  "internal/api/": ["internal/api/files.go"],
  "web/": ["web/package.json", "web/src/"],
  "web/src/": ["web/src/main.tsx"],
  "node_modules/": [],
};

export const CHANGED_PATHS = [
  "internal/",
  "internal/projectfiles/",
  "internal/projectfiles/reader.go",
  "internal/projectfiles/roots.go",
  "internal/api/",
  "internal/api/files.go",
  "web/",
  "web/src/",
  "web/src/main.tsx",
  "docs/",
  "docs/old.md",
];

/**
 * The deep, long-named change set from #1480: review attachments six to eight levels down,
 * sibling directories that share a long prefix, and a few shallow files. Directories are listed
 * before their children, as the Changes panel sends them.
 */
export const DEEP_CHANGED_PATHS: string[] = (() => {
  const review = "packages/frontend/proto/review/";
  const screens = [
    "admin-billing-customer-detail",
    "admin-billing-customer-invoices",
    "admin-billing-subscription-cancellation-notice",
  ];
  const attachments = [
    "1a7f3c2e-9b41-4d0a-8e55-0c6f2b9d7a13.png",
    "6b2e9d40-17c8-4f3a-b0d2-5e8a1c4f9b67.png",
    "c94d1e8a-3f27-4b6c-9a10-7d5e2f8b0c31.png",
  ];
  const paths = ["packages/", "packages/frontend/", "packages/frontend/proto/", review];
  for (const screen of screens) {
    const dir = `${review}${screen}/`;
    paths.push(dir, `${dir}${screen}.proto.tsx`, `${dir}admin-billing-fixtures.ts`);
    paths.push(`${dir}attachments/`, ...attachments.map(name => `${dir}attachments/${name}`));
  }
  paths.push("packages/frontend/src/", "packages/frontend/src/index.ts", "README.md");
  return paths;
})();

export const OLD_READER = SAMPLE_GO.replace("\ttruncated := len(entries) > r.limit\n", "").replace(
  "Ignored   bool\n\tSensitive bool\n",
  "Ignored   bool\n"
);

function unifiedPatch(path: string, hunk: string): string {
  return `diff --git a/${path} b/${path}\nindex 1a2b3c4..5d6e7f8 100644\n--- a/${path}\n+++ b/${path}\n${hunk}`;
}

export const PATCHES: Record<string, string> = {
  "internal/projectfiles/reader.go": unifiedPatch(
    "internal/projectfiles/reader.go",
    "@@ -19,5 +19,6 @@ type Entry struct {\n \tSize      int64\n \tIgnored   bool\n+\tSensitive bool\n }\n \n // Reader lists and reads files confined to a root.\n@@ -59,6 +60,7 @@ func (r *Reader) List(ctx context.Context, dir string) ([]Entry, bool, error) {\n \t\t}\n \t\treturn nil\n \t})\n+\ttruncated := len(entries) > r.limit\n \tif truncated {\n \t\tentries = entries[:r.limit]\n \t}\n"
  ),
  "internal/projectfiles/roots.go": unifiedPatch(
    "internal/projectfiles/roots.go",
    "@@ -1,5 +1,7 @@\n package projectfiles\n \n-// Root is a browsable directory.\n+// Root is a browsable directory: the project checkout, a worktree, or a\n+// session working directory.\n type Root struct {\n+\tKind string\n \tPath string\n"
  ),
  "web/src/main.tsx": unifiedPatch(
    "web/src/main.tsx",
    '@@ -1,3 +1,3 @@\n-import { App } from "./app";\n+import { App } from "./app/app";\n \n createRoot(document.getElementById("root")!).render(<App />);\n'
  ),
};

// A 24×24 checker PNG and a small SVG whose <script> must never execute.
export const PNG_BASE64 =
  "iVBORw0KGgoAAAANSUhEUgAAABgAAAAYCAIAAABvFaqvAAAAMklEQVR4nGNQwwAvwrXQEDFqGEYNoqNB5GnDVDNqED0NIk/bkEiQw9igwZeORg0iqAYA2EMWf0r4Y2EAAAAASUVORK5CYII=";

export const SVG_WITH_SCRIPT =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><script>document.title="pwned"</script><circle cx="32" cy="32" r="24" fill="#2f5bff"/></svg>';
