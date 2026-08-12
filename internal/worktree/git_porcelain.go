package worktree

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type ParseError struct {
	Format string
	Record string
}

// ParseUntrackedStatusV2 returns the exact untracked additions from a porcelain-v2
// snapshot. Ignored records are deliberately absent from the result.
func ParseUntrackedStatusV2(output []byte) ([]string, error) {
	files := make([]string, 0)
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 || record[0] == '#' || record[0] == '!' {
			continue
		}
		if record[0] != '?' {
			continue
		}
		if len(record) < 3 || record[1] != ' ' {
			return nil, &ParseError{Format: "status-v2-untracked", Record: string(record)}
		}
		name := string(record[2:])
		if name == "" {
			return nil, &ParseError{Format: "status-v2-untracked", Record: string(record)}
		}
		files = append(files, name)
	}
	slices.Sort(files)
	return files, nil
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("worktree: parse %s porcelain record %q", e.Format, e.Record)
}

type GitWorktree struct {
	Path           string
	HeadSHA        string
	Branch         string
	Main           bool
	Detached       bool
	Bare           bool
	Locked         bool
	LockReason     string
	Prunable       bool
	PrunableReason string
}

func ParseWorktreeList(output []byte) ([]GitWorktree, error) {
	fields := bytes.Split(output, []byte{0})
	entries := make([]GitWorktree, 0)
	var current *GitWorktree
	recordIndex := 0
	flush := func() error {
		if current == nil {
			return nil
		}
		if strings.TrimSpace(current.Path) == "" {
			return &ParseError{Format: "worktree-list", Record: "missing worktree path"}
		}
		current.Main = recordIndex == 0
		if !current.Bare {
			entries = append(entries, *current)
		}
		recordIndex++
		current = nil
		return nil
	}

	for _, raw := range fields {
		field := strings.TrimSuffix(string(raw), "\n")
		if field == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		if strings.HasPrefix(field, "worktree ") {
			if err := flush(); err != nil {
				return nil, err
			}
			current = &GitWorktree{Path: strings.TrimSpace(strings.TrimPrefix(field, "worktree "))}
			continue
		}
		if current == nil {
			return nil, &ParseError{Format: "worktree-list", Record: field}
		}
		switch {
		case strings.HasPrefix(field, "HEAD "):
			current.HeadSHA = strings.TrimSpace(strings.TrimPrefix(field, "HEAD "))
		case strings.HasPrefix(field, "branch "):
			current.Branch = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(field, "branch ")), "refs/heads/")
		case field == "detached":
			current.Detached = true
		case field == "bare":
			current.Bare = true
		case field == "locked":
			current.Locked = true
		case strings.HasPrefix(field, "locked "):
			current.Locked = true
			current.LockReason = strings.TrimSpace(strings.TrimPrefix(field, "locked "))
		case field == "prunable":
			current.Prunable = true
		case strings.HasPrefix(field, "prunable "):
			current.Prunable = true
			current.PrunableReason = strings.TrimSpace(strings.TrimPrefix(field, "prunable "))
		default:
			return nil, &ParseError{Format: "worktree-list", Record: field}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return entries, nil
}

type GitStatus struct {
	Branch      string
	Upstream    string
	HeadSHA     string
	Detached    bool
	DirtyFiles  int
	HasUpstream bool
	Ahead       *int
	Behind      *int
}

func ParseStatusV2(output []byte) (GitStatus, error) {
	var result GitStatus
	remaining := output
	headOIDSeen := false
	headNameSeen := false
	for len(remaining) > 0 && remaining[0] == '#' {
		separator := nextStatusSeparator(remaining)
		line := string(remaining[:separator])
		remaining = remaining[min(separator+1, len(remaining)):]
		switch {
		case strings.HasPrefix(line, "# branch.oid "):
			result.HeadSHA = strings.TrimSpace(strings.TrimPrefix(line, "# branch.oid "))
			headOIDSeen = true
		case strings.HasPrefix(line, "# branch.head "):
			headNameSeen = true
			value := strings.TrimSpace(strings.TrimPrefix(line, "# branch.head "))
			if value == "(detached)" {
				result.Detached = true
				result.Branch = ""
			} else {
				result.Branch = value
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			result.Upstream = strings.TrimSpace(strings.TrimPrefix(line, "# branch.upstream "))
			result.HasUpstream = result.Upstream != ""
		case strings.HasPrefix(line, "# branch.ab "):
			ahead, behind, err := parseAheadBehind(strings.TrimSpace(strings.TrimPrefix(line, "# branch.ab ")))
			if err != nil {
				return GitStatus{}, err
			}
			result.Ahead, result.Behind = &ahead, &behind
		case line[0] == '#':
			continue
		default:
			return GitStatus{}, &ParseError{Format: "status-v2", Record: line}
		}
	}
	if !headOIDSeen || !headNameSeen {
		return GitStatus{}, &ParseError{Format: "status-v2", Record: "missing branch identity headers"}
	}
	records := bytes.Split(remaining, []byte{0})
	for index := 0; index < len(records); index++ {
		raw := records[index]
		if len(raw) == 0 {
			continue
		}
		switch raw[0] {
		case '1', 'u', '?':
			result.DirtyFiles++
		case '2':
			if index+1 >= len(records) || len(records[index+1]) == 0 {
				return GitStatus{}, &ParseError{Format: "status-v2", Record: string(raw)}
			}
			result.DirtyFiles++
			index++
		case '!':
			continue
		default:
			return GitStatus{}, &ParseError{Format: "status-v2", Record: string(raw)}
		}
	}
	if result.HasUpstream && (result.Ahead == nil || result.Behind == nil) {
		return GitStatus{}, &ParseError{Format: "status-v2", Record: "upstream without branch.ab"}
	}
	return result, nil
}

func nextStatusSeparator(value []byte) int {
	newline := bytes.IndexByte(value, '\n')
	nul := bytes.IndexByte(value, 0)
	switch {
	case newline < 0 && nul < 0:
		return len(value)
	case newline < 0:
		return nul
	case nul < 0:
		return newline
	default:
		return min(newline, nul)
	}
}

func parseAheadBehind(value string) (int, int, error) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "+") || !strings.HasPrefix(parts[1], "-") {
		return 0, 0, &ParseError{Format: "status-v2", Record: value}
	}
	ahead, err := strconv.Atoi(strings.TrimPrefix(parts[0], "+"))
	if err != nil {
		return 0, 0, &ParseError{Format: "status-v2", Record: value}
	}
	behind, err := strconv.Atoi(strings.TrimPrefix(parts[1], "-"))
	if err != nil {
		return 0, 0, &ParseError{Format: "status-v2", Record: value}
	}
	return ahead, behind, nil
}

type Numstat struct {
	Files      map[string]struct{}
	Insertions int
	Deletions  int
}

func ParseNumstat(output []byte) (Numstat, error) {
	result := Numstat{Files: make(map[string]struct{})}
	records := bytes.Split(output, []byte{0})
	for index := 0; index < len(records); index++ {
		record := string(records[index])
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\t", 3)
		if len(parts) != 3 {
			return Numstat{}, &ParseError{Format: "numstat", Record: record}
		}
		insertions, err := parseNumstatCount(parts[0])
		if err != nil {
			return Numstat{}, err
		}
		deletions, err := parseNumstatCount(parts[1])
		if err != nil {
			return Numstat{}, err
		}
		path := parts[2]
		if path == "" {
			if index+2 >= len(records) || len(records[index+1]) == 0 || len(records[index+2]) == 0 {
				return Numstat{}, &ParseError{Format: "numstat", Record: record}
			}
			path = string(records[index+2])
			index += 2
		}
		result.Files[path] = struct{}{}
		result.Insertions += insertions
		result.Deletions += deletions
	}
	return result, nil
}

func parseNumstatCount(value string) (int, error) {
	if value == "-" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, &ParseError{Format: "numstat", Record: value}
	}
	return parsed, nil
}

func MergeNumstat(values ...Numstat) Numstat {
	merged := Numstat{Files: make(map[string]struct{})}
	for _, value := range values {
		for path := range value.Files {
			merged.Files[path] = struct{}{}
		}
		merged.Insertions += value.Insertions
		merged.Deletions += value.Deletions
	}
	return merged
}
