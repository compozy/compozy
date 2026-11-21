package procutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
)

func scanProcessDescendants(ctx context.Context, roots, watched []int) ([]ScannedProcess, bool, error) {
	clock, err := linuxProcessClock()
	if err != nil {
		return nil, true, err
	}
	rows, err := readLinuxProcessTree(ctx, os.DirFS("/proc"), clock, roots, watched)
	return rows, true, err
}

func readLinuxProcessTree(
	ctx context.Context, proc fs.FS, clock linuxClock, roots, watched []int,
) ([]ScannedProcess, error) {
	rows, err := readLinuxDescendants(ctx, proc, clock, roots, watched)
	if err == nil || ctx.Err() != nil {
		return rows, err
	}
	// A disappearing child can cause the kernel to omit its surviving siblings from the children file.
	return readLinuxProcesses(ctx, proc, clock, false)
}

type linuxChild struct{ pid, parent int }

func readLinuxDescendants(
	ctx context.Context, proc fs.FS, clock linuxClock, roots, watched []int,
) ([]ScannedProcess, error) {
	queue := make([]linuxChild, 0, len(roots))
	for _, pid := range roots {
		queue = append(queue, linuxChild{pid: pid})
	}
	rows := make([]ScannedProcess, 0, len(roots)+len(watched))
	seen := make(map[int]bool, cap(rows))
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		child := queue[0]
		queue = queue[1:]
		if child.pid <= 1 || seen[child.pid] {
			continue
		}
		seen[child.pid] = true
		p, err := readLinuxProcess(ctx, proc, child.pid, clock)
		if child.parent == 0 && errors.Is(err, ErrProcessIdentityGone) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if child.parent != 0 && p.PPID != child.parent {
			return nil, errors.New("process parent changed during scan")
		}
		rows = append(rows, p)
		if p.Zombie {
			continue
		}
		queue, err = appendLinuxChildren(ctx, proc, p.PID, queue)
		if err != nil {
			return nil, err
		}
	}
	for _, pid := range watched {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if pid <= 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		if p, err := readLinuxProcess(ctx, proc, pid, clock); err == nil {
			rows = append(rows, p)
		}
	}
	return rows, ctx.Err()
}

func appendLinuxChildren(ctx context.Context, proc fs.FS, parent int, queue []linuxChild) ([]linuxChild, error) {
	taskDir := strconv.Itoa(parent) + "/task"
	threads, err := fs.ReadDir(proc, taskDir)
	if err != nil {
		return nil, fmt.Errorf("read threads for PID %d: %w", parent, err)
	}
	for _, thread := range threads {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := fs.ReadFile(proc, taskDir+"/"+thread.Name()+"/children")
		if err != nil {
			return nil, fmt.Errorf("read children for thread %s: %w", thread.Name(), err)
		}
		for value := range bytes.FieldsSeq(data) {
			pid, err := strconv.Atoi(string(value))
			if err != nil {
				return nil, fmt.Errorf("parse child PID for thread %s: %w", thread.Name(), err)
			}
			queue = append(queue, linuxChild{pid: pid, parent: parent})
		}
	}
	return queue, nil
}
