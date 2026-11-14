package runtimemcp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func ResolveContext(c ReadContext) ReadContext {
	if c.CWD == "" {
		return c
	}
	for dir := filepath.Clean(c.CWD); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		path := filepath.Join(dir, ".git")
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if c.WorktreeRoot == "" {
			c.WorktreeRoot = dir
		}
		if c.GitRoot != "" {
			break
		}
		c.GitRoot = dir
		if info.Mode().IsRegular() {
			data, _, err := fileutil.ReadRegularFile(path, maxFileBytes)
			if err != nil {
				break
			}
			gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
			if !ok {
				break
			}
			if !filepath.IsAbs(gitdir) {
				gitdir = filepath.Join(dir, gitdir)
			}
			common, _, err := fileutil.ReadRegularFile(filepath.Join(gitdir, "commondir"), maxFileBytes)
			if err != nil {
				break
			}
			commonDir := strings.TrimSpace(string(common))
			if !filepath.IsAbs(commonDir) {
				commonDir = filepath.Join(gitdir, commonDir)
			}
			c.GitRoot = filepath.Dir(filepath.Clean(commonDir))
		}
		break
	}
	return c
}
