package procutil

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func scanProcesses(ctx context.Context, environment bool) ([]ScannedProcess, bool, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, true, err
	}
	clock, err := linuxProcessClock()
	if err != nil {
		return nil, true, err
	}
	rows := make([]ScannedProcess, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, true, err
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		p, err := linuxProcess(pid, clock)
		if err != nil {
			continue
		}
		root := fmt.Sprintf("/proc/%d", pid)
		if environment {
			if env, e := os.ReadFile(filepath.Join(root, "environ")); e == nil {
				processMarkers(&p, strings.Split(string(env), "\x00"))
			}
		}

		rows = append(rows, p)
	}
	return rows, true, nil
}

type linuxClock struct {
	boot, ticks int64
	bootID      string
}

// Keep the wall-time projection stable for the daemon lifetime; kernel tokens
// preserve identity when a later daemon starts after a wall-clock correction.
var processClockCache linuxClockCache

type linuxClockCache struct {
	mu    sync.Mutex
	clock linuxClock
}

func (c *linuxClockCache) get(load func() (linuxClock, error)) (linuxClock, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.clock.ticks > 0 {
		return c.clock, nil
	}
	clock, err := load()
	if err != nil {
		return linuxClock{}, err
	}
	c.clock = clock
	return clock, nil
}

func linuxProcessClock() (linuxClock, error) {
	return processClockCache.get(loadLinuxClock)
}

func loadLinuxClock() (linuxClock, error) {
	boot, ticks, err := readLinuxProcessClock()
	if err != nil {
		return linuxClock{}, err
	}
	id, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return linuxClock{}, err
	}
	if strings.TrimSpace(string(id)) == "" {
		return linuxClock{}, errors.New("empty boot identity")
	}
	return linuxClock{boot: boot, ticks: ticks, bootID: strings.TrimSpace(string(id))}, nil
}

func readLinuxProcessClock() (int64, int64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	var boot int64
	for line := range strings.SplitSeq(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "btime "); ok {
			boot, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, 0, err
			}
			break
		}
	}
	aux, err := os.ReadFile("/proc/self/auxv")
	if err != nil {
		return 0, 0, err
	}
	width := strconv.IntSize / 8
	for len(aux) >= width*2 {
		var key, value uint64
		if width == 8 {
			key = binary.NativeEndian.Uint64(aux)
			value = binary.NativeEndian.Uint64(aux[width:])
		} else {
			key = uint64(binary.NativeEndian.Uint32(aux))
			value = uint64(binary.NativeEndian.Uint32(aux[width:]))
		}
		if key == 17 && value > 0 && value <= math.MaxInt64 && boot > 0 {
			return boot, int64(value), nil
		}
		aux = aux[width*2:]
	}
	return 0, 0, errors.New("process clock unavailable")
}

func linuxProcess(pid int, clock linuxClock) (ScannedProcess, error) {
	boot, ticks := clock.boot, clock.ticks
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if errors.Is(err, os.ErrNotExist) {
		return ScannedProcess{}, ErrProcessIdentityGone
	}
	if err != nil {
		return ScannedProcess{}, err
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return ScannedProcess{}, errors.New("invalid process stat")
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 {
		return ScannedProcess{}, errors.New("incomplete process stat")
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return ScannedProcess{}, err
	}
	pgid, err := strconv.Atoi(fields[2])
	if err != nil {
		return ScannedProcess{}, err
	}
	start, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return ScannedProcess{}, err
	}
	return ScannedProcess{
		PID:         pid,
		KernelToken: clock.bootID + ":" + fields[19],
		PPID:        ppid,
		PGID:        pgid,
		StartedAt:   time.Unix(boot+start/ticks, (start%ticks)*int64(time.Second)/ticks).UTC(),
		Zombie:      fields[0] == "Z",
		Ports:       []int{},
	}, nil
}

func scanProcessIdentity(pid int) (ScannedProcess, error) {
	clock, err := linuxProcessClock()
	if err != nil {
		return ScannedProcess{}, err
	}
	return linuxProcess(pid, clock)
}

func linuxListeners() map[string]int {
	ports := make(map[string]int)
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 10 || fields[3] != "0A" {
				continue
			}
			_, hex, ok := strings.Cut(fields[1], ":")
			if !ok {
				continue
			}
			port, err := strconv.ParseInt(hex, 16, 32)
			if err == nil {
				ports[fields[9]] = int(port)
			}
		}
	}
	return ports
}

func linuxProcessPorts(root string, listeners map[string]int) []int {
	ports := []int{}
	entries, err := os.ReadDir(filepath.Join(root, "fd"))
	if err != nil {
		return ports
	}
	for _, entry := range entries {
		link, err := os.Readlink(filepath.Join(root, "fd", entry.Name()))
		if err != nil {
			continue
		}
		if inode, ok := strings.CutPrefix(link, "socket:["); ok {
			if port, ok := listeners[strings.TrimSuffix(inode, "]")]; ok {
				ports = append(ports, port)
			}
		}
	}
	return ports
}

func enrichProcesses(ctx context.Context, rows []ScannedProcess) {
	listeners := linuxListeners()
	for i := range rows {
		if ctx.Err() != nil {
			return
		}
		root := fmt.Sprintf("/proc/%d", rows[i].PID)
		if cmd, e := os.ReadFile(filepath.Join(root, "cmdline")); e == nil {
			processArguments(&rows[i], strings.Split(strings.TrimRight(string(cmd), "\x00"), "\x00"))
		}
		if cwd, e := os.Readlink(filepath.Join(root, "cwd")); e == nil {
			rows[i].CWD = cwd
		}
		rows[i].Ports = linuxProcessPorts(root, listeners)
	}
}
