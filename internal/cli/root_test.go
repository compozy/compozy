package cli

import (
	"reflect"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

var cliStateTestMu sync.Mutex

func lockCLIStateForTest(t *testing.T) {
	t.Helper()
	cliStateTestMu.Lock()
	t.Cleanup(cliStateTestMu.Unlock)
}

func TestBuildCLIArgsIncludesAutoCommit(t *testing.T) {
	lockCLIStateForTest(t)

	origAutoCommit := autoCommit
	origTimeout := timeout
	origAddDirs := append([]string(nil), addDirs...)
	t.Cleanup(func() {
		autoCommit = origAutoCommit
		timeout = origTimeout
		addDirs = origAddDirs
	})

	autoCommit = false
	timeout = "10m"
	addDirs = []string{"../shared", "../docs", "../shared"}
	args := buildCLIArgs()
	if args.AutoCommit {
		t.Fatalf("expected AutoCommit=false in cli args")
	}
	if !reflect.DeepEqual(args.AddDirs, []string{"../shared", "../docs"}) {
		t.Fatalf("expected normalized addDirs in cli args, got %#v", args.AddDirs)
	}

	autoCommit = true
	args = buildCLIArgs()
	if !args.AutoCommit {
		t.Fatalf("expected AutoCommit=true in cli args")
	}
}

func TestApplyStringSliceInputParsesAddDirsFromFormValue(t *testing.T) {
	lockCLIStateForTest(t)

	origAddDirs := append([]string(nil), addDirs...)
	t.Cleanup(func() {
		addDirs = origAddDirs
	})

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringSlice("add-dir", nil, "test add-dir")

	fi := &formInputs{
		addDirs: " ../shared, ../docs ,, ../shared \n ../workspace ",
	}

	fi.apply(cmd)

	want := []string{"../shared", "../docs", "../workspace"}
	if !reflect.DeepEqual(addDirs, want) {
		t.Fatalf("unexpected addDirs from form\nwant: %#v\ngot:  %#v", want, addDirs)
	}
}
