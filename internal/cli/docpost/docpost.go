package docpost

import (
	"context"

	"errors"
	"fmt"
	"io/fs"
	"os"

	"path/filepath"
	"regexp"

	"strings"
)

const (
	docpostCompozyKey     = "compozy"
	docpostCompozyMDXPath = "compozy.mdx"
	docpostIndexKey       = "index"
	docpostIndexMDXPath   = "index.mdx"
	docpostMetaJSONPath   = "meta.json"
)

// linkBasePath is the URL prefix the site router mounts the CLI reference at.
// We rewrite inter-command links to use absolute paths under this prefix so
// they resolve the same regardless of which nested page they live on.
const linkBasePath = "/docs/cli"

// OutputProfile describes the output behavior of one Cobra command. Cobra
// propagates root flags to every command, including help-only groups, so the
// generated markdown alone cannot distinguish inherited flag acceptance from
// a command's renderer.
type OutputProfile string

const (
	// OutputProfileResult identifies a command that renders a command result.
	OutputProfileResult OutputProfile = "result"
	// OutputProfileHelp identifies a command that only renders Cobra help.
	OutputProfileHelp OutputProfile = "help"
	// OutputProfileNoOutput identifies a command with no result renderer.
	OutputProfileNoOutput OutputProfile = "none"
	// OutputProfileProtocol identifies a command whose standard output is a protocol stream.
	OutputProfileProtocol OutputProfile = "protocol"
	// OutputProfileRaw identifies a command that writes a command-specific text stream.
	OutputProfileRaw OutputProfile = "raw"
	// OutputProfileHumanJSONL identifies a stream that supports human and JSONL output only.
	OutputProfileHumanJSONL OutputProfile = "human-jsonl"
	// OutputProfileTerminalInteractive identifies an attached terminal byte stream.
	OutputProfileTerminalInteractive OutputProfile = "terminal-interactive"
	// OutputProfileTerminalOpen identifies a command that renders a result only when detached.
	OutputProfileTerminalOpen OutputProfile = "terminal-open"
	// OutputProfileTerminalQuote identifies a bounded terminal quote with human and JSON renderers.
	OutputProfileTerminalQuote OutputProfile = "terminal-quote"
)

// Options supplies command-tree facts that Cobra's Markdown generator does
// not retain in the generated files.
type Options struct {
	OutputProfiles map[string]OutputProfile
}

var (
	autoGenLine  = regexp.MustCompile(`(?m)^###### Auto generated.*$\n?`)
	seeAlsoRe    = regexp.MustCompile(`(?ms)^### SEE ALSO\n.*`)
	crossLinkRe  = regexp.MustCompile(`\[([^\]]+)\]\((compozy[A-Za-z0-9_\-]*)\.md\)`)
	strippedLink = regexp.MustCompile(`\]\((compozy[A-Za-z0-9_\-]*)\)`)
	segmentRe    = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
)

// Process reads all compozy*.md files from srcDir, transforms them into
// Fumadocs-compatible MDX, and writes them to dstDir using a nested
// directory layout: `compozy` → compozy.mdx, `compozy_agent` → agent/index.mdx,
// `compozy_agent_list` → agent/list.mdx, and so on.
//
// The root-level index.mdx and meta.json of dstDir are hand-maintained and
// never touched by Process. Subdirectory meta.json files are regenerated on
// each run.
// Stale files from prior runs are removed before writing.
func Process(ctx context.Context, srcDir, dstDir string, opts Options) error {
	if err := ensureContext(ctx, "start doc post-processing"); err != nil {
		return err
	}
	if err := prepareOutputDir(dstDir); err != nil {
		return err
	}

	inputs, err := readInputs(ctx, srcDir)
	if err != nil {
		return err
	}

	hasChildren := computeHasChildren(inputs)
	if err := validateOutputPaths(inputs, hasChildren); err != nil {
		return err
	}
	targets := buildTargetMap(inputs)
	if err := cleanOutput(ctx, dstDir); err != nil {
		return err
	}

	for _, in := range inputs {
		if err := ensureContext(ctx, fmt.Sprintf("write %s", in.fileName)); err != nil {
			return err
		}
		body := TransformMarkdown(in.raw, in.commandName())
		body = remapLinks(body, targets)
		body = enrichDocument(body, in, inputs, targets, outputProfileFor(in, opts))

		outRel := in.outputPath(hasChildren)
		dst := filepath.Join(dstDir, filepath.FromSlash(outRel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("docpost: mkdir %s: %w", dst, err)
		}
		if err := os.WriteFile(dst, []byte(body), 0o600); err != nil {
			return fmt.Errorf("docpost: write %s: %w", dst, err)
		}
	}

	return writeSubdirMetas(ctx, dstDir)
}

func outputProfileFor(in input, opts Options) OutputProfile {
	if opts.OutputProfiles == nil {
		return OutputProfileResult
	}
	if profile, ok := opts.OutputProfiles[in.commandName()]; ok {
		return profile
	}
	return OutputProfileResult
}

func prepareOutputDir(dstDir string) error {
	info, err := os.Stat(dstDir)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return fmt.Errorf("docpost: create output dir: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("docpost: stat output dir %s: %w", dstDir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("docpost: output path %s must be a directory", dstDir)
	}

	managed, err := isManagedOutputDir(dstDir)
	if err != nil {
		return err
	}
	if !managed {
		return fmt.Errorf(
			"docpost: refusing to clean non-empty unmanaged output dir %q",
			dstDir,
		)
	}

	return nil
}

func isManagedOutputDir(dstDir string) (bool, error) {
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		return false, fmt.Errorf("docpost: read output dir %s: %w", dstDir, err)
	}
	if len(entries) == 0 {
		return true, nil
	}

	hasEditorialIndex := false
	hasEditorialMeta := false
	hasGeneratedRoot := false

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		switch entry.Name() {
		case docpostIndexMDXPath:
			hasEditorialIndex = true
		case docpostMetaJSONPath:
			hasEditorialMeta = true
		case docpostCompozyMDXPath:
			hasGeneratedRoot = true
		default:
			if strings.HasSuffix(entry.Name(), ".mdx") {
				continue
			}
			return false, nil
		}
	}

	return hasGeneratedRoot || (hasEditorialIndex && hasEditorialMeta), nil
}

type input struct {
	fileName string
	baseName string
	segments []string
	raw      string
}

func (in input) isRoot() bool {
	return len(in.segments) == 0
}

func (in input) commandName() string {
	return baseNameToCommand(in.baseName)
}

func (in input) targetURL() string {
	if in.isRoot() {
		return linkBasePath + "/compozy"
	}
	return linkBasePath + "/" + strings.Join(in.segments, "/")
}

func (in input) outputPath(hasChildren map[string]bool) string {
	return outPath(in, hasChildren)
}

func readInputs(ctx context.Context, srcDir string) ([]input, error) {
	if err := ensureContext(ctx, fmt.Sprintf("read source dir %s", srcDir)); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("docpost: read source dir %s: %w", srcDir, err)
	}

	var inputs []input
	for _, entry := range entries {
		in, ok, err := readInput(ctx, srcDir, entry)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		inputs = append(inputs, in)
	}
	return inputs, nil
}

func readInput(ctx context.Context, srcDir string, entry fs.DirEntry) (input, bool, error) {
	fullPath := filepath.Join(srcDir, entry.Name())
	if err := ensureContext(ctx, fmt.Sprintf("read %s", fullPath)); err != nil {
		return input{}, false, err
	}
	if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
		return input{}, false, nil
	}
	base := strings.TrimSuffix(entry.Name(), ".md")
	segments, err := commandSegments(entry.Name(), base)
	if err != nil {
		return input{}, false, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return input{}, false, fmt.Errorf("docpost: read %s: %w", fullPath, err)
	}
	return input{
		fileName: entry.Name(),
		baseName: base,
		segments: segments,
		raw:      string(data),
	}, true, nil
}

func commandSegments(fileName string, base string) ([]string, error) {
	if base == docpostCompozyKey {
		return nil, nil
	}
	if !strings.HasPrefix(base, "compozy_") {
		return nil, fmt.Errorf(
			"docpost: unexpected filename %q (must be 'compozy.md' or start with 'compozy_')",
			fileName,
		)
	}
	segments := strings.Split(base, "_")[1:]
	for _, segment := range segments {
		if !segmentRe.MatchString(segment) {
			return nil, fmt.Errorf("docpost: unexpected filename %q (invalid command segment %q)", fileName, segment)
		}
	}
	return segments, nil
}
