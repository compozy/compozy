package attachment

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/engine/core"
)

// EffectiveItem carries an attachment along with the CWD that should be used
// to resolve its filesystem paths. This is necessary because attachments can
// originate from task, agent, or action scopes with different roots.
type EffectiveItem struct {
	Att Attachment
	CWD *core.PathCWD
}

// canonicalKey generates a deterministic de-duplication key for an attachment
// using its type and either URL or canonical absolute path. When the URL/path
// is empty (should not happen after Phase1), an empty key is returned.
func canonicalKey(att Attachment, cwd *core.PathCWD) string {
	if att == nil {
		return ""
	}
	tp, src, urlStr, path := attFields(att)
	if strings.TrimSpace(string(tp)) == "" {
		return ""
	}
	if src == SourceURL {
		if urlStr != "" {
			if u, err := url.Parse(strings.TrimSpace(urlStr)); err == nil {
				u.Fragment = ""
				u.User = nil
				u.Host = strings.ToLower(u.Host)
				return string(tp) + ":url:" + u.String()
			}
			return string(tp) + ":url:" + strings.TrimSpace(urlStr)
		}
		return ""
	}
	if path != "" {
		return string(tp) + ":path:" + makeAbs(cwd, path)
	}
	return ""
}

// attFields extracts the common discriminator, source and singular URL/Path
// values for any supported attachment type.
func attFields(att Attachment) (Type, Source, string, string) {
	switch a := att.(type) {
	case *ImageAttachment:
		return a.Type(), a.Source, a.URL, a.Path
	case *PDFAttachment:
		return a.Type(), a.Source, a.URL, a.Path
	case *AudioAttachment:
		return a.Type(), a.Source, a.URL, a.Path
	case *VideoAttachment:
		return a.Type(), a.Source, a.URL, a.Path
	case *URLAttachment:
		return a.Type(), SourceURL, a.URL, ""
	case *FileAttachment:
		return a.Type(), SourcePath, "", a.Path
	default:
		return att.Type(), "", "", ""
	}
}

func makeAbs(cwd *core.PathCWD, rel string) string {
	var base string
	if cwd == nil || cwd.Path == "" {
		base = ""
	} else {
		base = filepath.Clean(cwd.Path)
	}
	var abs string
	if base == "" {
		abs = filepath.Clean(rel)
	} else {
		abs = filepath.Clean(filepath.Join(base, rel))
	}
	if base != "" {
		relToBase, err := filepath.Rel(base, abs)
		if err != nil || relToBase == ".." || strings.HasPrefix(relToBase, ".."+string(os.PathSeparator)) {
			return ""
		}
	}
	return abs
}

// ComputeEffectiveItems merges attachments from task, agent, and action scopes
// following precedence (task < agent < action) with deterministic de-duplication
// by canonical key. The order of first appearance is preserved.
func ComputeEffectiveItems(
	task []Attachment, taskCWD *core.PathCWD,
	agent []Attachment, agentCWD *core.PathCWD,
	action []Attachment, actionCWD *core.PathCWD,
) []EffectiveItem {
	order := make([]string, 0)
	items := make(map[string]EffectiveItem)
	put := func(att Attachment, cwd *core.PathCWD) {
		if att == nil {
			return
		}
		key := canonicalKey(att, cwd)
		if key == "" {
			return
		}
		if _, seen := items[key]; !seen {
			order = append(order, key)
		}
		items[key] = EffectiveItem{Att: att, CWD: cwd}
	}
	for _, a := range task {
		put(a, taskCWD)
	}
	for _, a := range agent {
		put(a, agentCWD)
	}
	for _, a := range action {
		put(a, actionCWD)
	}
	out := make([]EffectiveItem, 0, len(order))
	for _, k := range order {
		out = append(out, items[k])
	}
	return out
}
