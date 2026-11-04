package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/procutil"
)

type ApprovalReader interface {
	ApprovedHash(context.Context, string, string, string) (string, error)
}
type ScriptRunner struct {
	DefinitionDir, ProfileID, Agent string
	ScriptsEnabled                  bool
	Timeout                         time.Duration
	MaxOutputBytes                  int
	EnvAllow                        []string
	Approvals                       ApprovalReader
}

var _ Runner = (*ScriptRunner)(nil)

func ReadScript(dir, name string) ([]byte, string, error)    { return readScript(dir, name, true) }
func InspectScript(dir, name string) ([]byte, string, error) { return readScript(dir, name, false) }
func readScript(dir, name string, executable bool) ([]byte, string, error) {
	if !ValidScript(name) {
		return nil, "", Failure("source_script_invalid", "invalid script name")
	}
	definition, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, "", Failure("source_script_invalid", "definition directory is invalid")
	}
	root, err := filepath.EvalSymlinks(filepath.Join(definition, "sources"))
	if err != nil {
		return nil, "", Failure("source_script_missing", "sources folder is missing")
	}
	relative, err := filepath.Rel(definition, root)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, "", Failure("source_script_invalid", "sources folder escapes definition")
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, name))
	if err != nil {
		return nil, "", Failure("source_script_missing", "script is missing")
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", Failure("source_script_invalid", "script escapes sources folder")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, "", Failure("source_script_invalid", "script must be a regular file")
	}
	f, err := fileutil.OpenRegularFile(path)
	if err != nil {
		return nil, "", Failure("source_script_missing", "script could not be read")
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, "", Failure("source_script_invalid", "script must be a regular file")
	}
	if executable && info.Mode()&0111 == 0 {
		return nil, "", Failure("source_script_not_executable", "script is not executable")
	}
	data, err := io.ReadAll(io.LimitReader(f, 4<<20+1))
	if err != nil || len(data) > 4<<20 {
		return nil, "", Failure("source_script_invalid", "script exceeds 4 MiB")
	}
	hash := sha256.Sum256(data)
	return data, "sha256:" + hex.EncodeToString(hash[:]), nil
}
func (s *ScriptRunner) Poll(ctx context.Context, ref Ref, req Request) (Result, error) {
	if !s.ScriptsEnabled {
		return Result{}, Failure("source_scripts_disabled", "scripts are disabled")
	}
	data, err := s.verifiedScript(ctx, ref.Script)
	if err != nil {
		return Result{}, err
	}
	private, err := os.MkdirTemp("", "compozy-source-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(private)
	path := filepath.Join(private, ref.Script)
	// Private verified bytes must be executable; the containing directory is operator-only.
	//nolint:gosec // G306: executing a verified private script requires the owner execute bit.
	if err := os.WriteFile(path, data, 0700); err != nil {
		return Result{}, err
	}
	if len(req.State) == 0 {
		req.State = []byte("null")
	}
	if len(req.Params) == 0 {
		req.Params = []byte("{}")
	}
	input, err := json.Marshal(req)
	if err != nil {
		return Result{}, err
	}
	lifetime, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	cmd := exec.CommandContext(lifetime, path)
	cmd.Dir = req.Workspace.RootDir
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = scriptEnv(s.EnvAllow, ref.ID, s.Agent)
	procutil.ConfigureCommandProcessGroup(cmd)
	cmd.Cancel = func() error { return procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	stdout := &boundedOutput{limit: s.MaxOutputBytes, cancel: cancel}
	stderr := &tailOutput{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return Result{}, Failure("source_exit_nonzero", diagnostics.RedactAndBound(err.Error(), 500))
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
		waitErr := cmd.Wait()
		return Result{}, errors.Join(err, killErr, waitErr)
	}
	err = cmd.Wait()
	cleanupErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
	if stdout.exceeded {
		return Result{}, Failure("source_output_too_large", "stdout exceeds limit")
	}
	if lifetime.Err() != nil {
		return Result{}, Failure("source_timeout", "script poll canceled or timed out")
	}
	if err != nil {
		return Result{}, Failure(
			"source_exit_nonzero",
			diagnostics.RedactAndBound(err.Error()+": "+string(stderr.data), 500),
		)
	}
	if cleanupErr != nil {
		return Result{}, Failure("source_exit_nonzero", diagnostics.RedactAndBound(cleanupErr.Error(), 500))
	}
	var result Result
	if err := json.Unmarshal(stdout.data, &result, json.RejectUnknownMembers(true)); err != nil {
		return Result{}, Failure("source_output_invalid", "stdout must contain one source result JSON object")
	}
	if result.Items == nil {
		return Result{}, Failure("source_output_invalid", "items must be an array")
	}
	if err := ValidateResult(result, req.Limits); err != nil {
		return Result{}, err
	}
	return result, nil
}
func scriptEnv(allow []string, id, agent string) []string {
	names := append([]string{"PATH", "HOME", "LANG", "TMPDIR", "USER"}, allow...)
	env := []string{}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] || strings.ContainsAny(name, "=\x00") || strings.HasPrefix(name, "COMPOZY_") {
			continue
		}
		seen[name] = true
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return append(env, "COMPOZY_SOURCE_ID="+id, "COMPOZY_AGENT="+agent)
}

type boundedOutput struct {
	data     []byte
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := max(0, b.limit-len(b.data))
	b.data = append(b.data, p[:min(remaining, n)]...)
	if n > remaining {
		b.exceeded = true
		b.cancel()
	}
	return n, nil
}

type tailOutput struct{ data []byte }

func (b *tailOutput) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	if len(b.data) > 4096 {
		b.data = bytes.Clone(b.data[len(b.data)-4096:])
	}
	return len(p), nil
}

func (s *ScriptRunner) verifiedScript(ctx context.Context, name string) ([]byte, error) {
	data, hash, err := ReadScript(s.DefinitionDir, name)
	if err != nil {
		return nil, err
	}
	dir, err := filepath.EvalSymlinks(s.DefinitionDir)
	if err != nil {
		return nil, Failure("source_script_invalid", "invalid definition directory")
	}
	if s.Approvals == nil {
		return nil, Failure("source_script_unapproved", "script requires operator approval")
	}
	approved, err := s.Approvals.ApprovedHash(ctx, s.ProfileID, dir, name)
	if err != nil {
		return nil, err
	}
	if approved != hash {
		return nil, Failure("source_script_unapproved", "script requires operator approval")
	}

	return data, nil
}
