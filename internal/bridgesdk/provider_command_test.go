package bridgesdk

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestRunProviderCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should serve the default and explicit commands through one shared runner", func(t *testing.T) {
		t.Parallel()

		calls := 0
		serve := func(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
			calls++
			if stdin == nil || stdout == nil || stderr == nil {
				t.Fatal("provider streams must remain available")
			}
			return nil
		}
		for _, args := range [][]string{nil, {"serve"}} {
			err := RunProviderCommand(
				"slack",
				args,
				bytes.NewReader(nil),
				io.Discard,
				io.Discard,
				serve,
			)
			if err != nil {
				t.Fatalf("RunProviderCommand(%v) error = %v", args, err)
			}
		}
		if calls != 2 {
			t.Fatalf("serve calls = %d, want 2", calls)
		}
	})

	t.Run("Should preserve provider-specific unsupported command diagnostics", func(t *testing.T) {
		t.Parallel()

		err := RunProviderCommand(
			"telegram",
			[]string{"verify"},
			bytes.NewReader(nil),
			io.Discard,
			io.Discard,
			func(io.Reader, io.Writer, io.Writer) error { return errors.New("must not run") },
		)
		if got, want := err.Error(), `telegram: unsupported command "verify"`; got != want {
			t.Fatalf("RunProviderCommand() error = %q, want %q", got, want)
		}
	})
}
