package core

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// BrowseDirectory lists immediate directory entries so the onboarding workspace
// picker can navigate the local filesystem. It is read-only and operator-scoped:
// the daemon already runs with the operator's filesystem access.
func (h *BaseHandlers) BrowseDirectory(c *gin.Context) {
	home := operatorHomeDir()

	requested := strings.TrimSpace(c.Query("path"))
	if requested == "" {
		requested = home
	}
	if requested == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("api: directory path is required"))
		return
	}
	if !filepath.IsAbs(requested) {
		h.respondError(c, http.StatusBadRequest, errors.New("api: directory path must be absolute"))
		return
	}
	if containsControlChar(requested) {
		h.respondError(c, http.StatusBadRequest, errors.New("api: directory path contains invalid characters"))
		return
	}

	resolved, err := resolveBrowseDir(requested)
	if err != nil {
		h.respondError(c, statusForBrowseError(err), err)
		return
	}

	showHidden := strings.EqualFold(strings.TrimSpace(c.Query("show_hidden")), "true")
	dirsOnly := strings.EqualFold(strings.TrimSpace(c.Query("dirs_only")), "true")

	entries, err := readBrowseEntries(resolved, showHidden, dirsOnly)
	if err != nil {
		h.respondError(c, statusForBrowseError(err), err)
		return
	}

	response := contract.FSBrowseResponse{
		Path:    resolved,
		Home:    home,
		Entries: entries,
	}
	if parent := filepath.Dir(resolved); parent != resolved {
		response.Parent = parent
	}
	c.JSON(http.StatusOK, response)
}

func resolveBrowseDir(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("api: resolve directory %q: %w", path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("api: stat directory %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("api: %q is not a directory: %w", resolved, errBrowseNotDir)
	}
	return resolved, nil
}

func readBrowseEntries(dir string, showHidden bool, dirsOnly bool) ([]contract.FSEntryPayload, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("api: read directory %q: %w", dir, err)
	}
	entries := make([]contract.FSEntryPayload, 0, len(dirEntries))
	for _, entry := range dirEntries {
		name := entry.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			if info, statErr := os.Stat(filepath.Join(dir, name)); statErr == nil {
				isDir = info.IsDir()
			}
		}
		if dirsOnly && !isDir {
			continue
		}
		entries = append(entries, contract.FSEntryPayload{
			Name:  name,
			Path:  filepath.Join(dir, name),
			IsDir: isDir,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

var errBrowseNotDir = errors.New("path is not a directory")

func statusForBrowseError(err error) int {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, os.ErrPermission):
		return http.StatusForbidden
	case errors.Is(err, errBrowseNotDir):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func containsControlChar(path string) bool {
	for _, r := range path {
		if r == 0 || (r < 0x20) || r == 0x7f {
			return true
		}
	}
	return false
}

func operatorHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(home)
}
