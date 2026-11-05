package config

import (
	"bytes"
	"log/slog"
	"os"
	"regexp"
	"slices"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/frontmatter"
)

// Persisted-file migration only; public inputs remain strict. Removal: v0.6.0.
const retiredAgentKind = "stays_on"
const agentKindRewriteDelimiter = "---"

var (
	agentKindYAMLLine     = regexp.MustCompile(`^kind:[ \t]+("stays_on"|'stays_on'|stays_on)([ \t]+#.*)?[ \t]*$`)
	agentKindTOMLLine     = regexp.MustCompile(`^kind[ \t]*=[ \t]*("stays_on"|'stays_on')([ \t]+#.*)?[ \t]*$`)
	agentKindTopLevelLine = regexp.MustCompile(`^kind[ \t]*[:=]`)
)

// migratePersistedAgentKind preserves every byte outside the scalar kind value.
// Read-only/package-owned or unsupported layouts still decode losslessly for this window.
func migratePersistedAgentKind(
	directory *fileutil.Directory,
	path string,
	info os.FileInfo,
	contents []byte,
	publish bool,
) {
	kind, err := decodeAgentFileKind(contents)
	if err != nil || kind != retiredAgentKind {
		return
	}
	updated, supported := rewriteAgentKindValue(contents)
	if !publish || !supported || updated == nil || info.Mode().Perm()&0200 == 0 {
		slog.Warn("agent persisted kind migration deferred; use always_on before v0.6.0", "path", path)
		return
	}
	if err := publishRetiredConfigArchive(directory, path, info, contents, updated); err != nil {
		slog.Warn("agent persisted kind migration deferred; use always_on before v0.6.0", "path", path, "error", err)
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
