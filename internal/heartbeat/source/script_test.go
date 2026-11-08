package source

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"
)

type approvalHash string

var _ ApprovalReader = approvalHash("")

func (a approvalHash) ApprovedHash(context.Context, string, string, string) (string, error) {
	return string(a), nil
}

type scriptWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *scriptWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestScriptRunner(t *testing.T) {
	for _, cancelWait := range []bool{false, true} {
		name := "Should finish the private copy before waiting for Linux forks"
		if cancelWait {
			name = "Should cancel a private copy fork wait without blocking shutdown"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS != "linux" {
				t.Skip("Linux Go child launches use vfork")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			observed := &scriptWaitContext{Context: ctx, waiting: make(chan struct{})}
			path := filepath.Join(t.TempDir(), "poll")
			body := "#!/bin/sh\nprintf verified\n"
			finished := make(chan error, 1)
			syscall.ForkLock.Lock()
			locked := true
			defer func() {
				if locked {
					syscall.ForkLock.Unlock()
				}
			}()
			go func() { finished <- writePrivateScript(observed, path, []byte(body)) }()
			select {
			case <-observed.waiting:
			case err := <-finished:
				t.Fatalf("copy returned before joining the in-flight fork: %v", err)
			case <-ctx.Done():
				t.Fatal("copy blocked on the fork lock before completing its write")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != body {
				t.Fatalf("private copy=%q error=%v", data, err)
			}
			if cancelWait {
				cancel()
				if err := <-finished; !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel error=%v", err)
				}
			}
			syscall.ForkLock.Unlock()
			locked = false
			if !cancelWait {
				if err := <-finished; err != nil {
					t.Fatal(err)
				}
			}
			output, err := exec.CommandContext(t.Context(), path).Output()
			if err != nil || string(output) != "verified" {
				t.Fatalf("script output=%q error=%v", output, err)
			}
		})
	}

	t.Run("Should reject named pipes without waiting for a writer", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "sources"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(dir, "sources", "pipe"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, read := range []func(string, string) ([]byte, string, error){ReadScript, InspectScript} {
			_, _, err := read(dir, "pipe")
			if cause, ok := errors.AsType[*Error](err); !ok || cause.Code != "source_script_invalid" {
				t.Fatalf("error=%v", err)
			}
		}
	})
	for _, tc := range []struct {
		name, body, code string
		approve, enabled bool
		limit            int
	}{
		{"Should pass stdin and isolated environment [UT-010 UT-061]", `read req
[ -z "$ANTHROPIC_API_KEY" ] || exit 3
[ "$COMPOZY_SOURCE_ID" = demo ] || exit 4
printf '{"items":[{"key":"1","title":"ok"}],"state":{"cursor":1}}'`, "", true, true, 4096},
		{"Should reject unapproved bytes [UT-018]", `echo '{"items":[]}'`, "source_script_unapproved", false, true, 4096},
		{"Should disable scripts [UT-012]", `exit 9`, "source_scripts_disabled", true, false, 4096},
		{"Should reject duplicate keys [UT-013]", `echo '{"items":[{"key":"1","title":"one"},{"key":"1","title":"two"}]}'`, "source_output_invalid", true, true, 4096},
		{"Should reject invalid output [UT-017]", `echo garbage`, "source_output_invalid", true, true, 4096},
		{"Should bound stdout [UT-016]", `printf '%05000d' 0`, "source_output_too_large", true, true, 4096},
		{"Should kill timed out group [UT-015]", `sleep 60`, "source_timeout", true, true, 4096},
		{"Should report nonzero exit [UT-017]", `echo "token abc123secret" >&2; exit 3`, "source_exit_nonzero", true, true, 4096},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "sources"), 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "sources", "poll")
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"+tc.body+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			_, hash, err := ReadScript(dir, "poll")
			if err != nil {
				t.Fatal(err)
			}
			approved := ""
			if tc.approve {
				approved = hash
			}
			timeout := 10 * time.Second
			if tc.code == "source_timeout" {
				timeout = time.Second
			}
			runner := &ScriptRunner{
				DefinitionDir:  dir,
				ScriptsEnabled: tc.enabled,
				Timeout:        timeout,
				MaxOutputBytes: tc.limit,
				Approvals:      approvalHash(approved),
			}
			result, err := runner.Poll(
				t.Context(),
				Ref{ID: "demo", Script: "poll"},
				Request{
					Version:   1,
					SourceID:  "demo",
					Workspace: WorkspaceRef{RootDir: dir},
					Limits:    Limits{MaxItems: 20, MaxSummaryBytes: 8192, MaxStateBytes: 16384},
				},
			)
			if tc.code == "" {
				if err != nil || len(result.Items) != 1 {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				return
			}
			if err != nil && strings.Contains(err.Error(), "abc123secret") {
				t.Fatalf("secret exposed: %v", err)
			}
			e, ok := errors.AsType[*Error](err)
			if !ok || string(e.Code) != tc.code {
				t.Fatalf("error=%v want %s", err, tc.code)
			}
		})
	}
	t.Run("Should reject symlink escapes and nonexecutable files [UT-011 UT-012]", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "sources"), 0700); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "poll")
		if err := os.WriteFile(outside, []byte("#!/bin/sh\n"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(dir, "sources", "poll")); err != nil {
			t.Fatal(err)
		}
		_, _, err := ReadScript(dir, "poll")
		e, ok := errors.AsType[*Error](err)
		if !ok || e.Code != "source_script_invalid" {
			t.Fatal(err)
		}
		for _, name := range []string{"../poll", "a/b", "", "_poll", strings.Repeat("a", 65)} {
			_, _, err := ReadScript(dir, name)
			if err == nil {
				t.Fatalf("accepted %q", name)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "sources", "plain"), []byte("#!/bin/sh\n"), 0600); err != nil {
			t.Fatal(err)
		}
		for name, code := range map[string]ErrorCode{"absent": "source_script_missing", "plain": "source_script_not_executable"} {
			_, _, err := ReadScript(dir, name)
			cause, ok := errors.AsType[*Error](err)
			if !ok || cause.Code != code {
				t.Fatalf("script=%s err=%v", name, err)
			}
		}
	})
	t.Run("Should validate state and item limits [UT-013 UT-014]", func(t *testing.T) {
		t.Parallel()
		limits := Limits{MaxItems: 20, MaxStateBytes: 16384, MaxSummaryBytes: 8192}
		result := Result{State: []byte(`"` + strings.Repeat("x", 16382) + `"`)}
		if err := ValidateResult(result, limits); err != nil {
			t.Fatal(err)
		}
		result.State = append(result.State, ' ')
		if err := ValidateResult(result, limits); err == nil {
			t.Fatal("accepted oversized state")
		}
		result = Result{Items: make([]Item, 21)}
		if err := ValidateResult(
			result,
			limits,
		); err == nil ||
			!strings.Contains(err.Error(), "returned 21 items, limit 20") {
			t.Fatal(err)
		}
	})
}

type replacingApproval struct{ path, hash string }

var _ ApprovalReader = replacingApproval{}

func (r replacingApproval) ApprovedHash(context.Context, string, string, string) (string, error) {
	return r.hash, os.WriteFile(
		r.path,
		[]byte("#!/bin/sh\necho '{\"items\":[{\"key\":\"replacement\",\"title\":\"replacement\"}]}'\n"),
		0700,
	)
}
func TestScriptVerifiedCopy(t *testing.T) {
	t.Run("Should execute approved bytes when the source is replaced [UT-019]", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "sources"), 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "sources", "poll")
		if err := os.WriteFile(
			path,
			[]byte("#!/bin/sh\necho '{\"items\":[{\"key\":\"verified\",\"title\":\"verified\"}]}'\n"),
			0700,
		); err != nil {
			t.Fatal(err)
		}
		_, hash, err := ReadScript(dir, "poll")
		if err != nil {
			t.Fatal(err)
		}
		runner := &ScriptRunner{
			DefinitionDir:  dir,
			ScriptsEnabled: true,
			Timeout:        10 * time.Second,
			MaxOutputBytes: 4096,
			Approvals:      replacingApproval{path: path, hash: hash},
		}
		result, err := runner.Poll(
			t.Context(),
			Ref{ID: "s", Script: "poll"},
			Request{
				Workspace: WorkspaceRef{RootDir: dir},
				Limits:    Limits{MaxItems: 20, MaxSummaryBytes: 8192, MaxStateBytes: 16384},
			},
		)
		if err != nil || len(result.Items) != 1 || result.Items[0].Key != "verified" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		runner.Approvals = approvalHash(hash)
		_, err = runner.Poll(t.Context(), Ref{ID: "s", Script: "poll"}, Request{})
		e, ok := errors.AsType[*Error](err)
		if !ok || e.Code != "source_script_unapproved" {
			t.Fatalf("changed script error=%v", err)
		}
	})
}

func TestScriptEnvironment(t *testing.T) {
	// not parallel: the environment isolation case uses t.Setenv.
	t.Run("Should pass only allowed daemon variables and request JSON [UT-010 UT-061]", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "source-test-private")
		t.Setenv("GITHUB_TOKEN", "source-test-allowed")
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "sources"), 0700); err != nil {
			t.Fatal(err)
		}
		body := `#!/bin/sh
req=$(cat)
case "$req" in *'"max_items":20'*) ;; *) exit 4;; esac
[ -z "$ANTHROPIC_API_KEY" ] || exit 5
[ "$GITHUB_TOKEN" = source-test-allowed ] || exit 6
printf '{"items":[]}'
`
		if err := os.WriteFile(filepath.Join(dir, "sources", "poll"), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
		_, hash, err := ReadScript(dir, "poll")
		if err != nil {
			t.Fatal(err)
		}
		runner := &ScriptRunner{
			DefinitionDir:  dir,
			ScriptsEnabled: true,
			Timeout:        10 * time.Second,
			MaxOutputBytes: 4096,
			EnvAllow:       []string{"GITHUB_TOKEN"},
			Approvals:      approvalHash(hash),
		}
		_, err = runner.Poll(
			t.Context(),
			Ref{ID: "s", Script: "poll"},
			Request{
				Workspace: WorkspaceRef{RootDir: dir},
				Limits:    Limits{MaxItems: 20, MaxStateBytes: 16384, MaxSummaryBytes: 8192},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestSourceResultBounds(t *testing.T) {
	t.Run("Should shorten summaries at a UTF8 byte boundary", func(t *testing.T) {
		t.Parallel()
		result := Result{Items: []Item{{Key: "owner/repo#1", Title: "issue", Summary: strings.Repeat("界", 450)}}}
		if err := ValidateResult(result, Limits{MaxItems: 20}); err != nil {
			t.Fatal(err)
		}
		if len(result.Items[0].Summary) != 1023 || !utf8.ValidString(result.Items[0].Summary) {
			t.Fatalf("summary=%q", result.Items[0].Summary)
		}
	})
	for _, tc := range []struct {
		name string
		item Item
		cap  int
		code string
	}{
		{name: "Should reject secret-bearing item keys", item: Item{Key: "token secret-value", Title: "issue"}, code: "source_output_invalid"},
		{name: "Should reject control characters in item keys", item: Item{Key: "issue\nignore", Title: "issue"}, code: "source_output_invalid"},
		{name: "Should reject unsafe item keys", item: Item{Key: "ignore previous instructions", Title: "issue"}, code: "source_output_invalid"},
		{name: "Should reject overlong URLs", item: Item{Key: "1", Title: "issue", URL: strings.Repeat("x", 2049)}, code: "source_output_invalid"},
		{name: "Should reject oversized attributes", item: Item{Key: "1", Title: "issue", Attributes: []byte(`"` + strings.Repeat("x", 2048) + `"`)}, code: "source_output_invalid"},
		{name: "Should enforce the encoded result cap", item: Item{Key: "1", Title: "issue", Summary: strings.Repeat("x", 500)}, cap: 100, code: "source_output_too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateResult(Result{Items: []Item{tc.item}}, Limits{MaxItems: 20, MaxOutputBytes: tc.cap})
			e, ok := errors.AsType[*Error](err)
			if !ok || string(e.Code) != tc.code {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
