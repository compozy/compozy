package inline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	entrypointFileName             = "__inline_entrypoint.ts"
	modulePrefix                   = "inline"
	defaultModuleExt               = ".ts"
	defaultDirPerm     fs.FileMode = 0o755
)

// Options configures the inline manager with project metadata and resource store dependencies.
// Fields are normalized when building the manager so runtime paths are deterministic.
type Options struct {
	ProjectRoot    string
	ProjectName    string
	Store          resources.ResourceStore
	UserEntrypoint string
	WorkerFilePerm fs.FileMode
}

// Manager materializes inline tools on disk and watches project resources for live updates.
// It keeps generated modules and entrypoint files in sync so worker runtimes execute fresh code.
type Manager struct {
	opts           Options
	inlineDir      string
	entrypointPath string

	mu             sync.Mutex
	modules        map[string]moduleState
	entrypointHash string

	startOnce sync.Once
	closeOnce sync.Once
	startErr  error

	syncCh chan struct{}
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type moduleState struct {
	fileName string
	checksum string
}

type moduleSpec struct {
	id       string
	code     string
	fileName string
	checksum string
}

// NewManager constructs an inline Manager, validating options and preparing runtime directories
// without starting background synchronization loops.
func NewManager(ctx context.Context, opts Options) (*Manager, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if opts.Store == nil {
		return nil, fmt.Errorf("resource store is required")
	}
	project := strings.TrimSpace(opts.ProjectName)
	if project == "" {
		return nil, fmt.Errorf("project name is required")
	}
	root := strings.TrimSpace(opts.ProjectRoot)
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve project root: %w", err)
		}
		root = cwd
	}
	perm := opts.WorkerFilePerm
	if perm == 0 {
		perm = 0o600
	}
	inlineDir := filepath.Join(core.GetStoreDir(root), "runtime", "inline")
	manager := &Manager{
		opts: Options{
			ProjectRoot:    root,
			ProjectName:    project,
			Store:          opts.Store,
			UserEntrypoint: strings.TrimSpace(opts.UserEntrypoint),
			WorkerFilePerm: perm,
		},
		inlineDir:      inlineDir,
		entrypointPath: filepath.Join(inlineDir, entrypointFileName),
		modules:        make(map[string]moduleState),
	}
	return manager, nil
}

// Start initializes synchronization goroutines and subscribes to tool changes in the resource store.
// It returns any initialization error and preserves the first failure for subsequent callers.
func (m *Manager) Start(ctx context.Context) error {
	m.startOnce.Do(func() {
		if ctx == nil {
			m.startErr = fmt.Errorf("context is required")
			return
		}
		if err := os.MkdirAll(m.inlineDir, defaultDirPerm); err != nil {
			m.startErr = fmt.Errorf("ensure inline directory: %w", err)
			return
		}
		syncCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		m.cancel = cancel
		m.syncCh = make(chan struct{}, 1)
		m.wg.Add(1)
		go m.runSyncLoop(syncCtx)
		if err := m.Sync(ctx); err != nil {
			m.startErr = err
			cancel()
			m.wg.Wait()
			return
		}
		events, err := m.opts.Store.Watch(syncCtx, m.opts.ProjectName, resources.ResourceTool)
		if err != nil {
			m.startErr = fmt.Errorf("watch tool resources: %w", err)
			cancel()
			m.wg.Wait()
			return
		}
		m.wg.Add(1)
		go m.runWatcher(syncCtx, events)
		m.startErr = nil
	})
	return m.startErr
}

// Close signals shutdown to background loops and waits for all goroutines to exit.
// It is safe to call multiple times and always returns nil.
func (m *Manager) Close(_ context.Context) error {
	var err error
	m.closeOnce.Do(func() {
		if m.cancel != nil {
			m.cancel()
		}
		m.wg.Wait()
	})
	return err
}

// Sync reconciles inline modules against the current tool registry and writes updated source files.
// It can be invoked manually and is also used internally by the background watcher.
func (m *Manager) Sync(ctx context.Context) error {
	if err := os.MkdirAll(m.inlineDir, defaultDirPerm); err != nil {
		return fmt.Errorf("ensure inline directory: %w", err)
	}
	modules, err := m.collectModules(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.applyModuleDiff(ctx, modules); err != nil {
		return err
	}
	return m.writeEntrypoint(ctx, modules)
}

// EntrypointPath returns the absolute path to the generated inline entrypoint TypeScript file.
func (m *Manager) EntrypointPath() string {
	return m.entrypointPath
}

// ModulePath reports the absolute file path for a tool module and whether it exists in the local cache.
func (m *Manager) ModulePath(toolID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.modules[toolID]
	if !ok {
		return "", false
	}
	return filepath.Join(m.inlineDir, state.fileName), true
}

func (m *Manager) enqueueSync() {
	if m.syncCh == nil {
		return
	}
	select {
	case m.syncCh <- struct{}{}:
	default:
	}
}

func (m *Manager) runSyncLoop(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.syncCh:
			if err := m.Sync(ctx); err != nil {
				log := logger.FromContext(ctx)
				if log != nil {
					log.Warn("inline manager sync failed", "error", err)
				}
			}
		}
	}
}

func (m *Manager) runWatcher(ctx context.Context, events <-chan resources.Event) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				return
			}
			m.enqueueSync()
		}
	}
}

func (m *Manager) collectModules(ctx context.Context) ([]moduleSpec, error) {
	items, err := m.opts.Store.ListWithValues(ctx, m.opts.ProjectName, resources.ResourceTool)
	if err != nil {
		return nil, fmt.Errorf("list inline tools: %w", err)
	}
	modules := make([]moduleSpec, 0, len(items))
	for _, item := range items {
		cfg, err := extractToolConfig(item.Value)
		if err != nil {
			log := logger.FromContext(ctx)
			if log != nil {
				log.Warn("skip invalid tool config for inline sync", "error", err, "tool_id", item.Key.ID)
			}
			continue
		}
		if !includeInlineTool(cfg) {
			continue
		}
		code := ensureTrailingNewline(cfg.Code)
		hash := contentHash(code)
		fileName := fmt.Sprintf("%s_%s%s", sanitizeToolID(cfg.ID), hash[:12], defaultModuleExt)
		modules = append(modules, moduleSpec{
			id:       cfg.ID,
			code:     code,
			fileName: fileName,
			checksum: hash,
		})
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].id < modules[j].id
	})
	return modules, nil
}

func (m *Manager) applyModuleDiff(_ context.Context, modules []moduleSpec) error {
	next := make(map[string]moduleState, len(modules))
	for _, spec := range modules {
		current, ok := m.modules[spec.id]
		if ok && current.checksum == spec.checksum && current.fileName == spec.fileName {
			next[spec.id] = current
			continue
		}
		if err := writeFileAtomic(m.inlineDir, spec.fileName, []byte(spec.code), m.opts.WorkerFilePerm); err != nil {
			return fmt.Errorf("write inline module %s: %w", spec.id, err)
		}
		if ok && current.fileName != spec.fileName {
			_ = os.Remove(filepath.Join(m.inlineDir, current.fileName))
		}
		next[spec.id] = moduleState{fileName: spec.fileName, checksum: spec.checksum}
	}
	for toolID, state := range m.modules {
		if _, ok := next[toolID]; ok {
			continue
		}
		path := filepath.Join(m.inlineDir, state.fileName)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale inline module %s: %w", toolID, err)
		}
	}
	m.modules = next
	return nil
}

func (m *Manager) writeEntrypoint(_ context.Context, modules []moduleSpec) error {
	content := m.buildEntrypoint(modules)
	hash := contentHash(content)
	if hash == m.entrypointHash {
		return nil
	}
	dir := filepath.Dir(m.entrypointPath)
	name := filepath.Base(m.entrypointPath)
	if err := writeFileAtomic(dir, name, []byte(content), m.opts.WorkerFilePerm); err != nil {
		return fmt.Errorf("write inline entrypoint: %w", err)
	}
	m.entrypointHash = hash
	return nil
}

func (m *Manager) buildEntrypoint(modules []moduleSpec) string {
	var imports []string
	var assignments []string
	for idx, spec := range modules {
		alias := fmt.Sprintf("%s%d", modulePrefix, idx)
		imports = append(imports, fmt.Sprintf("import %s from \"./%s\";", alias, filepath.ToSlash(spec.fileName)))
		assignments = append(assignments, fmt.Sprintf("  %q: %s,", spec.id, alias))
	}
	userImport := m.resolveUserImport()
	builder := strings.Builder{}
	builder.WriteString("// Code generated by Compozy inline manager. DO NOT EDIT.\n")
	if userImport != "" {
		builder.WriteString(fmt.Sprintf("import * as userExports from %q;\n", userImport))
	} else {
		builder.WriteString("const userExports = {};\n")
	}
	for _, line := range imports {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	builder.WriteString("const baseExports = userExports?.default ?? userExports ?? {};\n")
	builder.WriteString("const inlineExports = {\n")
	for _, assign := range assignments {
		builder.WriteString(assign)
		builder.WriteByte('\n')
	}
	builder.WriteString("};\n")
	builder.WriteString("export default {\n")
	builder.WriteString("  ...baseExports,\n")
	if len(assignments) > 0 {
		builder.WriteString("  ...inlineExports,\n")
	}
	builder.WriteString("};\n")
	return builder.String()
}

func (m *Manager) resolveUserImport() string {
	path := strings.TrimSpace(m.opts.UserEntrypoint)
	if path == "" {
		return ""
	}
	if bareModuleSpecifier(path) {
		return path
	}
	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(m.opts.ProjectRoot, target)
	}
	target = filepath.Clean(target)
	rel, err := filepath.Rel(filepath.Dir(m.entrypointPath), target)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "./") {
		return rel
	}
	return "./" + rel
}

func includeInlineTool(cfg *tool.Config) bool {
	if cfg == nil {
		return false
	}
	code := strings.TrimSpace(cfg.Code)
	if code == "" {
		return false
	}
	impl := strings.TrimSpace(cfg.Implementation)
	if impl == "" || impl == tool.ImplementationRuntime {
		return true
	}
	return false
}

func extractToolConfig(value any) (*tool.Config, error) {
	switch v := value.(type) {
	case *tool.Config:
		return v, nil
	case tool.Config:
		return &v, nil
	default:
		return nil, fmt.Errorf("unexpected tool config type %T", value)
	}
}

func sanitizeToolID(id string) string {
	if strings.TrimSpace(id) == "" {
		return "tool"
	}
	builder := strings.Builder{}
	for _, r := range strings.ToLower(id) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	slug := strings.Trim(builder.String(), "-_")
	if slug == "" {
		return "tool"
	}
	return slug
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func ensureTrailingNewline(code string) string {
	if strings.HasSuffix(code, "\n") {
		return code
	}
	return code + "\n"
}

func writeFileAtomic(dir, name string, data []byte, perm fs.FileMode) error {
	if perm == 0 {
		perm = 0o600
	}
	tmp, err := os.CreateTemp(dir, "inline-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	target := filepath.Join(dir, name)
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func bareModuleSpecifier(path string) bool {
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, ".") {
		return false
	}
	return !strings.ContainsAny(path, `/\`)
}
