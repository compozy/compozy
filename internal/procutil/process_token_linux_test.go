package procutil

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestProcessToken(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, stat string
		missing    bool
		want       string
		wantErr    string
	}{
		{name: "Should parse ticks after a command containing parentheses", stat: "42 (a ) b) S " + strings.Repeat("0 ", 18) + "98765 0", want: "boot-id:98765"},
		{name: "Should reject a truncated process stat", stat: "42 (a) S 0", wantErr: "invalid process identity"},
		{name: "Should reject malformed ticks", stat: "42 (a) S " + strings.Repeat("0 ", 18) + "oops", wantErr: "invalid process start ticks"},
		{name: "Should reject a vanished process", missing: true, wantErr: "read process identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := processToken(42, func(path string) ([]byte, error) {
				if path == "/proc/sys/kernel/random/boot_id" {
					return []byte("boot-id\n"), nil
				}
				if tc.missing {
					return nil, os.ErrNotExist
				}
				return []byte(tc.stat), nil
			})
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("token = %q, %v", got, err)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if tc.missing && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		})
	}
}
