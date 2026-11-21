package procutil

import (
	"bytes"
	"context"
	"strings"
	"time"
)

// ScannedProcess retains only attribution markers, never the complete environment.
type ScannedProcess struct {
	PID, PPID, PGID                     int
	StartedAt                           time.Time
	KernelToken                         string
	SessionID, Home, Profile, Workspace string
	Argv0, Command, CWD                 string
	Ports                               []int
	Zombie                              bool
}

type ProcessSnapshot struct {
	Supported bool
	ScannedAt time.Time
	Duration  time.Duration
	Processes []ScannedProcess
}

func ScanProcesses(ctx context.Context) (ProcessSnapshot, error) {
	start := time.Now()
	processes, supported, err := scanProcesses(ctx, true)
	return ProcessSnapshot{
		Supported: supported,
		ScannedAt: start.UTC(),
		Duration:  time.Since(start),
		Processes: processes,
	}, err
}

func processMarkers(p *ScannedProcess, env []byte) {
	for entry := range bytes.SplitSeq(env, []byte{0}) {
		key, value, found := bytes.Cut(entry, []byte{'='})
		if !found {
			continue
		}
		switch string(key) {
		case "COMPOZY_SESSION_ID":
			p.SessionID = string(value)
		case "COMPOZY_HOME":
			p.Home = string(value)
		case "COMPOZY_PROFILE_ID":
			p.Profile = string(value)
		case "COMPOZY_WORKSPACE_ID":
			p.Workspace = string(value)
		}
	}
}

// ScanProcessTree reads host-wide lineage without loading process environments.
func ScanProcessTree(ctx context.Context) (ProcessSnapshot, error) {
	start := time.Now()
	rows, supported, err := scanProcesses(ctx, false)
	return ProcessSnapshot{
		Supported: supported,
		ScannedAt: start.UTC(),
		Duration:  time.Since(start),
		Processes: rows,
	}, err
}

// ScanProcessDescendants reads the roots, their descendants, and individually watched PIDs.
func ScanProcessDescendants(ctx context.Context, roots, watched []int) (ProcessSnapshot, error) {
	start := time.Now()
	rows, supported, err := scanProcessDescendants(ctx, roots, watched)
	return ProcessSnapshot{
		Supported: supported,
		ScannedAt: start.UTC(),
		Duration:  time.Since(start),
		Processes: rows,
	}, err
}

// EnrichProcesses adds best-effort cwd and listener metadata to already attributed rows.
func EnrichProcesses(ctx context.Context, rows []ScannedProcess) {
	original := make([]struct{ argv0, command string }, len(rows))
	for i := range rows {
		original[i].argv0, original[i].command = rows[i].Argv0, rows[i].Command
	}
	enrichProcesses(ctx, rows)
	for i, p := range rows {
		current, err := scanProcessIdentity(p.PID)
		if err != nil || !current.StartedAt.Equal(p.StartedAt) {
			rows[i].Argv0, rows[i].Command = original[i].argv0, original[i].command
			rows[i].CWD = ""
			rows[i].Ports = []int{}
		}
	}
}

func InspectProcess(pid int) (ScannedProcess, error) { return scanProcessIdentity(pid) }

func processArguments(p *ScannedProcess, args []string) {
	if len(args) == 0 {
		return
	}
	p.Argv0 = args[0]
	p.Command = strings.Join(args, " ")
}
