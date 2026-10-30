package procutil

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/execabs"
	"golang.org/x/sys/unix"
)

func scanProcesses(ctx context.Context, environment bool) ([]ScannedProcess, bool, error) {
	rows, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, true, fmt.Errorf("scan process table: %w", err)
	}
	result := make([]ScannedProcess, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		if err := ctx.Err(); err != nil {
			return nil, true, err
		}
		p := darwinProcess(row)
		// Retain inaccessible members too: a group signal requires proven ownership of every member.
		if environment && int(row.Eproc.Ucred.Uid) == os.Getuid() {
			raw, readErr := unix.SysctlRaw("kern.procargs2", p.PID)
			if readErr == nil {
				darwinArguments(&p, raw)
			}
		}
		result = append(result, p)
	}
	return result, true, nil
}

func darwinProcess(row *unix.KinfoProc) ScannedProcess {
	return ScannedProcess{PID: int(row.Proc.P_pid), PPID: int(row.Eproc.Ppid), PGID: int(row.Eproc.Pgid),
		StartedAt: time.Unix(row.Proc.P_starttime.Sec, int64(row.Proc.P_starttime.Usec)*1000).UTC(),
		Argv0:     unix.ByteSliceToString(row.Proc.P_comm[:]), Zombie: row.Proc.P_stat == 5, Ports: []int{}}
}

func darwinArguments(p *ScannedProcess, raw []byte) {
	if len(raw) < 4 {
		return
	}
	argc := int(binary.NativeEndian.Uint32(raw[:4]))
	raw = raw[4:]
	end := bytes.IndexByte(raw, 0)
	if end < 0 || argc < 1 {
		return
	}
	raw = bytes.TrimLeft(raw[end+1:], "\x00")
	args := make([]string, 0, argc)
	for range argc {
		end = bytes.IndexByte(raw, 0)
		if end < 0 {
			return
		}
		args = append(args, string(raw[:end]))
		raw = raw[end+1:]
	}
	processArguments(p, args)
	processMarkers(p, strings.Split(string(raw), "\x00"))
}

func scanProcessIdentity(pid int) (ScannedProcess, error) {
	row, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if errors.Is(err, unix.ESRCH) || (err != nil && errors.Is(unix.Kill(pid, 0), unix.ESRCH)) ||
		(err == nil && row.Proc.P_pid == 0) {
		return ScannedProcess{}, ErrProcessIdentityGone
	}
	if err != nil {
		return ScannedProcess{}, err
	}
	return darwinProcess(row), nil
}

// lsof is a bounded, best-effort fallback for cwd and TCP listeners without a cgo dependency.
func darwinProcessFiles(ctx context.Context, rows []ScannedProcess) {
	ids := make([]string, 0)
	byPID := make(map[int]int)
	for i, p := range rows {
		if p.SessionID != "" {
			ids = append(ids, strconv.Itoa(p.PID))
			byPID[p.PID] = i
		}
	}
	if len(ids) == 0 {
		return
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := execabs.CommandContext(lookupCtx, "/usr/sbin/lsof", "-nP", "-a", "-p", strings.Join(ids, ","), "-FpcftnT")
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return
	}
	parseDarwinProcessFiles(output, rows, byPID)
}

func parseDarwinProcessFiles(output []byte, rows []ScannedProcess, byPID map[int]int) {
	index := -1
	fd := ""
	port := 0
	listening := false
	flush := func() {
		if index >= 0 && port > 0 && listening {
			rows[index].Ports = append(rows[index].Ports, port)
		}
		port = 0
		listening = false
	}
	for line := range strings.SplitSeq(string(output), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			flush()
			pid, e := strconv.Atoi(line[1:])
			index = -1
			if i, ok := byPID[pid]; e == nil && ok {
				index = i
			}
		case 'f':
			flush()
			fd = line[1:]
		case 'n':
			if index < 0 {
				continue
			}
			if fd == "cwd" {
				rows[index].CWD = line[1:]
				continue
			}
			if _, value, ok := strings.CutLast(line, ":"); ok {
				if n, e := strconv.Atoi(value); e == nil {
					port = n
				}
			}
		case 'T':
			if line == "TST=LISTEN" {
				listening = true
			}
		}
	}
	flush()
}

func enrichProcesses(ctx context.Context, rows []ScannedProcess) { darwinProcessFiles(ctx, rows) }
