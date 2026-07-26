package memory

import (
	"context"
	"errors"
	"fmt"

	"os"
	"path/filepath"

	"strings"

	"time"

	"github.com/compozy/agh/internal/fileutil"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func (s *Store) dirForScope(scope memcontract.Scope) (string, error) {
	normalized := scope.Normalize()
	if err := normalized.Validate(); err != nil {
		return "", wrapValidationError("resolve scope", string(scope), err)
	}

	switch normalized {
	case memcontract.ScopeGlobal:
		if s.globalDir == "" {
			return "", wrapValidationError("resolve scope", string(scope), errors.New("global directory is required"))
		}
		return s.globalDir, nil
	case memcontract.ScopeWorkspace:
		if s.workspaceDir == "" {
			return "", wrapValidationError(
				"resolve scope",
				string(scope),
				errors.New("workspace directory is required"),
			)
		}
		return s.workspaceDir, nil
	case memcontract.ScopeAgent:
		return s.agentMemoryDir()
	default:
		return "", wrapValidationError("resolve scope", string(scope), fmt.Errorf("unsupported scope %q", scope))
	}
}

func (s *Store) agentConfigured() bool {
	return strings.TrimSpace(s.agentName) != "" || strings.TrimSpace(string(s.agentTier)) != ""
}

func (s *Store) agentMemoryDir() (string, error) {
	agentName, err := cleanPathSegment("agent", s.agentName)
	if err != nil {
		return "", err
	}
	tier := s.agentTier.Normalize()
	if err := tier.Validate(); err != nil {
		return "", wrapValidationError("resolve agent tier", string(s.agentTier), err)
	}
	switch tier {
	case memcontract.AgentTierGlobal:
		root, err := globalHomeFromMemoryDir(s.globalDir)
		if err != nil {
			return "", err
		}
		return filepath.Join(root, "agents", agentName, memoryDirName), nil
	case memcontract.AgentTierWorkspace:
		if strings.TrimSpace(s.workspaceRoot) == "" {
			return "", wrapValidationError(
				"resolve agent workspace",
				agentName,
				errors.New("workspace directory is required"),
			)
		}
		return filepath.Join(s.workspaceRoot, ".agh", "agents", agentName, memoryDirName), nil
	default:
		return "", wrapValidationError(
			"resolve agent tier",
			string(s.agentTier),
			fmt.Errorf("unsupported agent tier %q", s.agentTier),
		)
	}
}

func globalHomeFromMemoryDir(globalDir string) (string, error) {
	dir := cleanDirPath(globalDir)
	if dir == "" {
		return "", wrapValidationError(
			"resolve global agent memory",
			"global",
			errors.New("global directory is required"),
		)
	}
	if filepath.Base(dir) == memoryDirName {
		return filepath.Dir(dir), nil
	}
	return filepath.Dir(dir), nil
}

func cleanPathSegment(kind string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", wrapValidationError("resolve "+kind, value, fmt.Errorf("%s is required", kind))
	}
	if strings.ContainsRune(trimmed, 0) || filepath.IsAbs(trimmed) {
		return "", wrapValidationError("resolve "+kind, value, fmt.Errorf("invalid %s path segment", kind))
	}
	if trimmed == "." || trimmed == ".." || strings.ContainsAny(trimmed, `/\`) {
		return "", wrapValidationError("resolve "+kind, value, fmt.Errorf("invalid %s path segment", kind))
	}
	return trimmed, nil
}

func (s *Store) pathFor(scope memcontract.Scope, filename string) (string, error) {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return "", err
	}

	base, err := cleanFilename(filename)
	if err != nil {
		return "", wrapValidationError("resolve filename", filename, err)
	}

	return filepath.Join(dir, base), nil
}

func (s *Store) completeHeaderForScope(
	scope memcontract.Scope,
	header memcontract.Header,
) (memcontract.Header, error) {
	normalized := scope.Normalize()
	if normalized == "" {
		return header, nil
	}
	header.Scope = normalized
	if normalized != memcontract.ScopeAgent {
		return header, nil
	}
	agentName, err := cleanPathSegment("agent", s.agentName)
	if err != nil {
		return memcontract.Header{}, err
	}
	tier := s.agentTier.Normalize()
	if err := tier.Validate(); err != nil {
		return memcontract.Header{}, wrapValidationError("resolve agent tier", string(s.agentTier), err)
	}
	if strings.TrimSpace(header.AgentName) == "" {
		header.AgentName = agentName
	} else if strings.TrimSpace(header.AgentName) != agentName {
		return memcontract.Header{}, wrapValidationError(
			"resolve agent",
			header.AgentName,
			fmt.Errorf("frontmatter agent does not match store agent %q", agentName),
		)
	}
	if header.AgentTier.Normalize() == "" {
		header.AgentTier = tier
	} else if header.AgentTier.Normalize() != tier {
		return memcontract.Header{}, wrapValidationError(
			"resolve agent tier",
			string(header.AgentTier),
			fmt.Errorf("frontmatter agent tier does not match store tier %q", tier),
		)
	}
	return header, nil
}

func (s *Store) syncScope(ctx context.Context, scope memcontract.Scope) error {
	scope = scope.Normalize()
	headers, err := s.scan(scope, 0)
	if err != nil {
		return err
	}
	if err := s.syncIndex(scope, headers); err != nil {
		return err
	}
	if err := s.syncCatalogScope(ctx, scope, headers); err != nil {
		return err
	}
	return nil
}

func (s *Store) syncIndex(scope memcontract.Scope, headers []memcontract.Header) error {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return err
	}

	indexPath := filepath.Join(dir, indexFilename)
	if len(headers) == 0 {
		if err := os.Remove(indexPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("memory: remove empty index %q: %w", indexPath, err)
		}
		return nil
	}

	if err := fileutil.AtomicWrite(indexPath, []byte(renderIndex(headers))); err != nil {
		return fmt.Errorf("memory: write index %q: %w", indexPath, err)
	}
	return nil
}

func (s *Store) syncCatalogScope(ctx context.Context, scope memcontract.Scope, headers []memcontract.Header) error {
	if s.catalog == nil {
		return nil
	}
	workspaceRoot, workspaceID, err := s.catalogWorkspaceForScope(ctx, scope)
	if err != nil {
		return err
	}
	docs, err := s.documentsForHeaders(scope, workspaceRoot, workspaceID, headers)
	if err != nil {
		return err
	}
	if err := s.catalog.replaceScope(
		ctx,
		scope,
		workspaceID,
		s.catalogAgentName(scope),
		s.catalogAgentTier(scope),
		docs,
	); err != nil {
		return err
	}
	if err := s.catalog.setLastReindex(ctx, time.Now().UTC()); err != nil {
		return err
	}
	return nil
}
