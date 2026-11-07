package projectfiles

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/workspacefiles"
	"github.com/compozy/compozy/internal/worktree"
)

func (r *FileReader) List(ctx context.Context, root Root, dir string, opts ListOptions) (listing Listing, err error) {
	if err := ctx.Err(); err != nil {
		return Listing{}, err
	}
	rel, _, err := resolvePath(root, dir)
	if err != nil {
		return Listing{}, err
	}
	confined, err := os.OpenRoot(root.AbsPath)
	if err != nil {
		return Listing{}, ErrRootUnavailable
	}
	defer func() { err = errors.Join(err, confined.Close()) }()
	info, err := confined.Stat(rel)
	if err != nil || !info.IsDir() {
		return Listing{}, ErrNotADirectory
	}
	file, err := confined.OpenFile(rel, os.O_RDONLY|directoryOpenFlags, 0)
	if err != nil {
		return Listing{}, ErrPathOutsideRoot
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err = file.Stat()
	if err != nil || !info.IsDir() {
		return Listing{}, ErrNotADirectory
	}
	children, err := file.ReadDir(-1)
	if err != nil {
		return Listing{}, ErrNotADirectory
	}
	result := Listing{Root: root, Path: dir, Entries: []Entry{}}
	states, ignored, changedDirs, branch, statusErr := r.cachedStatus(ctx, root, opts.Refresh)
	result.StatusUnavailable = statusErr != nil
	if result.Root.Branch == "" {
		result.Root.Branch = branch
	}
	for _, child := range orderedListingChildren(root, confined, dir, children) {
		if err := ctx.Err(); err != nil {
			return Listing{}, err
		}
		entry, ok := listingEntry(root, confined, dir, child, states, ignored, changedDirs)
		if !ok || (!opts.ShowIgnored && entry.Ignored) {
			continue
		}

		result.Entries = append(result.Entries, entry)
		if len(result.Entries) > EntryLimit {
			break
		}
	}
	slices.SortFunc(result.Entries, func(a, b Entry) int {
		if a.Kind != b.Kind {
			if a.Kind == EntryKindDirectory {
				return -1
			}
			return 1
		}
		return naturalCompare(a.Name, b.Name)
	})
	result.Truncated = len(result.Entries) > EntryLimit
	result.Entries = result.Entries[:min(len(result.Entries), EntryLimit)]
	return result, nil
}
func naturalCompare(a, b string) int {
	left, right := strings.ToLower(a), strings.ToLower(b)
	for left != "" && right != "" {
		if left[0] >= '0' && left[0] <= '9' && right[0] >= '0' && right[0] <= '9' {
			i, j := 0, 0
			for i < len(left) && left[i] >= '0' && left[i] <= '9' {
				i++
			}
			for j < len(right) && right[j] >= '0' && right[j] <= '9' {
				j++
			}
			x, y := strings.TrimLeft(left[:i], "0"), strings.TrimLeft(right[:j], "0")
			if c := cmp.Compare(len(x), len(y)); c != 0 {
				return c
			}
			if c := cmp.Compare(x, y); c != 0 {
				return c
			}
			left, right = left[i:], right[j:]
			continue
		}
		if c := cmp.Compare(left[0], right[0]); c != 0 {
			return c
		}
		left, right = left[1:], right[1:]
	}
	if c := cmp.Compare(len(left), len(right)); c != 0 {
		return c
	}
	return cmp.Compare(a, b)
}
func (r *FileReader) status(ctx context.Context, root Root) (map[string]GitStatus, map[string]bool, string, error) {
	states, ignored := map[string]GitStatus{}, map[string]bool{}
	if r.gitRunner == nil {
		return states, ignored, "", ErrRootUnavailable
	}
	prefix, err := r.git(ctx, root.AbsPath, "rev-parse", "--show-prefix")
	if err != nil {
		return states, ignored, "", err
	}
	data, err := r.git(
		ctx,
		root.AbsPath,
		"status",
		"--porcelain=v2",
		"--branch",
		"-z",
		"--ignored=matching",
		"--untracked-files=all",
		"--",
		".",
	)
	if err != nil {
		return states, ignored, "", err
	}
	parsed, err := worktree.ParseStatusV2(data)
	if err != nil {
		return states, ignored, "", err
	}
	parseStatusEntries(data, strings.TrimSuffix(string(prefix), "\n"), states, ignored)
	return states, ignored, parsed.Branch, nil
}

func listingEntry(
	root Root,
	confined *os.Root,
	dir string,
	child os.DirEntry,
	states map[string]GitStatus,
	ignored map[string]bool,
	changedDirs map[string]bool,
) (Entry, bool) {
	name := path.Join(dir, child.Name())
	if child.Type()&os.ModeSymlink != 0 {
		if _, keep := workspacefiles.ClassifySymlink(root.AbsPath, name); !keep {
			return Entry{}, false
		}
	}
	if err := ValidatePath(name); err != nil {
		return Entry{}, false
	}
	target, sensitive, err := resolvePath(root, name)
	if err != nil {
		return Entry{}, false
	}
	stat, err := confined.Stat(target)
	if err != nil || (!stat.IsDir() && !stat.Mode().IsRegular()) {
		return Entry{}, false
	}
	entry := Entry{
		Name:      child.Name(),
		Path:      name,
		Kind:      EntryKindFile,
		Size:      stat.Size(),
		Symlink:   child.Type()&os.ModeSymlink != 0,
		Sensitive: sensitive,
		GitStatus: states[name],
	}
	for parent := name; parent != "." && parent != "/"; parent = path.Dir(parent) {
		if ignored[parent] {
			entry.Ignored = true
			break
		}
	}
	if stat.IsDir() {
		entry.Kind = EntryKindDirectory
		entry.Size = 0
		entry.ContainsChanges = changedDirs[name]
	}
	return entry, true
}

func parseStatusEntries(data []byte, prefix string, states map[string]GitStatus, ignored map[string]bool) {
	records := strings.Split(string(data), "\x00")
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) < 2 || record[0] == '#' {
			continue
		}
		var name string
		var state GitStatus
		switch record[0] {
		case '?', '!':
			name = record[2:]
			state = "untracked"
		case '1':
			fields := strings.SplitN(record, " ", 9)
			if len(fields) != 9 {
				continue
			}
			name = fields[8]
			state = "modified"
			if strings.Contains(fields[1], "A") {
				state = "added"
			}
		case '2':
			fields := strings.SplitN(record, " ", 10)
			if len(fields) != 10 {
				continue
			}
			name = fields[9]
			state = "renamed"
			i++
		case 'u':
			fields := strings.SplitN(record, " ", 11)
			if len(fields) != 11 {
				continue
			}
			name = fields[10]
			state = "conflicted"
		default:
			continue
		}
		name, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		name = strings.TrimSuffix(filepath.ToSlash(name), "/")
		if ValidatePath(name) != nil {
			continue
		}
		if record[0] == '!' {
			ignored[name] = true
		} else {
			states[name] = state
		}
	}
}

func changedDirectories(states map[string]GitStatus) map[string]bool {
	changedDirs := map[string]bool{}
	for changed := range states {
		for parent := path.Dir(changed); parent != "." && parent != "/"; parent = path.Dir(parent) {
			changedDirs[parent] = true
		}
	}
	return changedDirs
}
