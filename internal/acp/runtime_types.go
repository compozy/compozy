package acp

import (
	"context"
	"io"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

// LaunchSpec describes the ACP-capable command to start locally.
type LaunchSpec struct {
	Command string
	// ResolvedExecutable and Args carry a launcher's prepared terminal identity.
	// A launcher that prepares them must execute this absolute identity without
	// resolving Command again.
	ResolvedExecutable string
	Args               []string
	Cwd                string
	AdditionalDirs     []string
	Env                []string
}

// Launcher starts an ACP-capable agent process locally.
type Launcher interface {
	Launch(ctx context.Context, spec LaunchSpec) (Handle, error)
}

// LaunchPreparer resolves one final launch specification inside the launcher's runtime.
type LaunchPreparer interface {
	PrepareLaunch(ctx context.Context, spec LaunchSpec) (LaunchSpec, error)
}

// CommandSpec identifies one already-resolved command in a runtime.
type CommandSpec struct {
	Executable string
	Args       []string
	Cwd        string
	Env        []string
}

// CommandResult reports one completed command without interpreting its output.
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// CommandRuntime resolves and runs diagnostic commands in one runtime.
type CommandRuntime interface {
	Resolve(ctx context.Context, command string, env []string, cwd string) (string, error)
	Run(ctx context.Context, spec CommandSpec) (CommandResult, error)
}

// Handle represents a running agent process.
type Handle interface {
	PID() int
	Cwd() string
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() string
	Done() <-chan struct{}
	Wait() error
	Stop(ctx context.Context) error
}

// PermissionOperation identifies a ToolHost operation subject to policy.
type PermissionOperation string

const (
	// PermissionOperationReadTextFile authorizes ACP text file reads.
	PermissionOperationReadTextFile PermissionOperation = "fs/read_text_file"
	// PermissionOperationWriteTextFile authorizes ACP text file writes.
	PermissionOperationWriteTextFile PermissionOperation = "fs/write_text_file"
	// PermissionOperationCreateTerminal authorizes terminal creation.
	PermissionOperationCreateTerminal PermissionOperation = "terminal/create"
	// PermissionOperationCloseTerminal authorizes closing a live terminal.
	PermissionOperationCloseTerminal PermissionOperation = "terminal/close"
	// PermissionOperationRequestToolGrant authorizes interactive permission requests.
	PermissionOperationRequestToolGrant PermissionOperation = "session/request_permission"
)

// PermissionDecision is a daemon policy decision for an ACP permission request.
type PermissionDecision string

const (
	// PermissionDecisionPending asks an operator or client to decide.
	PermissionDecisionPending PermissionDecision = "pending"
	// PermissionDecisionCanceled closes a request without an operator decision.
	PermissionDecisionCanceled PermissionDecision = "canceled"
	// PermissionDecisionAllowOnce permits one operation.
	PermissionDecisionAllowOnce PermissionDecision = "allow-once"
	// PermissionDecisionAllowAlways permits this class of operation persistently.
	PermissionDecisionAllowAlways PermissionDecision = "allow-always"
	// PermissionDecisionRejectOnce rejects one operation.
	PermissionDecisionRejectOnce PermissionDecision = "reject-once"
	// PermissionDecisionRejectAlways rejects this class of operation persistently.
	PermissionDecisionRejectAlways PermissionDecision = "reject-always"
)

// ToolHost abstracts ACP file, permission, and terminal operations for a runtime.
type ToolHost interface {
	ReadTextFile(ctx context.Context, path string) (string, error)
	WriteTextFile(ctx context.Context, path string, content string) error
	ResolvePath(path string) (string, error)
	Authorize(op PermissionOperation) error
	PermissionDecision(req acpsdk.RequestPermissionRequest) (PermissionDecision, bool)
	CreateTerminal(ctx context.Context, req acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error)
	KillTerminal(id string) error
	TerminalOutput(id string) (string, error)
	WaitForTerminalExit(ctx context.Context, id string) (int, error)
	ReleaseTerminal(id string) error
}

// ProcessSignal identifies one action in the session owner's termination ladder.
type ProcessSignal string

const (
	ProcessSignalCloseInput ProcessSignal = "close-input"
	ProcessSignalTerminate  ProcessSignal = "terminate"
	ProcessSignalKill       ProcessSignal = "kill"
)
