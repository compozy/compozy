package opendesign

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/fileutil"
	compozysdk "github.com/compozy/compozy/sdk/go"
)

const (
	maxArtifacts     = 32
	maxArtifactBytes = 1 << 20
	maxInputBytes    = 8 << 20
	maxResultBytes   = 1 << 20
	lintTimeout      = 30 * time.Second
)

//go:embed lint.gen.mjs
var lintProgram string

type lintInput struct {
	Paths []string `json:"paths"`
}
type lintDocument struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	HTML   string `json:"html"`
}

func handleLint(ctx context.Context, req compozysdk.ToolRequest[lintInput]) (compozysdk.ToolResult, error) {
	started := time.Now()
	documents, err := readArtifacts(ctx, req.TrustedWorkspace, req.Input.Paths)
	if err != nil {
		if ctx.Err() != nil {
			return compozysdk.ToolResult{}, ctx.Err()
		}
		return compozysdk.ToolResult{}, compozysdk.NewInvalidParamsError(err.Error(), nil)
	}
	encoded, err := json.Marshal(documents)
	if err != nil {
		return compozysdk.ToolResult{}, fmt.Errorf("open-design: encode lint input: %w", err)
	}
	result, err := executeLint(ctx, encoded)
	if err != nil {
		return compozysdk.ToolResult{}, err
	}
	return compozysdk.ToolResult{
		Structured: result,
		Bytes:      int64(len(result)),
		DurationMS: time.Since(started).Milliseconds(),
		Preview:    fmt.Sprintf("Checked %d HTML artifact(s) with the original OpenDesign linter.", len(documents)),
	}, nil
}

func readArtifacts(
	ctx context.Context,
	scope *compozysdk.ExtensionToolWorkspaceScope,
	paths []string,
) (documents []lintDocument, err error) {
	if scope == nil || scope.ID == "" || !filepath.IsAbs(scope.Root) {
		return nil, errors.New("open-design: a trusted workspace is required")
	}
	if len(paths) == 0 || len(paths) > maxArtifacts {
		return nil, fmt.Errorf("open-design: paths must contain 1 to %d HTML files", maxArtifacts)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Resolve only the daemon-authenticated root. Every artifact component is
	// subsequently opened relative to held directory handles without symlinks.
	canonicalRoot, err := filepath.EvalSymlinks(scope.Root)
	if err != nil {
		return nil, fmt.Errorf("open-design: resolve workspace: %w", err)
	}
	root, err := fileutil.OpenDirectory(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("open-design: open workspace: %w", err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	docs, err := root.OpenDirectory("docs")
	if err != nil {
		return nil, fmt.Errorf("open-design: open workspace docs: %w", err)
	}
	defer func() { err = errors.Join(err, docs.Close()) }()
	designs, err := docs.OpenDirectory("design")
	if err != nil {
		return nil, fmt.Errorf("open-design: open workspace docs/design: %w", err)
	}
	defer func() { err = errors.Join(err, designs.Close()) }()
	documents = make([]lintDocument, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	total := 0
	for _, name := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if strings.Contains(name, "\\") || path.Clean(name) != name || !strings.HasPrefix(name, "docs/design/") ||
			strings.ToLower(path.Ext(name)) != ".html" {
			return nil, fmt.Errorf(
				"open-design: %q must be a normalized workspace-relative HTML path under docs/design/",
				name,
			)
		}
		if seen[name] {
			return nil, fmt.Errorf("open-design: duplicate artifact %q", name)
		}
		seen[name] = true
		content, err := readHTML(designs, strings.TrimPrefix(name, "docs/design/"))
		if err != nil {
			return nil, fmt.Errorf("open-design: read %s: %w", name, err)
		}
		total += len(content)
		if total > maxInputBytes {
			return nil, fmt.Errorf("open-design: combined HTML exceeds %d bytes", maxInputBytes)
		}
		digest := sha256.Sum256(content)
		documents = append(
			documents,
			lintDocument{Path: name, SHA256: hex.EncodeToString(digest[:]), HTML: string(content)},
		)
	}
	return documents, nil
}

func readHTML(root *fileutil.Directory, name string) (content []byte, err error) {
	if parent, child, nested := strings.Cut(name, "/"); nested {
		var directory *fileutil.Directory
		directory, err = root.OpenDirectory(parent)
		if err != nil {
			return nil, err
		}
		defer func() { err = errors.Join(err, directory.Close()) }()
		return readHTML(directory, child)
	}
	file, err := root.OpenRegularFile(name)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	content, err = io.ReadAll(io.LimitReader(file, maxArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxArtifactBytes {
		return nil, fmt.Errorf("HTML exceeds %d bytes", maxArtifactBytes)
	}
	if len(bytes.TrimSpace(content)) == 0 || !utf8.Valid(content) {
		return nil, errors.New("artifact must contain nonempty UTF-8 HTML")
	}
	return content, nil
}

func executeLint(ctx context.Context, input []byte) (json.RawMessage, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf(
			"open-design: Node.js is required to run the original linter; install Node.js and restart the extension: %w",
			err,
		)
	}
	runCtx, cancel := context.WithTimeout(ctx, lintTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, node, "--input-type=module", "-e", lintProgram, "--", "--stdio")
	// Node preload settings must not inject code into the embedded linter.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "NODE_OPTIONS") && !strings.EqualFold(key, "NODE_PATH") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Stdin = bytes.NewReader(input)
	command.WaitDelay = time.Second
	output := &limitedBuffer{remaining: maxResultBytes}
	command.Stdout = output
	stderr := &limitedBuffer{remaining: 4096}
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if runCtx.Err() != nil {
			return nil, fmt.Errorf("open-design: lint interrupted: %w", runCtx.Err())
		}
		return nil, fmt.Errorf("open-design: linter failed: %w: %s", err, stderr.String())
	}
	if !json.Valid(output.Bytes()) {
		return nil, errors.New("open-design: linter returned invalid JSON")
	}
	return json.RawMessage(output.Bytes()), nil
}

type limitedBuffer struct {
	bytes.Buffer
	remaining int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.remaining {
		return 0, errors.New("open-design: linter output limit exceeded")
	}
	n, err := b.Buffer.Write(p)
	b.remaining -= n
	return n, err
}
