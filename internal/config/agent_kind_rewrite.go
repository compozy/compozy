package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/frontmatter"
)

// AgentKindRewriteSkip names why the retired-kind migration left an AGENT.md file untouched.
type AgentKindRewriteSkip string

const (
	AgentKindRewriteSkipSymlink       AgentKindRewriteSkip = "symlink"
	AgentKindRewriteSkipReadOnly      AgentKindRewriteSkip = "read_only"
	AgentKindRewriteSkipChanged       AgentKindRewriteSkip = "changed_during_rewrite"
	AgentKindRewriteSkipUnsupported   AgentKindRewriteSkip = "unsupported_form"
	AgentKindRewriteSkipIO            AgentKindRewriteSkip = "io"
	agentKindRewriteFile                                   = "AGENT.md"
	agentKindRewriteDelimiter                              = "---"
	agentKindRewriteOwnerWritableMask                      = 0o200
)

// ErrAgentFileChanged reports that AGENT.md changed between the rewrite's read and its final
// compare, so the rewrite was not published.
var ErrAgentFileChanged = errors.New("config: agent file changed during rewrite")

// Top-level frontmatter kind lines the migration edits in place. The value group spans the retired
// value with its optional quotes; anything else on a top-level kind line is an unsupported form.
var (
	agentKindYAMLLine     = regexp.MustCompile(`^kind:[ \t]+("stays_on"|'stays_on'|stays_on)([ \t]+#.*)?[ \t]*$`)
	agentKindTOMLLine     = regexp.MustCompile(`^kind[ \t]*=[ \t]*("stays_on"|'stays_on')([ \t]+#.*)?[ \t]*$`)
	agentKindTopLevelLine = regexp.MustCompile(`^kind[ \t]*[:=]`)
)

// agentKindRewriteHooks are test seams around the publish precondition.
type agentKindRewriteHooks struct {
	beforeCompare func(path string)
}

// MigrateRetiredAgentKinds is the user-state migration for the v0.5.0 kind spelling (#1239, SD-013):
// it rewrites the top-level frontmatter kind value stays_on → always_on in <root>/<agent>/AGENT.md
// for each direct child of root, so strict loading never meets it. It never follows a symlink,
// edits only the value's bytes, keeps the file mode, and publishes only if the file is unchanged at
// the final compare. Files it cannot edit safely are logged as skips and keep failing to load with
// an error that names always_on. err reports only a failure to enumerate root.
func MigrateRetiredAgentKinds(
	root string,
	log *slog.Logger,
) (rewritten []string, skipped map[string]AgentKindRewriteSkip, err error) {
	return rewriteAgentKindAliases(root, log, agentKindRewriteHooks{})
}

func rewriteAgentKindAliases(
	root string,
	log *slog.Logger,
	hooks agentKindRewriteHooks,
) (rewritten []string, skipped map[string]AgentKindRewriteSkip, err error) {
	if log == nil {
		log = slog.Default()
	}
	pass := &agentKindRewritePass{log: log, hooks: hooks, skipped: map[string]AgentKindRewriteSkip{}}
	directory, err := fileutil.OpenDirectory(root)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil, nil, nil
		case isSymlinkPath(root, err):
			pass.skip(root, AgentKindRewriteSkipSymlink)
			return nil, pass.skipped, nil
		}
		return nil, nil, fmt.Errorf("config: open agent root %q: %w", root, err)
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	names, err := directory.ReadDir()
	if err != nil {
		return nil, nil, fmt.Errorf("config: list agent root %q: %w", root, err)
	}
	slices.Sort(names)
	for _, name := range names {
		pass.rewriteChild(directory, root, name)
	}
	return pass.rewritten, pass.skipped, nil
}

// MigrateRetiredAgentKind runs the same migration for one agent directory, <agentsDir>/<name>, and
// reports whether its AGENT.md was rewritten. Loaders call it after a file fails to load, so files
// that already load pay nothing. err reports only a failure to open agentsDir.
func MigrateRetiredAgentKind(agentsDir string, name string, log *slog.Logger) (migrated bool, err error) {
	if log == nil {
		log = slog.Default()
	}
	pass := &agentKindRewritePass{log: log, skipped: map[string]AgentKindRewriteSkip{}}
	directory, err := fileutil.OpenDirectory(agentsDir)
	if err != nil {
		if isSymlinkPath(agentsDir, err) {
			pass.skip(agentsDir, AgentKindRewriteSkipSymlink)
			return false, nil
		}
		return false, fmt.Errorf("config: open agent root %q: %w", agentsDir, err)
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	pass.rewriteChild(directory, agentsDir, name)
	return len(pass.rewritten) > 0, nil
}

type agentKindRewritePass struct {
	log       *slog.Logger
	hooks     agentKindRewriteHooks
	rewritten []string
	skipped   map[string]AgentKindRewriteSkip
}

func (p *agentKindRewritePass) skip(path string, reason AgentKindRewriteSkip) {
	p.skipped[path] = reason
	p.log.Warn("agent kind rewrite skipped", "reason", string(reason), "path", path)
}

func (p *agentKindRewritePass) rewriteChild(root *fileutil.Directory, rootPath string, name string) {
	childPath := filepath.Join(rootPath, name)
	child, err := root.OpenDirectoryForMutation(name)
	if err != nil {
		switch {
		case isSymlinkPath(childPath, err):
			p.skip(childPath, AgentKindRewriteSkipSymlink)
		case errors.Is(err, fileutil.ErrNotDirectory), errors.Is(err, fs.ErrNotExist):
		default:
			p.skip(childPath, AgentKindRewriteSkipIO)
		}
		return
	}
	defer func() {
		if closeErr := child.Close(); closeErr != nil {
			p.log.Warn("agent kind rewrite could not close directory", "path", childPath, "error", closeErr)
		}
	}()
	path := filepath.Join(childPath, agentKindRewriteFile)
	original, info, err := child.ReadRegularFile(agentKindRewriteFile)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case isSymlinkPath(path, err):
			p.skip(path, AgentKindRewriteSkipSymlink)
		default:
			p.skip(path, AgentKindRewriteSkipIO)
		}
		return
	}
	updated, ok := rewriteAgentKindValue(original)
	if updated == nil {
		if !ok {
			p.skip(path, AgentKindRewriteSkipUnsupported)
		}
		return
	}
	if info.Mode().Perm()&agentKindRewriteOwnerWritableMask == 0 {
		p.skip(path, AgentKindRewriteSkipReadOnly)
		return
	}
	err = child.AtomicWriteFileIf(agentKindRewriteFile, updated, info.Mode().Perm(), func() error {
		if p.hooks.beforeCompare != nil {
			p.hooks.beforeCompare(path)
		}
		current, _, readErr := child.ReadRegularFile(agentKindRewriteFile)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(current, original) {
			return ErrAgentFileChanged
		}
		return nil
	})
	p.record(path, err)
}

// record classifies one publish result by whether the new bytes became visible.
func (p *agentKindRewritePass) record(path string, err error) {
	published, reason := classifyAgentKindRewriteError(err)
	if !published {
		p.skip(path, reason)
		return
	}
	p.rewritten = append(p.rewritten, path)
	if err != nil {
		p.log.Warn("agent kind rewrite durability error", "path", path, "err", err)
		return
	}
	p.log.Info("rewrote agent kind stays_on to always_on", "path", path)
}

// classifyAgentKindRewriteError reports whether a publish attempt made the new bytes visible and,
// when it did not, why the file was skipped.
func classifyAgentKindRewriteError(err error) (published bool, reason AgentKindRewriteSkip) {
	switch {
	case err == nil, errors.Is(err, fileutil.ErrPublishedNotDurable):
		return true, ""
	case errors.Is(err, ErrAgentFileChanged):
		return false, AgentKindRewriteSkipChanged
	default:
		return false, AgentKindRewriteSkipIO
	}
}

// rewriteAgentKindValue returns the original bytes with the top-level kind value span replaced.
// It returns (nil, true) when the file needs no rewrite and (nil, false) for a stays_on file it
// leaves untouched: its kind line has a shape the span edit does not support, or its top-level
// kind line reads stays_on but the frontmatter fails strict decode.
func rewriteAgentKindValue(original []byte) ([]byte, bool) {
	if !bytes.Contains(original, []byte(retiredAgentKind)) {
		return nil, true
	}
	start, end, ok := agentKindValueSpan(original)
	kind, err := decodeAgentFileKind(original)
	if err != nil {
		return nil, !ok
	}
	if kind != retiredAgentKind {
		return nil, true
	}
	if !ok {
		return nil, false
	}
	updated := slices.Concat(original[:start], []byte(AgentKindAlwaysOn), original[end:])
	if kind, err := decodeAgentFileKind(updated); err != nil || kind != AgentKindAlwaysOn {
		return nil, false
	}
	return updated, true
}

// decodeAgentFileKind strictly decodes the frontmatter and reports its kind. It does not run the
// loader's validation, so a well-formed file the loader rejects for another reason still reports
// its kind; a frontmatter that fails strict decode (an unknown key or invalid YAML/TOML) returns
// the decode error.
func decodeAgentFileKind(content []byte) (kind string, err error) {
	var parsed parsedAgentDef
	_, err = frontmatter.Decode(content, func(data []byte) error {
		return decodeAgentFrontmatter(data, &parsed)
	})
	return parsed.Kind, err
}

// agentKindValueSpan locates the stays_on bytes of the single top-level kind line inside the
// frontmatter of the original, unnormalized buffer.
func agentKindValueSpan(content []byte) (start int, end int, ok bool) {
	offset, opened := 0, false
	found := false
	for offset < len(content) {
		lineEnd := bytes.IndexByte(content[offset:], '\n')
		next := len(content)
		if lineEnd >= 0 {
			next = offset + lineEnd + 1
			lineEnd += offset
		} else {
			lineEnd = len(content)
		}
		line := bytes.TrimSuffix(content[offset:lineEnd], []byte("\r"))
		switch {
		case !opened:
			if string(line) != agentKindRewriteDelimiter {
				return 0, 0, false
			}
			opened = true
		case string(line) == agentKindRewriteDelimiter:
			return start, end, found
		case agentKindTopLevelLine.Match(line):
			match := agentKindYAMLLine.FindSubmatchIndex(line)
			if match == nil {
				match = agentKindTOMLLine.FindSubmatchIndex(line)
			}
			if match == nil || found {
				return 0, 0, false
			}
			valueStart, valueEnd := match[2], match[3]
			if line[valueStart] == '"' || line[valueStart] == '\'' {
				valueStart, valueEnd = valueStart+1, valueEnd-1
			}
			start, end, found = offset+valueStart, offset+valueEnd, true
		}
		offset = next
	}
	return 0, 0, false
}

func isSymlinkPath(path string, err error) bool {
	if errors.Is(err, fileutil.ErrSymlink) {
		return true
	}
	info, statErr := os.Lstat(path)
	return statErr == nil && info.Mode()&os.ModeSymlink != 0
}
