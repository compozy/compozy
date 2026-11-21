package procutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"
)

func TestProcessToken(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, stat string
		readErr    error
		want       string
		wantErr    string
	}{
		{name: "Should parse ticks after a command containing parentheses", stat: "42 (a ) b) S " + strings.Repeat("0 ", 18) + "98765 0", want: "boot-id:98765"},
		{name: "Should reject a truncated process stat", stat: "42 (a) S 0", wantErr: "invalid process identity"},
		{name: "Should reject malformed ticks", stat: "42 (a) S " + strings.Repeat("0 ", 18) + "oops", wantErr: "invalid process start ticks"},
		{name: "Should reject a vanished process", readErr: os.ErrNotExist, wantErr: "read process identity"},
		{name: "Should recognize a process exiting during stat read", readErr: syscall.ESRCH, wantErr: "read process identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := processToken(42, func(path string) ([]byte, error) {
				if path == "/proc/sys/kernel/random/boot_id" {
					return []byte("boot-id\n"), nil
				}
				if tc.readErr != nil {
					return nil, tc.readErr
				}
				return []byte(tc.stat), nil
			})
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("token = %q, %v", got, err)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if tc.readErr != nil && (!errors.Is(err, tc.readErr) || !errors.Is(err, ErrProcessIdentityGone)) {
				t.Fatal(err)
			}
		})
	}
}

// Invariants: the Linux proc reader discovers descendants without reading unrelated processes,
// and a child disappearing during enumeration cannot hide a surviving sibling.
// Owning layer and canonical suite: procutil Linux process inspection at the filesystem boundary.
func TestLinuxProcessTree(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should read grandchildren across threads and keep watched PIDs without scanning the host",
		func(t *testing.T) {
			t.Parallel()
			proc := scopedProcFixture("101")
			rows, err := readLinuxProcessTree(
				t.Context(),
				proc,
				linuxClock{boot: 100, ticks: 100},
				[]int{100},
				[]int{200},
			)
			if err != nil {
				t.Fatal(err)
			}
			if got := scannedPIDs(rows); !slices.Equal(got, []int{100, 101, 102, 200}) {
				t.Fatalf("PIDs = %v, want leader, child, grandchild and detached observation", got)
			}
			if slices.Contains(proc.reads, "900/stat") {
				t.Fatalf("read unrelated process: %v", proc.reads)
			}
		},
	)
	t.Run("Should find a surviving descendant when a listed sibling exits before its stat read", func(t *testing.T) {
		t.Parallel()
		proc := scopedProcFixture("103") // 103 disappeared and 101 was omitted from the children file.
		rows, err := readLinuxProcessTree(t.Context(), proc, linuxClock{boot: 100, ticks: 100}, []int{100}, nil)
		if err != nil {
			t.Fatal(err)
		}
		got := scannedPIDs(rows)
		if !slices.Contains(got, 102) || slices.Contains(got, 103) {
			t.Fatalf("PIDs = %v, want the surviving grandchild and no vanished process", got)
		}
	})
}

type recordedProcFS struct {
	fs.FS
	reads []string
}

func (p *recordedProcFS) ReadFile(name string) ([]byte, error) {
	p.reads = append(p.reads, name)
	return fs.ReadFile(p.FS, name)
}

func scopedProcFixture(children string) *recordedProcFS {
	files := fstest.MapFS{
		"100/task/100/children": {},
		"100/task/110/children": {Data: []byte(children)},
		"101/task/101/children": {Data: []byte("102")},
		"102/task/102/children": {},
	}
	for pid, parent := range map[int]int{100: 1, 101: 100, 102: 101, 200: 1, 900: 1} {
		stat := fmt.Sprintf("%d (task) S %d 100 %s100", pid, parent, strings.Repeat("0 ", 16))
		files[fmt.Sprintf("%d/stat", pid)] = &fstest.MapFile{Data: []byte(stat)}
	}
	return &recordedProcFS{FS: files}
}

func scannedPIDs(rows []ScannedProcess) []int {
	pids := make([]int, 0, len(rows))
	for _, row := range rows {
		pids = append(pids, row.PID)
	}
	slices.Sort(pids)
	return pids
}

// Invariant: scanner and durable token reads identify the same live kernel process.
// Owner/canonical suite: procutil Linux identity.
func TestScannedKernelToken(t *testing.T) {
	t.Parallel()
	t.Run("Should keep the kernel token stable when the boot projection shifts", func(t *testing.T) {
		clock, err := linuxProcessClock()
		if err != nil {
			t.Fatal(err)
		}
		original, err := linuxProcess(os.Getpid(), clock)
		if err != nil {
			t.Fatal(err)
		}
		token, err := ProcessToken(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		clock.boot += 3600
		shifted, err := linuxProcess(os.Getpid(), clock)
		if err != nil {
			t.Fatal(err)
		}
		if original.KernelToken != token || shifted.KernelToken != token ||
			original.StartedAt.Equal(shifted.StartedAt) {
			t.Fatalf("original=%+v shifted=%+v token=%q", original, shifted, token)
		}
	})
}

// Invariant: a transient clock read failure must not permanently disable process scans.
// Owner/canonical suite: procutil Linux identity; load models the proc filesystem I/O boundary.
func TestLinuxProcessClock(t *testing.T) {
	t.Parallel()
	t.Run("Should retry failed reads and retain the first successful projection", func(t *testing.T) {
		var cache linuxClockCache
		calls := 0
		want := linuxClock{boot: 100, ticks: 100, bootID: "boot-id"}
		load := func() (linuxClock, error) {
			calls++
			if calls == 1 {
				return linuxClock{}, os.ErrPermission
			}
			return want, nil
		}
		if _, err := cache.get(load); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("first read: %v", err)
		}
		for range 2 {
			if got, err := cache.get(load); err != nil || got != want {
				t.Fatalf("clock=%+v error=%v", got, err)
			}
		}
		if calls != 2 {
			t.Fatalf("loads=%d, want 2", calls)
		}
	})
}
