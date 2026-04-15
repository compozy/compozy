package sound

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolvePath(t *testing.T) {
	t.Parallel()

	// Use t.TempDir() + filepath.Join() so the fixture is portable across
	// platforms — per CONTRIBUTING.md "Use t.TempDir() for filesystem isolation".
	absPath := filepath.Join(t.TempDir(), "custom.aiff")
	cases := []struct {
		name    string
		input   string
		wantAbs string
	}{
		{
			name:    "absolute path is returned verbatim",
			input:   absPath,
			wantAbs: absPath,
		},
		{
			name:    "trims surrounding whitespace on absolute path",
			input:   "  " + absPath + "  ",
			wantAbs: absPath,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolvePath(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantAbs {
				t.Fatalf("unexpected path: got %q, want %q", got, tc.wantAbs)
			}
		})
	}
}

func TestResolvePath_EmptyInputReturnsErrEmptySound(t *testing.T) {
	t.Parallel()
	cases := []string{"", "   ", "\t\n"}
	for _, input := range cases {
		input := input
		t.Run("input "+input, func(t *testing.T) {
			t.Parallel()
			_, err := ResolvePath(input)
			if !errors.Is(err, ErrEmptySound) {
				t.Fatalf("expected ErrEmptySound, got %v", err)
			}
		})
	}
}

func TestResolvePath_UnknownPresetErrorMessage(t *testing.T) {
	t.Parallel()
	const unknown = "nonsense-preset-xyz"

	_, err := ResolvePath(unknown)
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}

	msg := err.Error()
	if !strings.Contains(msg, unknown) {
		t.Errorf("error message should surface the unknown input %q, got: %q", unknown, msg)
	}
	// The error should list the known presets so users can spot typos against
	// the canonical list. Assert a couple of known presets are named.
	for _, expected := range []string{PresetGlass, PresetBasso} {
		if !strings.Contains(msg, expected) {
			t.Errorf("error message should list known preset %q, got: %q", expected, msg)
		}
	}
	if !strings.Contains(msg, "absolute path") {
		t.Errorf("error message should hint at absolute-path escape hatch, got: %q", msg)
	}
}

func TestResolvePath_KnownPresetForCurrentOS(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skipf("no preset table for %s", runtime.GOOS)
	}

	got, err := ResolvePath(PresetGlass)
	if err != nil {
		t.Fatalf("unexpected error resolving glass preset on %s: %v", runtime.GOOS, err)
	}
	if got == "" {
		t.Fatal("expected non-empty path for glass preset")
	}
}

func TestPresetPathForOS(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		preset   string
		goos     string
		wantOK   bool
		wantPath string
	}{
		{
			name:     "Should resolve darwin glass preset to System Sounds",
			preset:   PresetGlass,
			goos:     "darwin",
			wantOK:   true,
			wantPath: "/System/Library/Sounds/Glass.aiff",
		},
		{
			name:     "Should resolve linux basso preset to freedesktop error sound",
			preset:   PresetBasso,
			goos:     "linux",
			wantOK:   true,
			wantPath: "/usr/share/sounds/freedesktop/stereo/dialog-error.oga",
		},
		{
			name:   "Should return false for unknown preset",
			preset: "not-a-preset",
			goos:   "darwin",
			wantOK: false,
		},
		{
			name:   "Should return false for unsupported goos",
			preset: PresetGlass,
			goos:   "plan9",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := presetPathForOS(tc.preset, tc.goos)
			if ok != tc.wantOK {
				t.Fatalf("ok mismatch: got %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if got != tc.wantPath {
				t.Fatalf("unexpected path: got %q, want %q", got, tc.wantPath)
			}
		})
	}
}

func TestWindowsPresetPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		preset  string
		windir  string
		wantOK  bool
		wantOut string
	}{
		{
			name:    "Should resolve glass under the provided WINDIR",
			preset:  PresetGlass,
			windir:  `D:\WinRoot`,
			wantOK:  true,
			wantOut: `D:\WinRoot\Media\tada.wav`,
		},
		{
			name:    "Should trim trailing backslash from WINDIR",
			preset:  PresetPing,
			windir:  `D:\WinRoot\`,
			wantOK:  true,
			wantOut: `D:\WinRoot\Media\ding.wav`,
		},
		{
			name:    "Should fall back to C:\\Windows when WINDIR is empty",
			preset:  PresetBasso,
			windir:  "",
			wantOK:  true,
			wantOut: `C:\Windows\Media\chord.wav`,
		},
		{
			name:    "Should fall back to C:\\Windows when WINDIR is whitespace",
			preset:  PresetSubmarine,
			windir:  "   ",
			wantOK:  true,
			wantOut: `C:\Windows\Media\Ring01.wav`,
		},
		{
			name:   "Should return false for unknown preset",
			preset: "mystery",
			windir: `C:\Windows`,
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := windowsPresetPath(tc.preset, tc.windir)
			if ok != tc.wantOK {
				t.Fatalf("ok mismatch: got %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if got != tc.wantOut {
				t.Fatalf("unexpected path: got %q, want %q", got, tc.wantOut)
			}
		})
	}
}

func TestKnownPresets_ContainsDefaults(t *testing.T) {
	t.Parallel()

	presets := KnownPresets()
	if len(presets) == 0 {
		t.Fatal("expected at least one preset")
	}
	seen := make(map[string]struct{}, len(presets))
	for _, p := range presets {
		seen[p] = struct{}{}
	}
	for _, required := range []string{PresetGlass, PresetBasso} {
		if _, ok := seen[required]; !ok {
			t.Fatalf("KnownPresets missing required default %q", required)
		}
	}
}

func TestEscapePSSingleQuoted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no apostrophes is a no-op",
			in:   `C:\Windows\Media\tada.wav`,
			want: `C:\Windows\Media\tada.wav`,
		},
		{
			name: "single apostrophe is doubled",
			in:   `C:\Users\O'Neil\alert.wav`,
			want: `C:\Users\O''Neil\alert.wav`,
		},
		{
			name: "multiple apostrophes are all doubled",
			in:   `C:\a'b'c'.wav`,
			want: `C:\a''b''c''.wav`,
		},
		{
			name: "empty string is unchanged",
			in:   "",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := escapePSSingleQuoted(tc.in)
			if got != tc.want {
				t.Fatalf("escapePSSingleQuoted(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
